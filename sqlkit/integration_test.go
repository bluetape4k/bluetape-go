package sqlkit_test

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/bluetape4k/bluetape-go/sqlkit"
	postgrestestcontainer "github.com/bluetape4k/bluetape-go/testcontainers/postgres"
	_ "github.com/jackc/pgx/v5/stdlib"
)

func TestWithTxPostgresCommitAndRollback(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	t.Cleanup(cancel)

	db := openPostgresDB(ctx, t)
	if _, err := db.ExecContext(ctx, `create table sqlkit_accounts (id integer primary key, name text not null)`); err != nil {
		t.Fatalf("create table: %v", err)
	}

	if err := sqlkit.WithTx(ctx, db, nil, func(ctx context.Context, tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `insert into sqlkit_accounts (id, name) values ($1, $2)`, 1, "committed")
		return err
	}); err != nil {
		t.Fatalf("WithTx commit path failed: %v", err)
	}

	committed, err := sqlkit.QueryOne(ctx, db, `select name from sqlkit_accounts where id = $1`, scanString, 1)
	if err != nil {
		t.Fatalf("query committed row: %v", err)
	}
	if committed != "committed" {
		t.Fatalf("committed row = %q, want committed", committed)
	}

	expected := errors.New("rollback this transaction")
	err = sqlkit.WithTx(ctx, db, nil, func(ctx context.Context, tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, `insert into sqlkit_accounts (id, name) values ($1, $2)`, 2, "rolled back"); err != nil {
			return err
		}
		return expected
	})
	if !errors.Is(err, expected) {
		t.Fatalf("WithTx rollback path error = %v, want %v", err, expected)
	}

	_, ok, err := sqlkit.QueryOptional(ctx, db, `select name from sqlkit_accounts where id = $1`, scanString, 2)
	if err != nil {
		t.Fatalf("query rolled-back row: %v", err)
	}
	if ok {
		t.Fatal("rolled-back row is visible")
	}
}

func openPostgresDB(ctx context.Context, t *testing.T) *sql.DB {
	t.Helper()

	dsn := postgrestestcontainer.Start(ctx, t)
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatalf("open postgres: %v", err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Fatalf("close postgres: %v", err)
		}
	})
	if err := db.PingContext(ctx); err != nil {
		t.Fatalf("ping postgres: %v", err)
	}
	return db
}

func scanString(rows *sql.Rows) (string, error) {
	var value string
	if err := rows.Scan(&value); err != nil {
		return "", err
	}
	return value, nil
}
