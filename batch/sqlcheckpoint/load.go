package sqlcheckpoint

import (
	"bytes"
	"context"
	"database/sql"
	"errors"

	"github.com/bluetape4k/bluetape-go/batch"
)

const loadSQL = `select revision,
       pg_catalog.octet_length(payload),
       case when pg_catalog.octet_length(payload) <= $3 then payload end
from public.bluetape_batch_checkpoints
where namespace = $1::bytea and checkpoint_key = $2::bytea`

var (
	errEmptyKey                  = errors.New("sqlcheckpoint: checkpoint key must not be empty")
	errKeyTooLong                = errors.New("sqlcheckpoint: checkpoint key exceeds configured byte limit")
	errInvalidStoredRevision     = errors.New("sqlcheckpoint: stored checkpoint revision must be positive")
	errNegativeStoredPayloadSize = errors.New("sqlcheckpoint: stored checkpoint payload size is negative")
	errStoredPayloadTooLarge     = errors.New("sqlcheckpoint: stored checkpoint payload exceeds byte limit")
	errStoredPayloadSizeMismatch = errors.New("sqlcheckpoint: stored checkpoint payload size mismatch")
)

type rowScanner interface {
	Scan(...any) error
}

// Load는 Load 공개 API의 동작을 수행하며 batch 단계, checkpoint, writer 안전성, 재시작 계약을 보존한다.
//
// 매개변수:
//   - ctx: 호출자가 소유한 취소, deadline, 요청 범위를 전달한다.
//   - key: Load가 식별자, 상태, 이름, 또는 입력으로 해석하는 문자열 값이다. 빈 문자열 처리는 함수 계약을 따른다.
//
// 반환 오류는 입력 검증 실패, 취소, deadline, 상태 전이 실패, 또는 패키지 sentinel/typed error 계약을 보존한다.
func (w *Writer[T, C]) Load(ctx context.Context, key string) (batch.VersionedCheckpoint, bool, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return batch.VersionedCheckpoint{}, false, err
	}
	if !w.loadInitialized() {
		return batch.VersionedCheckpoint{}, false, errWriterUninitialized
	}

	rawKey := []byte(key)
	if len(rawKey) == 0 {
		return batch.VersionedCheckpoint{}, false, errEmptyKey
	}
	if len(rawKey) > w.options.maxKeyBytes || len(rawKey) > MaxKeyBytes {
		return batch.VersionedCheckpoint{}, false, errKeyTooLong
	}

	var revision int64
	var payloadLength int64
	var payload []byte
	err := w.queryRow(
		ctx,
		loadSQL,
		w.options.namespace,
		rawKey,
		w.options.maxPayloadBytes,
	).Scan(&revision, &payloadLength, &payload)
	if errors.Is(err, sql.ErrNoRows) {
		return batch.VersionedCheckpoint{}, false, nil
	}
	if err != nil {
		return batch.VersionedCheckpoint{}, false, newOperationError(OperationLoad, w.options.namespace, rawKey, err)
	}

	if revision <= 0 {
		return batch.VersionedCheckpoint{}, false, errInvalidStoredRevision
	}
	if payloadLength < 0 {
		return batch.VersionedCheckpoint{}, false, errNegativeStoredPayloadSize
	}
	if payloadLength > int64(w.options.maxPayloadBytes) || payloadLength > int64(MaxPayloadBytes) {
		return batch.VersionedCheckpoint{}, false, errStoredPayloadTooLarge
	}
	if payloadLength != int64(len(payload)) {
		return batch.VersionedCheckpoint{}, false, errStoredPayloadSizeMismatch
	}

	ownedPayload := bytes.Clone(payload)
	value, err := w.codec.Decode(ownedPayload)
	if err != nil {
		return batch.VersionedCheckpoint{}, false, newCodecError("decode", err)
	}
	return batch.VersionedCheckpoint{Value: value, Version: uint64(revision)}, true, nil
}

func (w *Writer[T, C]) loadInitialized() bool {
	return w != nil &&
		w.db != nil &&
		w.queryRow != nil &&
		w.write != nil &&
		w.codec.Encode != nil &&
		w.codec.Decode != nil &&
		len(w.options.namespace) > 0 &&
		w.options.maxKeyBytes >= 1 &&
		w.options.maxKeyBytes <= MaxKeyBytes &&
		w.options.maxPayloadBytes >= 1 &&
		w.options.maxPayloadBytes <= MaxPayloadBytes
}
