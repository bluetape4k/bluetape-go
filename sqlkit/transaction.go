package sqlkit

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

// TxFunc는 tx 안에서 work를 실행한다. error를 반환하면 transaction을 rollback하고 nil을 반환하면 commit한다.
type TxFunc func(ctx context.Context, tx *sql.Tx) error

// WithTx는 transaction을 시작하고 fn을 실행한 뒤 fn이 성공한 경우에만 commit한다.
//
// database handle lifecycle은 호출자가 소유한다. WithTx는 자신이 시작한 transaction만 소유한다.
// Context cancellation은 BeginTx와 fn에 그대로 전달되며, cancellation이나 deadline error를 재시도하지 않는다.
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
