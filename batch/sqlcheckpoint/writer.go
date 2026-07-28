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

// Codec는 struct 공개 타입이며 batch 단계, checkpoint, writer 안전성, 재시작 계약을 보존한다.
// 필드와 zero value, nil 허용 여부, 동시성 소유권은 생성자와 메서드의 한국어 주석 및 테스트 계약을 따른다.
type Codec[C any] struct {
	// Encode serializes one checkpoint value.
	Encode func(C) ([]byte, error)
	// Decode deserializes one checkpoint value.
	Decode func([]byte) (C, error)
}

// WriteTxFunc는 func 공개 타입이며 batch 단계, checkpoint, writer 안전성, 재시작 계약을 보존한다.
// 필드와 zero value, nil 허용 여부, 동시성 소유권은 생성자와 메서드의 한국어 주석 및 테스트 계약을 따른다.
type WriteTxFunc[T any] func(context.Context, sqlkit.Session, []T) error

// Writer는 struct 공개 타입이며 batch 단계, checkpoint, writer 안전성, 재시작 계약을 보존한다.
// 필드와 zero value, nil 허용 여부, 동시성 소유권은 생성자와 메서드의 한국어 주석 및 테스트 계약을 따른다.
type Writer[T any, C any] struct {
	db       *sql.DB
	options  normalizedOptions
	codec    Codec[C]
	write    WriteTxFunc[T]
	queryRow func(context.Context, string, ...any) rowScanner
	beginTx  func(context.Context) (transaction, error)
}

var _ batch.AtomicCheckpointWriter[any] = (*Writer[any, any])(nil)

// New는 New 공개 API의 동작을 수행하며 batch 단계, checkpoint, writer 안전성, 재시작 계약을 보존한다.
//
// 매개변수:
//   - db: New 동작에 필요한 db 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//   - options: New 동작에 필요한 options 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//   - codec: New 동작에 필요한 codec 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//   - write: New 동작에 필요한 write 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//
// 반환 오류는 입력 검증 실패, 취소, deadline, 상태 전이 실패, 또는 패키지 sentinel/typed error 계약을 보존한다.
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
