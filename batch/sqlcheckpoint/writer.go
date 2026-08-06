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

// Codec batch 단계, checkpoint, writer 안전성, 재시작에서 사용하는 구조체다.
type Codec[C any] struct {
	// Encode serializes one checkpoint value.
	Encode func(C) ([]byte, error)
	// Decode deserializes one checkpoint value.
	Decode func([]byte) (C, error)
}

// WriteTxFunc batch 단계, checkpoint, writer 안전성, 재시작에서 사용하는 함수 타입이다.
type WriteTxFunc[T any] func(context.Context, sqlkit.Session, []T) error

// Writer batch 단계, checkpoint, writer 안전성, 재시작에서 사용하는 구조체다.
type Writer[T any, C any] struct {
	db       *sql.DB
	options  normalizedOptions
	codec    Codec[C]
	write    WriteTxFunc[T]
	queryRow func(context.Context, string, ...any) rowScanner
	beginTx  func(context.Context) (transaction, error)
}

var _ batch.AtomicCheckpointWriter[any] = (*Writer[any, any])(nil)

// New batch 단계, checkpoint, writer 안전성, 재시작에 사용할 값을 생성한다.
//
// 매개변수:
//   - db: 사용할 database handle이다. nil 허용 여부는 생성자 검증을 따른다.
//   - options: 적용할 옵션 목록이다. nil이면 기본값만 사용한다.
//   - codec: 저장 값 인코딩에 사용할 codec이다.
//   - write: checkpoint 저장에 사용할 쓰기 함수다.
//
// 반환 오류는 입력 검증 실패, context 취소/deadline, 상태 전이 실패, 패키지 sentinel error와 typed error를 그대로 드러낸다.
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
