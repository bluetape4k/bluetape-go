package sqlkit_test

import (
	"context"
	"database/sql"

	"github.com/bluetape4k/bluetape-go/sqlkit"
)

var (
	_ sqlkit.Session = (*sql.DB)(nil)
	_ sqlkit.Session = (*sql.Tx)(nil)
)

type boundaryAccount struct {
	ID   int64
	Name string
}

type generatedAccountQueries interface {
	GetAccount(context.Context, int64) (boundaryAccount, error)
}

type generatedAccountBinder func(*sql.Tx) generatedAccountQueries

func ExampleSession_repositoryBoundary() {
	load := func(ctx context.Context, session sqlkit.Session, id int64) (boundaryAccount, error) {
		var account boundaryAccount
		err := session.QueryRowContext(
			ctx,
			"select id, name from accounts where id = $1",
			id,
		).Scan(&account.ID, &account.Name)
		return account, err
	}

	fromPool := func(ctx context.Context, db *sql.DB, id int64) (boundaryAccount, error) {
		return load(ctx, db, id)
	}
	fromTransaction := func(ctx context.Context, tx *sql.Tx, id int64) (boundaryAccount, error) {
		return load(ctx, tx, id)
	}

	_, _, _ = load, fromPool, fromTransaction
}

func ExampleWithTx_generatedQueryHandle() {
	load := func(
		ctx context.Context,
		db *sql.DB,
		bind generatedAccountBinder,
		id int64,
	) (boundaryAccount, error) {
		var account boundaryAccount
		err := sqlkit.WithTx(ctx, db, nil, func(ctx context.Context, tx *sql.Tx) error {
			queries := bind(tx)
			var err error
			account, err = queries.GetAccount(ctx, id)
			return err
		})
		return account, err
	}

	_ = load
}
