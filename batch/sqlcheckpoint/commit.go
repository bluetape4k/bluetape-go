package sqlcheckpoint

import (
	"context"
	"database/sql"
	"errors"
	"math"
	"time"

	"github.com/bluetape4k/bluetape-go/batch"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

const ownershipProbeTimeout = time.Second

var (
	errExpectedVersionRange   = errors.New("sqlcheckpoint: expected checkpoint version exceeds PostgreSQL bigint")
	errCheckpointType         = errors.New("sqlcheckpoint: checkpoint has unexpected type")
	errEncodedPayloadTooLarge = errors.New("sqlcheckpoint: encoded checkpoint payload exceeds byte limit")
)

// Commit batch 단계, checkpoint, writer 안전성, 재시작의 쓰기 동작을 수행한다.
//
// 매개변수:
//   - ctx: 호출자가 소유한 취소, deadline, 요청 범위를 전달한다.
//   - key: Commit가 해석할 문자열이다. 빈 문자열은 구현 검증을 따른다.
//   - expectedVersion: commit할 때 기대하는 현재 version이다.
//   - items: Commit가 순서와 snapshot 의미를 유지하며 읽는 items 목록이다. nil과 빈 슬라이스는 해당 함수의 입력 규칙에 따라 처리한다.
//   - checkpoint: 저장하거나 commit할 checkpoint 값이다.
//
// 반환 오류는 입력 검증 실패, context 취소/deadline, 상태 전이 실패, 패키지 sentinel error와 typed error를 그대로 드러낸다.
func (w *Writer[T, C]) Commit(
	ctx context.Context,
	key string,
	expectedVersion uint64,
	items []T,
	checkpoint any,
) (uint64, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	if !w.commitInitialized() {
		return 0, errWriterUninitialized
	}

	rawKey := []byte(key)
	if len(rawKey) == 0 {
		return 0, errEmptyKey
	}
	if len(rawKey) > w.options.maxKeyBytes || len(rawKey) > MaxKeyBytes {
		return 0, errKeyTooLong
	}
	if expectedVersion > uint64(math.MaxInt64) {
		return 0, errExpectedVersionRange
	}
	if expectedVersion == uint64(math.MaxInt64) {
		return 0, batch.ErrCheckpointVersionExhausted
	}

	typedCheckpoint, ok := checkpoint.(C)
	if !ok {
		return 0, errCheckpointType
	}
	payload, err := w.codec.Encode(typedCheckpoint)
	if err != nil {
		return 0, newCodecError("encode", err)
	}
	if payload == nil {
		payload = []byte{}
	}
	if len(payload) > w.options.maxPayloadBytes || len(payload) > MaxPayloadBytes {
		return 0, errEncodedPayloadTooLarge
	}

	tx, err := w.beginTx(ctx)
	if err != nil {
		return 0, newOperationError(OperationBegin, w.options.namespace, rawKey, err)
	}
	finished := false
	rollback := func() error {
		if finished {
			return nil
		}
		finished = true
		return tx.Rollback()
	}
	defer func() {
		if !finished {
			_ = tx.Rollback()
		}
	}()

	if len(items) > 0 {
		if _, err = tx.ExecContext(ctx, savepointSQL); err != nil {
			return 0, w.rollbackOperation(tx, rollback, OperationSavepoint, rawKey, err)
		}

		panicValue, panicked, callbackErr := invokeCallback(ctx, w.write, guardedSession{session: tx}, items)
		probe := probeTransactionOwnership(ctx, tx)
		if panicked {
			_ = rollback()
			if probe.proven {
				panic(panicValue)
			}
			panic(&AtomicityPanic{panicValue: panicValue})
		}

		if !probe.proven {
			cause := probe.err
			if callbackErr != nil {
				cause = errors.Join(callbackErr, cause)
			}
			if contextErr := ctx.Err(); contextErr != nil {
				cause = errors.Join(cause, contextErr)
			}
			opErr := w.rollbackOperation(tx, rollback, OperationOwnershipProbe, rawKey, cause)
			joined := []error{opErr, batch.ErrAtomicityUnknown, batch.ErrCommitUnknown}
			if probe.contractViolation {
				joined = append(joined, ErrCallbackContractViolation)
			}
			return 0, errors.Join(joined...)
		}

		if callbackErr != nil || probe.err != nil {
			operation := OperationCallback
			cause := callbackErr
			if callbackErr == nil {
				operation = OperationOwnershipProbe
				cause = probe.err
			} else if probe.err != nil {
				cause = errors.Join(callbackErr, probe.err)
			}
			return 0, w.rollbackOperation(tx, rollback, operation, rawKey, cause)
		}
		if err = ctx.Err(); err != nil {
			return 0, w.rollbackKnown(tx, rollback, rawKey, err)
		}
	}

	query := insertCheckpointSQL
	args := []any{w.options.namespace, rawKey, payload}
	if expectedVersion > 0 {
		query = updateCheckpointSQL
		args = append(args, int64(expectedVersion))
	}
	revision, err := tx.ScanRevision(ctx, query, args...)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, w.rollbackKnown(tx, rollback, rawKey, batch.ErrCheckpointConflict)
	}
	if err != nil {
		return 0, w.rollbackOperation(tx, rollback, OperationCheckpoint, rawKey, err)
	}
	if err = ctx.Err(); err != nil {
		return 0, w.rollbackKnown(tx, rollback, rawKey, err)
	}

	err = tx.Commit()
	finished = true
	if err != nil {
		cause := err
		if contextErr := ctx.Err(); contextErr != nil {
			cause = errors.Join(err, contextErr)
		}
		opErr := newOperationError(OperationCommit, w.options.namespace, rawKey, cause)
		var serverErr *pgconn.PgError
		if errors.As(err, &serverErr) || errors.Is(err, pgx.ErrTxCommitRollback) {
			return 0, opErr
		}
		return 0, errors.Join(opErr, batch.ErrCommitUnknown)
	}
	return uint64(revision), nil
}

func (w *Writer[T, C]) commitInitialized() bool {
	return w != nil &&
		w.db != nil &&
		w.beginTx != nil &&
		w.write != nil &&
		w.codec.Encode != nil &&
		w.codec.Decode != nil &&
		len(w.options.namespace) > 0 &&
		w.options.maxKeyBytes >= 1 &&
		w.options.maxKeyBytes <= MaxKeyBytes &&
		w.options.maxPayloadBytes >= 1 &&
		w.options.maxPayloadBytes <= MaxPayloadBytes
}

func (w *Writer[T, C]) rollbackOperation(
	_ transaction,
	rollback func() error,
	operation string,
	rawKey []byte,
	cause error,
) error {
	if rollbackErr := rollback(); rollbackErr != nil {
		cause = errors.Join(cause, rollbackErr)
	}
	return newOperationError(operation, w.options.namespace, rawKey, cause)
}

func (w *Writer[T, C]) rollbackKnown(
	_ transaction,
	rollback func() error,
	rawKey []byte,
	cause error,
) error {
	rollbackErr := rollback()
	if rollbackErr == nil {
		return cause
	}
	return newOperationError(OperationRollback, w.options.namespace, rawKey, errors.Join(cause, rollbackErr))
}

func invokeCallback[T any](
	ctx context.Context,
	write WriteTxFunc[T],
	session guardedSession,
	items []T,
) (panicValue any, panicked bool, callbackErr error) {
	callbackReturned := false
	defer func() {
		panicValue = recover()
		panicked = !callbackReturned
	}()
	callbackErr = write(ctx, session, items)
	callbackReturned = true
	return nil, panicked, callbackErr
}

type ownershipProbe struct {
	proven            bool
	err               error
	contractViolation bool
}

func probeTransactionOwnership(ctx context.Context, tx transaction) ownershipProbe {
	probeCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), ownershipProbeTimeout)
	defer cancel()

	_, releaseErr := tx.ExecContext(probeCtx, releaseSavepointSQL)
	if releaseErr == nil {
		return ownershipProbe{proven: true}
	}
	if postgresCode(releaseErr) == "25P02" {
		_, rollbackToErr := tx.ExecContext(probeCtx, rollbackToSavepointSQL)
		if rollbackToErr == nil {
			return ownershipProbe{proven: true, err: releaseErr}
		}
		return ownershipProbe{
			err:               errors.Join(releaseErr, rollbackToErr),
			contractViolation: positiveLifecycleEvidence(ctx, rollbackToErr),
		}
	}
	return ownershipProbe{
		err:               releaseErr,
		contractViolation: positiveLifecycleEvidence(ctx, releaseErr),
	}
}

func positiveLifecycleEvidence(ctx context.Context, err error) bool {
	code := postgresCode(err)
	if code == "25P01" || code == "3B001" {
		return true
	}
	return errors.Is(err, sql.ErrTxDone) && ctx.Err() == nil
}

func postgresCode(err error) string {
	var serverErr *pgconn.PgError
	if errors.As(err, &serverErr) {
		return serverErr.Code
	}
	return ""
}
