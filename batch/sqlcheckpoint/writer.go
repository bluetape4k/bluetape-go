package sqlcheckpoint

import (
	"context"
	"database/sql"
	"errors"

	"github.com/bluetape4k/bluetape-go/batch"
	"github.com/bluetape4k/bluetape-go/sqlkit"
)

var (
	errNilDB               = errors.New("sqlcheckpoint: db must not be nil")
	errNilWrite            = errors.New("sqlcheckpoint: write callback must not be nil")
	errNilCodecEncode      = errors.New("sqlcheckpoint: codec encode must not be nil")
	errNilCodecDecode      = errors.New("sqlcheckpoint: codec decode must not be nil")
	errWriterUninitialized = errors.New("sqlcheckpoint: writer is not initialized")
)

// Codec encodes and decodes checkpoint values. Its functions must be safe for
// concurrent use when the Writer is shared by concurrent callers.
type Codec[C any] struct {
	// Encode serializes one checkpoint value.
	Encode func(C) ([]byte, error)
	// Decode deserializes one checkpoint value.
	Decode func([]byte) (C, error)
}

// WriteTxFunc persists output items through a caller-defined SQL session in an
// explicit Read Committed transaction. It must be correct at that isolation
// level and safe for concurrent use when the Writer is shared.
type WriteTxFunc[T any] func(context.Context, sqlkit.Session, []T) error

// Writer atomically persists output items and durable checkpoints in PostgreSQL.
// A Writer must be created with New; its zero value is not initialized.
// A Writer is safe for concurrent use when its Codec and WriteTxFunc are safe
// for concurrent use. Calls for the same checkpoint key must still be serialized.
type Writer[T any, C any] struct {
	db       *sql.DB
	options  normalizedOptions
	codec    Codec[C]
	write    WriteTxFunc[T]
	queryRow func(context.Context, string, ...any) rowScanner
	beginTx  func(context.Context) (transaction, error)
}

var _ batch.AtomicCheckpointWriter[any] = (*Writer[any, any])(nil)

// New validates and stores the checkpoint writer configuration without performing database I/O.
// The caller retains ownership of db and is responsible for applying SchemaSQL.
// Commit transactions always use Read Committed and do not inherit an ambient
// role or database isolation default.
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

	w := &Writer[T, C]{
		db:      db,
		options: normalized,
		codec:   codec,
		write:   write,
		queryRow: func(ctx context.Context, query string, args ...any) rowScanner {
			return db.QueryRowContext(ctx, query, args...)
		},
	}
	w.beginTx = func(ctx context.Context) (transaction, error) {
		tx, err := beginCheckpointTransaction(ctx, w.db)
		if err != nil {
			return nil, err
		}
		return &sqlTransaction{tx: tx}, nil
	}
	return w, nil
}

func beginCheckpointTransaction(ctx context.Context, db *sql.DB) (*sql.Tx, error) {
	return db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
}
