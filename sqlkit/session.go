package sqlkit

import (
	"context"
	stdsql "database/sql"
)

// Execer is the context-aware Exec boundary shared by *sql.DB and *sql.Tx.
type Execer interface {
	ExecContext(ctx context.Context, query string, args ...any) (stdsql.Result, error)
}

// Queryer is the context-aware Query boundary shared by *sql.DB and *sql.Tx.
type Queryer interface {
	QueryContext(ctx context.Context, query string, args ...any) (*stdsql.Rows, error)
}

// QueryRower is the context-aware QueryRow boundary shared by *sql.DB and
// *sql.Tx.
type QueryRower interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *stdsql.Row
}

// Session is the common context-aware execution surface implemented by
// *sql.DB and *sql.Tx.
type Session interface {
	Execer
	Queryer
	QueryRower
}

// Beginner starts database/sql transactions. *sql.DB implements this
// interface.
type Beginner interface {
	BeginTx(ctx context.Context, opts *stdsql.TxOptions) (*stdsql.Tx, error)
}
