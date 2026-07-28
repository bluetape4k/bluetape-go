package sqlkit

import (
	"context"
	stdsql "database/sql"
)

// Execer *sql.DB와 *sql.Tx가 공유하는 context-aware Exec boundary다.
type Execer interface {
	ExecContext(ctx context.Context, query string, args ...any) (stdsql.Result, error)
}

// Queryer *sql.DB와 *sql.Tx가 공유하는 context-aware Query boundary다.
type Queryer interface {
	QueryContext(ctx context.Context, query string, args ...any) (*stdsql.Rows, error)
}

// QueryRower *sql.DB와 *sql.Tx가 공유하는 context-aware QueryRow boundary다.
type QueryRower interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *stdsql.Row
}

// Session *sql.DB와 *sql.Tx가 구현하는 공통 context-aware execution surface다.
type Session interface {
	Execer
	Queryer
	QueryRower
}

// Beginner database/sql transaction을 시작한다. *sql.DB가 이 interface를 구현한다.
type Beginner interface {
	BeginTx(ctx context.Context, opts *stdsql.TxOptions) (*stdsql.Tx, error)
}
