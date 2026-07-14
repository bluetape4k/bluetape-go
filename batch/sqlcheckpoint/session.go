package sqlcheckpoint

import (
	"context"
	"database/sql"

	"github.com/bluetape4k/bluetape-go/sqlkit"
)

type transaction interface {
	sqlkit.Session
	ScanRevision(context.Context, string, ...any) (int64, error)
	Commit() error
	Rollback() error
}

type sqlTransaction struct {
	tx *sql.Tx
}

func (t *sqlTransaction) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	return t.tx.ExecContext(ctx, query, args...)
}

func (t *sqlTransaction) QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	return t.tx.QueryContext(ctx, query, args...)
}

func (t *sqlTransaction) QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row {
	return t.tx.QueryRowContext(ctx, query, args...)
}

func (t *sqlTransaction) ScanRevision(ctx context.Context, query string, args ...any) (int64, error) {
	var revision int64
	err := t.tx.QueryRowContext(ctx, query, args...).Scan(&revision)
	return revision, err
}

func (t *sqlTransaction) Commit() error   { return t.tx.Commit() }
func (t *sqlTransaction) Rollback() error { return t.tx.Rollback() }

type guardedSession struct {
	session sqlkit.Session
}

func (s guardedSession) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	return s.session.ExecContext(ctx, query, args...)
}

func (s guardedSession) QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	return s.session.QueryContext(ctx, query, args...)
}

func (s guardedSession) QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row {
	return s.session.QueryRowContext(ctx, query, args...)
}
