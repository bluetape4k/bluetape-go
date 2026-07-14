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

// Load returns the typed checkpoint and its durable revision when the key exists.
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
		return batch.VersionedCheckpoint{}, false, newOperationError("load", w.options.namespace, rawKey, err)
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
