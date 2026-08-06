package sqlcheckpoint

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"math"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/bluetape4k/bluetape-go/batch"
	"github.com/bluetape4k/bluetape-go/sqlkit"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

const panicNilHelperEnv = "BLUETAPE_SQLCHECKPOINT_PANICNIL_HELPER"

func TestCommitValidatesAndEncodesBeforeBegin(t *testing.T) {
	tests := []struct {
		name       string
		key        string
		expected   uint64
		checkpoint any
		encodeErr  error
		want       error
		wantEncode int
	}{
		{name: "empty key", checkpoint: "value"},
		{name: "oversized key", key: "12345", checkpoint: "value"},
		{name: "revision outside bigint", key: "key", expected: uint64(math.MaxInt64) + 1, checkpoint: "value"},
		{name: "revision exhausted", key: "key", expected: uint64(math.MaxInt64), checkpoint: "value", want: batch.ErrCheckpointVersionExhausted},
		{name: "wrong checkpoint type", key: "key", checkpoint: 42},
		{name: "encode failure", key: "key", checkpoint: "value", encodeErr: errors.New("hostile-codec"), wantEncode: 1},
		{name: "payload too large", key: "key", checkpoint: "value", wantEncode: 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			begins := 0
			encodes := 0
			writer := newCommitWriter(t, Options{MaxKeyBytes: 4, MaxPayloadBytes: 4}, func(string) ([]byte, error) {
				encodes++
				if tt.encodeErr != nil {
					return nil, tt.encodeErr
				}
				if tt.name == "payload too large" {
					return []byte("12345"), nil
				}
				return []byte("data"), nil
			}, nil)
			writer.beginTx = func(context.Context) (transaction, error) {
				begins++
				return &fakeTransaction{}, nil
			}

			revision, err := writer.Commit(context.Background(), tt.key, tt.expected, nil, tt.checkpoint)
			if err == nil || revision != 0 {
				t.Fatalf("Commit() = %d, %v; want zero revision and error", revision, err)
			}
			if tt.want != nil && !errors.Is(err, tt.want) {
				t.Fatalf("Commit() error = %v, want %v", err, tt.want)
			}
			if begins != 0 {
				t.Fatalf("invalid Commit began %d transactions", begins)
			}
			if encodes != tt.wantEncode {
				t.Fatalf("Encode calls = %d, want %d", encodes, tt.wantEncode)
			}
		})
	}
}

func TestCommitReturnsPreCanceledContextWithoutBegin(t *testing.T) {
	begins := 0
	writer := newCommitWriter(t, Options{}, encodeString, nil)
	writer.beginTx = func(context.Context) (transaction, error) {
		begins++
		return &fakeTransaction{}, nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	revision, err := writer.Commit(ctx, "key", 0, nil, "payload")
	if revision != 0 || !errors.Is(err, context.Canceled) {
		t.Fatalf("Commit() = %d, %v; want context.Canceled", revision, err)
	}
	if begins != 0 {
		t.Fatalf("pre-canceled Commit began %d transactions", begins)
	}
}

func TestCommitNilAndZeroWriterReturnInitializationError(t *testing.T) {
	for _, tt := range []struct {
		name   string
		writer *Writer[string, string]
	}{
		{name: "nil"},
		{name: "zero", writer: new(Writer[string, string])},
	} {
		t.Run(tt.name, func(t *testing.T) {
			revision, err := tt.writer.Commit(context.Background(), "key", 0, nil, "payload")
			if revision != 0 || !errors.Is(err, errWriterUninitialized) {
				t.Fatalf("Commit() = %d, %v; want initialization error", revision, err)
			}
		})
	}
}

func TestCommitBeginFailureIsKnownSanitizedOperationError(t *testing.T) {
	beginErr := errors.New("postgres://user:hostile-password@host/hostile-db")
	begins := 0
	writer := newCommitWriter(t, Options{Namespace: "hostile-namespace"}, encodeString, nil)
	writer.beginTx = func(context.Context) (transaction, error) {
		begins++
		return nil, beginErr
	}

	revision, err := writer.Commit(context.Background(), "hostile-key", 0, nil, "payload")
	if revision != 0 || !errors.Is(err, beginErr) || errors.Is(err, batch.ErrCommitUnknown) || errors.Is(err, batch.ErrAtomicityUnknown) {
		t.Fatalf("Commit() = %d, %v; want known begin failure", revision, err)
	}
	if begins != 1 {
		t.Fatalf("Begin calls = %d, want 1", begins)
	}
	assertSanitizedOperationError(t, err, "begin", "postgres://", "hostile-password", "hostile-namespace", "hostile-key")
}

func TestCommitSavepointFailureSuppressesCallbackCASAndCommit(t *testing.T) {
	savepointErr := errors.New("hostile-savepoint-transport")
	tx := &fakeTransaction{execErrors: map[string][]error{savepointSQL: {savepointErr}}}
	callbackCalls := 0
	writer := newCommitWriter(t, Options{}, encodeString, func(context.Context, sqlkit.Session, []string) error {
		callbackCalls++
		return nil
	})
	writer.beginTx = tx.begin

	revision, err := writer.Commit(context.Background(), "key", 0, []string{"item"}, "payload")
	if revision != 0 || !errors.Is(err, savepointErr) {
		t.Fatalf("Commit() = %d, %v; want savepoint failure", revision, err)
	}
	assertSanitizedOperationError(t, err, "savepoint", "hostile-savepoint-transport")
	assertOperations(t, tx.operations, "savepoint", "rollback")
	if callbackCalls != 0 || tx.scanCalls != 0 || tx.commitCalls != 0 {
		t.Fatalf("savepoint failure dispatched callback=%d cas=%d commit=%d", callbackCalls, tx.scanCalls, tx.commitCalls)
	}
}

func TestCommitEncodeFailureIsTypedRedactedAndDoesNotBegin(t *testing.T) {
	codecErr := errors.New("hostile-payload hostile-codec-cause")
	begins := 0
	writer := newCommitWriter(t, Options{}, func(string) ([]byte, error) { return nil, codecErr }, nil)
	writer.beginTx = func(context.Context) (transaction, error) {
		begins++
		return &fakeTransaction{}, nil
	}

	revision, err := writer.Commit(context.Background(), "key", 0, nil, "checkpoint")
	if revision != 0 || !errors.Is(err, codecErr) {
		t.Fatalf("Commit() = %d, %v; want codec cause", revision, err)
	}
	var typed *CodecError
	if !errors.As(err, &typed) || typed.Operation() != "encode" {
		t.Fatalf("Commit() error = %T %v; want encode CodecError", err, err)
	}
	for _, marker := range []string{"hostile-payload", "hostile-codec-cause"} {
		if strings.Contains(err.Error(), marker) {
			t.Fatalf("CodecError leaked %q: %v", marker, err)
		}
	}
	if begins != 0 {
		t.Fatalf("encode failure began %d transactions", begins)
	}
}

func TestCommitEmptyItemsSuppressesGuardAndCallback(t *testing.T) {
	tx := &fakeTransaction{revision: 1}
	callbackCalls := 0
	writer := newCommitWriter(t, Options{Namespace: "tenant"}, encodeString, func(context.Context, sqlkit.Session, []string) error {
		callbackCalls++
		return nil
	})
	writer.beginTx = tx.begin

	revision, err := writer.Commit(nil, "key\x00", 0, nil, "checkpoint") //nolint:staticcheck // nil normalization is the contract under test.
	if err != nil || revision != 1 {
		t.Fatalf("Commit() = %d, %v; want 1, nil", revision, err)
	}
	if callbackCalls != 0 {
		t.Fatalf("callback calls = %d, want 0", callbackCalls)
	}
	assertOperations(t, tx.operations, "cas:insert", "commit")
	if tx.scanQuery != insertCheckpointSQL {
		t.Fatalf("CAS query = %q, want insertCheckpointSQL", tx.scanQuery)
	}
	assertCommitArgs(t, tx.scanArgs, []byte("tenant"), []byte("key\x00"), []byte("checkpoint"))
}

func TestCommitNormalizesNilEncodedPayloadToNonNilEmptyBytea(t *testing.T) {
	tx := &fakeTransaction{revision: 1}
	callbackCalls := 0
	writer := newCommitWriter(t, Options{}, func(string) ([]byte, error) { return nil, nil }, func(context.Context, sqlkit.Session, []string) error {
		callbackCalls++
		return nil
	})
	writer.beginTx = tx.begin

	revision, err := writer.Commit(context.Background(), "key", 0, []string{"item"}, "checkpoint")
	if err != nil || revision != 1 {
		t.Fatalf("Commit() = %d, %v; want 1, nil", revision, err)
	}
	if callbackCalls != 1 {
		t.Fatalf("callback calls = %d, want 1", callbackCalls)
	}
	if len(tx.scanArgs) != 3 {
		t.Fatalf("CAS args = %#v, want 3", tx.scanArgs)
	}
	payload, ok := tx.scanArgs[2].([]byte)
	if !ok || payload == nil || len(payload) != 0 {
		t.Fatalf("CAS payload = %#v, want non-nil empty []byte", tx.scanArgs[2])
	}
	assertOperations(t, tx.operations, "savepoint", "release", "cas:insert", "commit")
}

func TestCommitNonEmptyUsesGuardedSessionAndOneInsertCAS(t *testing.T) {
	tx := &fakeTransaction{revision: 1}
	callbackCalls := 0
	writer := newCommitWriter(t, Options{}, encodeString, func(ctx context.Context, session sqlkit.Session, items []string) error {
		callbackCalls++
		if ctx == nil || fmt.Sprint(items) != "[first second]" {
			t.Fatalf("callback arguments = %v, %v", ctx, items)
		}
		if _, ok := session.(interface{ Commit() error }); ok {
			t.Fatal("guarded callback session exposed Commit")
		}
		if _, ok := session.(interface{ Rollback() error }); ok {
			t.Fatal("guarded callback session exposed Rollback")
		}
		_, err := session.ExecContext(ctx, "insert business", 1)
		return err
	})
	writer.beginTx = tx.begin

	revision, err := writer.Commit(context.Background(), "key", 0, []string{"first", "second"}, "checkpoint")
	if err != nil || revision != 1 {
		t.Fatalf("Commit() = %d, %v; want 1, nil", revision, err)
	}
	if callbackCalls != 1 || tx.beginCalls != 1 || tx.commitCalls != 1 {
		t.Fatalf("calls: begin=%d callback=%d commit=%d", tx.beginCalls, callbackCalls, tx.commitCalls)
	}
	assertOperations(t, tx.operations, "savepoint", "exec:insert business", "release", "cas:insert", "commit")
}

func TestCommitUpdateUsesExactCASAndExpectedRevision(t *testing.T) {
	tx := &fakeTransaction{revision: 8}
	writer := newCommitWriter(t, Options{}, encodeString, nil)
	writer.beginTx = tx.begin

	revision, err := writer.Commit(context.Background(), "key", 7, nil, "payload")
	if err != nil || revision != 8 {
		t.Fatalf("Commit() = %d, %v; want 8, nil", revision, err)
	}
	assertOperations(t, tx.operations, "cas:update", "commit")
	if tx.scanQuery != updateCheckpointSQL {
		t.Fatalf("CAS query = %q, want updateCheckpointSQL", tx.scanQuery)
	}
	assertCommitArgs(t, tx.scanArgs[:3], []byte("default"), []byte("key"), []byte("payload"))
	if got, ok := tx.scanArgs[3].(int64); !ok || got != 7 {
		t.Fatalf("expected revision arg = %#v, want int64(7)", tx.scanArgs[3])
	}
}

func TestCommitSQLStatementsAreFixed(t *testing.T) {
	const wantInsert = `insert into public.bluetape_batch_checkpoints
(namespace, checkpoint_key, revision, payload, updated_at)
values ($1::bytea, $2::bytea, 1, $3::bytea, pg_catalog.clock_timestamp())
on conflict (namespace, checkpoint_key) do nothing
returning revision`
	const wantUpdate = `update public.bluetape_batch_checkpoints set
revision = revision + 1, payload = $3::bytea, updated_at = pg_catalog.clock_timestamp()
where namespace = $1::bytea and checkpoint_key = $2::bytea and revision = $4::bigint
returning revision`
	if insertCheckpointSQL != wantInsert || updateCheckpointSQL != wantUpdate {
		t.Fatalf("checkpoint CAS SQL changed:\ninsert=%q\nupdate=%q", insertCheckpointSQL, updateCheckpointSQL)
	}
	if savepointSQL != "savepoint bluetape_sqlcheckpoint_guard" ||
		releaseSavepointSQL != "release savepoint bluetape_sqlcheckpoint_guard" ||
		rollbackToSavepointSQL != "rollback to savepoint bluetape_sqlcheckpoint_guard" {
		t.Fatalf("reserved guard SQL changed: %q, %q, %q", savepointSQL, releaseSavepointSQL, rollbackToSavepointSQL)
	}
}

func TestCommitConflictRollsBackAndReturnsZero(t *testing.T) {
	tx := &fakeTransaction{scanErr: sql.ErrNoRows}
	writer := newCommitWriter(t, Options{}, encodeString, nil)
	writer.beginTx = tx.begin

	revision, err := writer.Commit(context.Background(), "key", 2, nil, "payload")
	if revision != 0 || !errors.Is(err, batch.ErrCheckpointConflict) {
		t.Fatalf("Commit() = %d, %v; want conflict", revision, err)
	}
	assertOperations(t, tx.operations, "cas:update", "rollback")
}

func TestCommitCallbackErrorProbesOwnershipThenRollsBack(t *testing.T) {
	callbackErr := errors.New("hostile-callback-cause")
	tx := &fakeTransaction{}
	writer := newCommitWriter(t, Options{}, encodeString, func(context.Context, sqlkit.Session, []string) error {
		return callbackErr
	})
	writer.beginTx = tx.begin

	revision, err := writer.Commit(context.Background(), "key", 0, []string{"item"}, "payload")
	if revision != 0 || !errors.Is(err, callbackErr) {
		t.Fatalf("Commit() = %d, %v; want callback cause", revision, err)
	}
	assertSanitizedOperationError(t, err, "callback", "hostile-callback-cause")
	assertOperations(t, tx.operations, "savepoint", "release", "rollback")
}

func TestOwnershipProbeRecoversAbortedTransactionOnlyAfterRollbackToSavepoint(t *testing.T) {
	probeErr := &pgconn.PgError{Code: "25P02", Message: "hostile-aborted"}
	tx := &fakeTransaction{execErrors: map[string][]error{releaseSavepointSQL: {probeErr}}}
	writer := newCommitWriter(t, Options{}, encodeString, func(context.Context, sqlkit.Session, []string) error { return nil })
	writer.beginTx = tx.begin

	revision, err := writer.Commit(context.Background(), "key", 0, []string{"item"}, "payload")
	if revision != 0 || !errors.Is(err, probeErr) {
		t.Fatalf("Commit() = %d, %v; want known probe failure", revision, err)
	}
	if errors.Is(err, batch.ErrAtomicityUnknown) || errors.Is(err, batch.ErrCommitUnknown) {
		t.Fatalf("proven ownership marked unknown: %v", err)
	}
	assertSanitizedOperationError(t, err, "ownership probe", "hostile-aborted")
	assertOperations(t, tx.operations, "savepoint", "release", "rollback-to", "rollback")
}

func TestOwnershipProbeDoesNotProveAbortedTransactionWhenRollbackToFails(t *testing.T) {
	tests := []struct {
		name         string
		rollbackErr  error
		wantContract bool
	}{
		{name: "invalid savepoint", rollbackErr: &pgconn.PgError{Code: "3B001", Message: "hostile-savepoint"}, wantContract: true},
		{name: "transport", rollbackErr: errors.New("hostile-transport")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tx := &fakeTransaction{execErrors: map[string][]error{
				releaseSavepointSQL:    {&pgconn.PgError{Code: "25P02", Message: "hostile-aborted"}},
				rollbackToSavepointSQL: {tt.rollbackErr},
			}}
			writer := newCommitWriter(t, Options{}, encodeString, func(context.Context, sqlkit.Session, []string) error { return nil })
			writer.beginTx = tx.begin

			revision, err := writer.Commit(context.Background(), "key", 0, []string{"item"}, "payload")
			if revision != 0 || !errors.Is(err, batch.ErrAtomicityUnknown) || !errors.Is(err, batch.ErrCommitUnknown) {
				t.Fatalf("Commit() = %d, %v; want unknown sentinels", revision, err)
			}
			if errors.Is(err, ErrCallbackContractViolation) != tt.wantContract {
				t.Fatalf("contract violation = %v, want %v", errors.Is(err, ErrCallbackContractViolation), tt.wantContract)
			}
			assertOperations(t, tx.operations, "savepoint", "release", "rollback-to", "rollback")
		})
	}
}

func TestOwnershipUnknownPreservesCallbackCauseInsideSanitizedError(t *testing.T) {
	callbackErr := errors.New("hostile-callback")
	tx := &fakeTransaction{execErrors: map[string][]error{releaseSavepointSQL: {errors.New("hostile-transport")}}}
	writer := newCommitWriter(t, Options{}, encodeString, func(context.Context, sqlkit.Session, []string) error { return callbackErr })
	writer.beginTx = tx.begin

	revision, err := writer.Commit(context.Background(), "key", 0, []string{"item"}, "payload")
	if revision != 0 || !errors.Is(err, callbackErr) || !errors.Is(err, batch.ErrAtomicityUnknown) {
		t.Fatalf("Commit() = %d, %v; want callback cause and unknown sentinel", revision, err)
	}
	assertSanitizedOperationError(t, err, "ownership probe", "hostile-callback", "hostile-transport")
}

func TestOwnershipPositiveLifecycleEvidenceFailsClosed(t *testing.T) {
	tests := []struct {
		name string
		err  error
	}{
		{name: "active context tx done", err: sql.ErrTxDone},
		{name: "no active transaction", err: &pgconn.PgError{Code: "25P01", Message: "hostile-no-transaction"}},
		{name: "invalid savepoint", err: &pgconn.PgError{Code: "3B001", Message: "hostile-savepoint"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tx := &fakeTransaction{execErrors: map[string][]error{releaseSavepointSQL: {tt.err}}}
			writer := newCommitWriter(t, Options{}, encodeString, func(context.Context, sqlkit.Session, []string) error { return nil })
			writer.beginTx = tx.begin

			revision, err := writer.Commit(context.Background(), "key", 0, []string{"item"}, "payload")
			if revision != 0 || !errors.Is(err, ErrCallbackContractViolation) || !errors.Is(err, batch.ErrAtomicityUnknown) || !errors.Is(err, batch.ErrCommitUnknown) {
				t.Fatalf("Commit() = %d, %v; want contract and unknown sentinels", revision, err)
			}
			assertSanitizedOperationError(t, err, "ownership probe", "hostile")
			assertOperations(t, tx.operations, "savepoint", "release", "rollback")
		})
	}
}

func TestOwnershipUnclassifiedAndCanceledFailuresDoNotGuessContractViolation(t *testing.T) {
	tests := []struct {
		name   string
		cancel bool
		err    error
	}{
		{name: "transport", err: errors.New("hostile-transport")},
		{name: "deadline", err: context.DeadlineExceeded},
		{name: "closed connection", err: sql.ErrConnDone},
		{name: "bad connection", err: driver.ErrBadConn},
		{name: "canceled tx done", cancel: true, err: sql.ErrTxDone},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			tx := &fakeTransaction{execErrors: map[string][]error{releaseSavepointSQL: {tt.err}}}
			writer := newCommitWriter(t, Options{}, encodeString, func(context.Context, sqlkit.Session, []string) error {
				if tt.cancel {
					cancel()
				}
				return nil
			})
			writer.beginTx = tx.begin

			revision, err := writer.Commit(ctx, "key", 0, []string{"item"}, "payload")
			if revision != 0 || !errors.Is(err, batch.ErrAtomicityUnknown) || !errors.Is(err, batch.ErrCommitUnknown) {
				t.Fatalf("Commit() = %d, %v; want unknown sentinels", revision, err)
			}
			if errors.Is(err, ErrCallbackContractViolation) {
				t.Fatalf("unclassified ownership failure guessed contract violation: %v", err)
			}
			assertOperations(t, tx.operations, "savepoint", "release", "rollback")
		})
	}
}

func TestAtomicityPanicOwnershipProvenRepanicsOriginalAfterRollback(t *testing.T) {
	panicValue := &secretPanic{marker: "hostile-original-panic"}
	tx := &fakeTransaction{}
	writer := newCommitWriter(t, Options{}, encodeString, func(context.Context, sqlkit.Session, []string) error {
		panic(panicValue)
	})
	writer.beginTx = tx.begin

	recovered := recoverCommit(t, func() { _, _ = writer.Commit(context.Background(), "key", 0, []string{"item"}, "payload") })
	if recovered != panicValue {
		t.Fatalf("recovered = %#v, want original panic", recovered)
	}
	assertOperations(t, tx.operations, "savepoint", "release", "rollback")
}

func TestAtomicityPanicOwnershipUnknownIsSanitizedAndPreservesValueOnlyByAccessor(t *testing.T) {
	panicValue := &secretPanic{marker: "hostile-original-panic"}
	probeErr := errors.New("hostile-provider-cause")
	tx := &fakeTransaction{execErrors: map[string][]error{releaseSavepointSQL: {probeErr}}}
	writer := newCommitWriter(t, Options{}, encodeString, func(context.Context, sqlkit.Session, []string) error {
		panic(panicValue)
	})
	writer.beginTx = tx.begin

	recovered := recoverCommit(t, func() { _, _ = writer.Commit(context.Background(), "key", 0, []string{"item"}, "payload") })
	atomicityPanic, ok := recovered.(*AtomicityPanic)
	if !ok {
		t.Fatalf("recovered = %T, want *AtomicityPanic", recovered)
	}
	if atomicityPanic.PanicValue() != panicValue || !errors.Is(atomicityPanic, batch.ErrAtomicityUnknown) || !errors.Is(atomicityPanic, batch.ErrCommitUnknown) {
		t.Fatalf("AtomicityPanic contract failed: %#v", atomicityPanic)
	}
	for _, rendered := range []string{atomicityPanic.Error(), fmt.Sprintf("%v", atomicityPanic), fmt.Sprintf("%+v", atomicityPanic), fmt.Sprintf("%v", atomicityPanic.Unwrap())} {
		if strings.Contains(rendered, panicValue.marker) || strings.Contains(rendered, "hostile-provider-cause") {
			t.Fatalf("AtomicityPanic leaked sensitive value: %s", rendered)
		}
	}
	assertOperations(t, tx.operations, "savepoint", "release", "rollback")
}

func TestAtomicityPanicNilUnderLegacyPanicNilMode(t *testing.T) {
	if os.Getenv(panicNilHelperEnv) != "1" {
		helperCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		command := exec.CommandContext(helperCtx, os.Args[0], "-test.run=^TestAtomicityPanicNilUnderLegacyPanicNilMode$")
		command.Env = panicNilSubprocessEnv()
		output, err := command.CombinedOutput()
		if err != nil {
			t.Fatalf("panic(nil) helper failed: %v\n%s", err, output)
		}
		return
	}

	t.Run("unknown ownership", func(t *testing.T) {
		tx := &fakeTransaction{execErrors: map[string][]error{releaseSavepointSQL: {errors.New("hostile-transport")}}}
		writer := newCommitWriter(t, Options{}, encodeString, func(context.Context, sqlkit.Session, []string) error {
			panic(nil)
		})
		writer.beginTx = tx.begin

		returned, recovered := capturePanic(func() {
			_, _ = writer.Commit(context.Background(), "key", 0, []string{"item"}, "payload")
		})
		if returned {
			t.Fatal("panic(nil) returned normally")
		}
		atomicityPanic, ok := recovered.(*AtomicityPanic)
		if !ok || atomicityPanic.PanicValue() != nil || !errors.Is(atomicityPanic, batch.ErrAtomicityUnknown) || !errors.Is(atomicityPanic, batch.ErrCommitUnknown) {
			t.Fatalf("recovered = %#v, want nil-valued *AtomicityPanic with unknown sentinels", recovered)
		}
		assertOperations(t, tx.operations, "savepoint", "release", "rollback")
		if tx.scanCalls != 0 || tx.commitCalls != 0 {
			t.Fatalf("unknown panic(nil) dispatched cas=%d commit=%d", tx.scanCalls, tx.commitCalls)
		}
	})

	t.Run("proven ownership", func(t *testing.T) {
		tx := &fakeTransaction{}
		writer := newCommitWriter(t, Options{}, encodeString, func(context.Context, sqlkit.Session, []string) error {
			panic(nil)
		})
		writer.beginTx = tx.begin

		returned, recovered := capturePanic(func() {
			_, _ = writer.Commit(context.Background(), "key", 0, []string{"item"}, "payload")
		})
		if returned || recovered != nil {
			t.Fatalf("proven panic(nil) = returned %v, recovered %#v; want original nil panic", returned, recovered)
		}
		assertOperations(t, tx.operations, "savepoint", "release", "rollback")
		if tx.scanCalls != 0 || tx.commitCalls != 0 {
			t.Fatalf("proven panic(nil) dispatched cas=%d commit=%d", tx.scanCalls, tx.commitCalls)
		}
	})
}

func TestAtomicityPanicTypedNilPreservesOriginalSemantics(t *testing.T) {
	var panicValue *secretPanic

	t.Run("unknown ownership", func(t *testing.T) {
		tx := &fakeTransaction{execErrors: map[string][]error{releaseSavepointSQL: {errors.New("hostile-transport")}}}
		writer := newCommitWriter(t, Options{}, encodeString, func(context.Context, sqlkit.Session, []string) error {
			panic(panicValue)
		})
		writer.beginTx = tx.begin

		returned, recovered := capturePanic(func() {
			_, _ = writer.Commit(context.Background(), "key", 0, []string{"item"}, "payload")
		})
		atomicityPanic, ok := recovered.(*AtomicityPanic)
		if returned || !ok || atomicityPanic.PanicValue() != panicValue {
			t.Fatalf("typed-nil unknown panic = returned %v, recovered %#v", returned, recovered)
		}
		if !errors.Is(atomicityPanic, batch.ErrAtomicityUnknown) || !errors.Is(atomicityPanic, batch.ErrCommitUnknown) {
			t.Fatalf("typed-nil AtomicityPanic missing unknown sentinels: %v", atomicityPanic)
		}
		assertOperations(t, tx.operations, "savepoint", "release", "rollback")
		if tx.scanCalls != 0 || tx.commitCalls != 0 {
			t.Fatalf("unknown typed-nil panic dispatched cas=%d commit=%d", tx.scanCalls, tx.commitCalls)
		}
	})

	t.Run("proven ownership", func(t *testing.T) {
		tx := &fakeTransaction{}
		writer := newCommitWriter(t, Options{}, encodeString, func(context.Context, sqlkit.Session, []string) error {
			panic(panicValue)
		})
		writer.beginTx = tx.begin

		returned, recovered := capturePanic(func() {
			_, _ = writer.Commit(context.Background(), "key", 0, []string{"item"}, "payload")
		})
		if returned || recovered != panicValue {
			t.Fatalf("typed-nil proven panic = returned %v, recovered %#v; want original", returned, recovered)
		}
		assertOperations(t, tx.operations, "savepoint", "release", "rollback")
		if tx.scanCalls != 0 || tx.commitCalls != 0 {
			t.Fatalf("proven typed-nil panic dispatched cas=%d commit=%d", tx.scanCalls, tx.commitCalls)
		}
	})
}

func TestCommitRechecksContextAfterOwnershipProbeBeforeCAS(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	tx := &fakeTransaction{revision: 1}
	writer := newCommitWriter(t, Options{}, encodeString, func(context.Context, sqlkit.Session, []string) error {
		cancel()
		return nil
	})
	writer.beginTx = tx.begin

	revision, err := writer.Commit(ctx, "key", 0, []string{"item"}, "payload")
	if revision != 0 || !errors.Is(err, context.Canceled) {
		t.Fatalf("Commit() = %d, %v; want known cancellation", revision, err)
	}
	assertOperations(t, tx.operations, "savepoint", "release", "rollback")
	if tx.scanCalls != 0 || tx.commitCalls != 0 {
		t.Fatalf("post-probe cancellation dispatched cas=%d commit=%d", tx.scanCalls, tx.commitCalls)
	}
}

func TestCommitRechecksContextAfterCASBeforeCommitDispatch(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	tx := &fakeTransaction{revision: 1, scanHook: cancel}
	writer := newCommitWriter(t, Options{}, encodeString, nil)
	writer.beginTx = tx.begin

	revision, err := writer.Commit(ctx, "key", 0, nil, "payload")
	if revision != 0 || !errors.Is(err, context.Canceled) {
		t.Fatalf("Commit() = %d, %v; want known cancellation", revision, err)
	}
	assertOperations(t, tx.operations, "cas:insert", "rollback")
	if tx.scanCalls != 1 || tx.commitCalls != 0 {
		t.Fatalf("post-CAS cancellation counts: cas=%d commit=%d", tx.scanCalls, tx.commitCalls)
	}
}

func TestCommitClassifiesServerRejectionAndTransportUnknown(t *testing.T) {
	tests := []struct {
		name        string
		commitErr   error
		wantUnknown bool
	}{
		{name: "server rejection", commitErr: &pgconn.PgError{Code: "40001", Message: "hostile-server"}},
		{name: "definitive commit rollback", commitErr: pgx.ErrTxCommitRollback},
		{name: "transport unknown", commitErr: errors.New("hostile-transport"), wantUnknown: true},
		{name: "bad connection", commitErr: sql.ErrConnDone, wantUnknown: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tx := &fakeTransaction{revision: 1, commitErr: tt.commitErr}
			writer := newCommitWriter(t, Options{}, encodeString, nil)
			writer.beginTx = tx.begin

			revision, err := writer.Commit(context.Background(), "key", 0, nil, "payload")
			if revision != 0 || err == nil {
				t.Fatalf("Commit() = %d, %v; want error", revision, err)
			}
			if !errors.Is(err, tt.commitErr) {
				t.Fatalf("Commit() error = %v, want cause %v", err, tt.commitErr)
			}
			if errors.Is(err, batch.ErrCommitUnknown) != tt.wantUnknown {
				t.Fatalf("ErrCommitUnknown = %v, want %v: %v", errors.Is(err, batch.ErrCommitUnknown), tt.wantUnknown, err)
			}
			if errors.Is(err, batch.ErrAtomicityUnknown) {
				t.Fatalf("provider Commit error marked atomicity unknown: %v", err)
			}
			assertSanitizedOperationError(t, err, "commit", "hostile")
			if tx.commitCalls != 1 {
				t.Fatalf("Commit calls = %d, want 1", tx.commitCalls)
			}
		})
	}
}

func TestCommitPreservesRollbackErrorCausallyWithoutRenderingIt(t *testing.T) {
	callbackErr := errors.New("hostile-callback")
	rollbackErr := errors.New("hostile-rollback")
	tx := &fakeTransaction{rollbackErr: rollbackErr}
	writer := newCommitWriter(t, Options{}, encodeString, func(context.Context, sqlkit.Session, []string) error { return callbackErr })
	writer.beginTx = tx.begin

	revision, err := writer.Commit(context.Background(), "key", 0, []string{"item"}, "payload")
	if revision != 0 || !errors.Is(err, callbackErr) || !errors.Is(err, rollbackErr) {
		t.Fatalf("Commit() = %d, %v; rollback cause not preserved", revision, err)
	}
	assertSanitizedOperationError(t, err, "callback", "hostile-callback", "hostile-rollback")
}

func TestCommitPreservesErrTxDoneFromCallbackRollback(t *testing.T) {
	callbackErr := errors.New("hostile-callback")
	tx := &fakeTransaction{rollbackErr: sql.ErrTxDone}
	writer := newCommitWriter(t, Options{}, encodeString, func(context.Context, sqlkit.Session, []string) error { return callbackErr })
	writer.beginTx = tx.begin

	revision, err := writer.Commit(context.Background(), "key", 0, []string{"item"}, "payload")
	if revision != 0 || !errors.Is(err, callbackErr) || !errors.Is(err, sql.ErrTxDone) {
		t.Fatalf("Commit() = %d, %v; want callback and sql.ErrTxDone causes", revision, err)
	}
	assertSanitizedOperationError(t, err, "callback", "hostile-callback", sql.ErrTxDone.Error())
}

func TestCommitPreservesErrTxDoneFromConflictRollback(t *testing.T) {
	tx := &fakeTransaction{scanErr: sql.ErrNoRows, rollbackErr: sql.ErrTxDone}
	writer := newCommitWriter(t, Options{}, encodeString, nil)
	writer.beginTx = tx.begin

	revision, err := writer.Commit(context.Background(), "key", 0, nil, "payload")
	if revision != 0 || !errors.Is(err, batch.ErrCheckpointConflict) || !errors.Is(err, sql.ErrTxDone) {
		t.Fatalf("Commit() = %d, %v; want conflict and sql.ErrTxDone causes", revision, err)
	}
	assertSanitizedOperationError(t, err, "rollback", batch.ErrCheckpointConflict.Error(), sql.ErrTxDone.Error())
}

func TestCommitDoesNotRetryAnyTransactionStage(t *testing.T) {
	callbackCalls := 0
	tx := &fakeTransaction{scanErr: sql.ErrNoRows}
	writer := newCommitWriter(t, Options{}, encodeString, func(context.Context, sqlkit.Session, []string) error {
		callbackCalls++
		return nil
	})
	writer.beginTx = tx.begin

	revision, err := writer.Commit(context.Background(), "key", 0, []string{"item"}, "payload")
	if revision != 0 || !errors.Is(err, batch.ErrCheckpointConflict) {
		t.Fatalf("Commit() = %d, %v; want conflict", revision, err)
	}
	if tx.beginCalls != 1 || callbackCalls != 1 || tx.scanCalls != 1 || tx.commitCalls != 0 {
		t.Fatalf("unexpected retry counts: begin=%d callback=%d cas=%d commit=%d", tx.beginCalls, callbackCalls, tx.scanCalls, tx.commitCalls)
	}
}

type fakeTransaction struct {
	operations  []string
	execErrors  map[string][]error
	revision    int64
	scanErr     error
	scanHook    func()
	commitErr   error
	rollbackErr error
	beginCalls  int
	scanCalls   int
	commitCalls int
	scanQuery   string
	scanArgs    []any
}

func (tx *fakeTransaction) begin(context.Context) (transaction, error) {
	tx.beginCalls++
	return tx, nil
}

func (tx *fakeTransaction) ExecContext(_ context.Context, query string, _ ...any) (sql.Result, error) {
	operation := "exec:" + query
	switch query {
	case savepointSQL:
		operation = "savepoint"
	case releaseSavepointSQL:
		operation = "release"
	case rollbackToSavepointSQL:
		operation = "rollback-to"
	}
	tx.operations = append(tx.operations, operation)
	if queued := tx.execErrors[query]; len(queued) > 0 {
		tx.execErrors[query] = queued[1:]
		return nil, queued[0]
	}
	return fakeResult(1), nil
}

func (*fakeTransaction) QueryContext(context.Context, string, ...any) (*sql.Rows, error) {
	panic("QueryContext is not used by the commit harness")
}

func (*fakeTransaction) QueryRowContext(context.Context, string, ...any) *sql.Row {
	panic("QueryRowContext is not used by the commit harness")
}

func (tx *fakeTransaction) ScanRevision(_ context.Context, query string, args ...any) (int64, error) {
	tx.scanCalls++
	tx.scanQuery = query
	tx.scanArgs = append([]any(nil), args...)
	operation := "cas:update"
	if query == insertCheckpointSQL {
		operation = "cas:insert"
	}
	tx.operations = append(tx.operations, operation)
	if tx.scanHook != nil {
		tx.scanHook()
	}
	return tx.revision, tx.scanErr
}

func (tx *fakeTransaction) Commit() error {
	tx.commitCalls++
	tx.operations = append(tx.operations, "commit")
	return tx.commitErr
}

func (tx *fakeTransaction) Rollback() error {
	tx.operations = append(tx.operations, "rollback")
	return tx.rollbackErr
}

type fakeResult int64

func (r fakeResult) LastInsertId() (int64, error) { return int64(r), nil }
func (r fakeResult) RowsAffected() (int64, error) { return int64(r), nil }

type secretPanic struct{ marker string }

func (p *secretPanic) Error() string { return p.marker }

func newCommitWriter(
	t *testing.T,
	options Options,
	encode func(string) ([]byte, error),
	write WriteTxFunc[string],
) *Writer[string, string] {
	t.Helper()
	if encode == nil {
		encode = encodeString
	}
	if write == nil {
		write = func(context.Context, sqlkit.Session, []string) error { return nil }
	}
	writer, err := New(&sql.DB{}, options, Codec[string]{Encode: encode, Decode: decodeString}, write)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	return writer
}

func encodeString(value string) ([]byte, error) { return []byte(value), nil }

func recoverCommit(t *testing.T, commit func()) (recovered any) {
	t.Helper()
	defer func() { recovered = recover() }()
	commit()
	t.Fatal("Commit did not panic")
	return nil
}

func capturePanic(commit func()) (returned bool, recovered any) {
	func() {
		defer func() { recovered = recover() }()
		commit()
		returned = true
	}()
	return returned, recovered
}

func panicNilSubprocessEnv() []string {
	environment := make([]string, 0, len(os.Environ())+2)
	for _, value := range os.Environ() {
		if strings.HasPrefix(value, "GODEBUG=") || strings.HasPrefix(value, panicNilHelperEnv+"=") {
			continue
		}
		environment = append(environment, value)
	}
	return append(environment, "GODEBUG=panicnil=1", panicNilHelperEnv+"=1")
}

func assertOperations(t *testing.T, got []string, want ...string) {
	t.Helper()
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("operations = %v, want %v", got, want)
	}
}

func assertCommitArgs(t *testing.T, got []any, namespace, key, payload []byte) {
	t.Helper()
	if len(got) != 3 {
		t.Fatalf("CAS args = %#v, want 3", got)
	}
	assertBytesArgument(t, got[0], namespace)
	assertBytesArgument(t, got[1], key)
	assertBytesArgument(t, got[2], payload)
}

func assertSanitizedOperationError(t *testing.T, err error, operation string, markers ...string) {
	t.Helper()
	var opErr *OpError
	if !errors.As(err, &opErr) || opErr.Operation() != operation {
		t.Fatalf("error = %T %v, want OpError operation %q", err, err, operation)
	}
	for _, marker := range markers {
		if strings.Contains(err.Error(), marker) {
			t.Fatalf("OpError leaked %q: %v", marker, err)
		}
	}
}
