package sqlkit_test

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/bluetape4k/bluetape-go/sqlkit"
)

func TestWithTxValidatesInputs(t *testing.T) {
	if err := sqlkit.WithTx(context.Background(), nil, nil, func(context.Context, *sql.Tx) error { return nil }); !errors.Is(err, sqlkit.ErrInvalidArgument) {
		t.Fatalf("nil db error = %v, want ErrInvalidArgument", err)
	}

	db := beginnerFunc(func(context.Context, *sql.TxOptions) (*sql.Tx, error) {
		t.Fatal("BeginTx should not be called for nil fn")
		return nil, nil
	})
	if err := sqlkit.WithTx(context.Background(), db, nil, nil); !errors.Is(err, sqlkit.ErrInvalidArgument) {
		t.Fatalf("nil fn error = %v, want ErrInvalidArgument", err)
	}
}

func TestWithTxPropagatesBeginError(t *testing.T) {
	expected := errors.New("begin failed")
	db := beginnerFunc(func(context.Context, *sql.TxOptions) (*sql.Tx, error) {
		return nil, expected
	})

	err := sqlkit.WithTx(context.Background(), db, nil, func(context.Context, *sql.Tx) error {
		t.Fatal("transaction function should not run")
		return nil
	})
	if !errors.Is(err, expected) {
		t.Fatalf("WithTx error = %v, want begin error", err)
	}
}

func TestWithTxPropagatesContextCancellationToBegin(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	db := beginnerFunc(func(ctx context.Context, _ *sql.TxOptions) (*sql.Tx, error) {
		return nil, ctx.Err()
	})

	err := sqlkit.WithTx(ctx, db, nil, func(context.Context, *sql.Tx) error {
		t.Fatal("transaction function should not run")
		return nil
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("WithTx error = %v, want context.Canceled", err)
	}
}

type beginnerFunc func(context.Context, *sql.TxOptions) (*sql.Tx, error)

func (f beginnerFunc) BeginTx(ctx context.Context, opts *sql.TxOptions) (*sql.Tx, error) {
	return f(ctx, opts)
}
