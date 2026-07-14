package sqlcheckpoint

import (
	"context"
	"database/sql"
	"errors"

	"github.com/bluetape4k/bluetape-go/batch"
	"github.com/bluetape4k/bluetape-go/sqlkit"
)

var (
	errNilDB          = errors.New("sqlcheckpoint: db must not be nil")
	errNilWrite       = errors.New("sqlcheckpoint: write callback must not be nil")
	errNilCodecEncode = errors.New("sqlcheckpoint: codec encode must not be nil")
	errNilCodecDecode = errors.New("sqlcheckpoint: codec decode must not be nil")
	errWriterNotReady = errors.New("sqlcheckpoint: load and commit are not implemented")
)

// Codec encodes and decodes checkpoint values.
type Codec[C any] struct {
	// Encode serializes one checkpoint value.
	Encode func(C) ([]byte, error)
	// Decode deserializes one checkpoint value.
	Decode func([]byte) (C, error)
}

// WriteTxFunc persists output items through a caller-defined SQL session.
type WriteTxFunc[T any] func(context.Context, sqlkit.Session, []T) error

// Writer atomically persists output items and durable checkpoints in PostgreSQL.
// A Writer must be created with New; its zero value is not initialized.
type Writer[T any, C any] struct {
	db      *sql.DB
	options normalizedOptions
	codec   Codec[C]
	write   WriteTxFunc[T]
}

var _ batch.AtomicCheckpointWriter[any] = (*Writer[any, any])(nil)

// New validates and stores the checkpoint writer configuration without performing database I/O.
// The caller retains ownership of db and is responsible for applying SchemaSQL.
func New[T any, C any](db *sql.DB, options Options, codec Codec[C], write WriteTxFunc[T]) (*Writer[T, C], error) {
	if db == nil {
		return nil, errNilDB
	}
	if write == nil {
		return nil, errNilWrite
	}
	if codec.Encode == nil {
		return nil, errNilCodecEncode
	}
	if codec.Decode == nil {
		return nil, errNilCodecDecode
	}

	normalized, err := options.normalize()
	if err != nil {
		return nil, err
	}

	return &Writer[T, C]{
		db:      db,
		options: normalized,
		codec:   codec,
		write:   write,
	}, nil
}

// Load is reserved for the checkpoint loading implementation.
func (*Writer[T, C]) Load(context.Context, string) (batch.VersionedCheckpoint, bool, error) {
	return batch.VersionedCheckpoint{}, false, errWriterNotReady
}

// Commit is reserved for the transactional checkpoint commit implementation.
func (*Writer[T, C]) Commit(context.Context, string, uint64, []T, any) (uint64, error) {
	return 0, errWriterNotReady
}
