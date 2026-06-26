package sqlkit

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

// TxFunc runs work inside tx. Returning an error rolls the transaction back;
// returning nil commits it.
type TxFunc func(ctx context.Context, tx *sql.Tx) error

// WithTx starts a transaction, runs fn, and commits only when fn succeeds.
//
// The caller owns the database handle lifecycle. WithTx only owns the
// transaction it starts. Context cancellation is passed to BeginTx and fn
// unchanged; cancellation or deadline errors are not retried.
func WithTx(ctx context.Context, db Beginner, opts *sql.TxOptions, fn TxFunc) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if db == nil {
		return fmt.Errorf("%w: db is nil", ErrInvalidArgument)
	}
	if fn == nil {
		return fmt.Errorf("%w: transaction function is nil", ErrInvalidArgument)
	}

	tx, err := db.BeginTx(ctx, opts)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}

	if err := fn(ctx, tx); err != nil {
		if rollbackErr := tx.Rollback(); rollbackErr != nil && !errors.Is(rollbackErr, sql.ErrTxDone) {
			return errors.Join(fmt.Errorf("transaction function: %w", err), fmt.Errorf("rollback transaction: %w", rollbackErr))
		}
		return fmt.Errorf("transaction function: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}
	return nil
}
