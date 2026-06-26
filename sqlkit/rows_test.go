package sqlkit_test

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"io"
	"sync/atomic"
	"testing"

	"github.com/bluetape4k/bluetape-go/sqlkit"
)

func TestQueryOneMapsSingleRow(t *testing.T) {
	db := openStubDB(t, &stubConfig{
		columns: []string{"value"},
		rows:    [][]driver.Value{{int64(42)}},
	})

	got, err := sqlkit.QueryOne(context.Background(), db, "select value", scanInt)
	if err != nil {
		t.Fatalf("QueryOne failed: %v", err)
	}
	if got != 42 {
		t.Fatalf("QueryOne = %d, want 42", got)
	}
}

func TestQueryOneReportsNoRows(t *testing.T) {
	db := openStubDB(t, &stubConfig{columns: []string{"value"}})

	_, err := sqlkit.QueryOne(context.Background(), db, "select value", scanInt)
	if !errors.Is(err, sqlkit.ErrNoRows) {
		t.Fatalf("QueryOne error = %v, want ErrNoRows", err)
	}
}

func TestQueryOptionalReportsTooManyRows(t *testing.T) {
	db := openStubDB(t, &stubConfig{
		columns: []string{"value"},
		rows:    [][]driver.Value{{int64(1)}, {int64(2)}},
	})

	_, _, err := sqlkit.QueryOptional(context.Background(), db, "select value", scanInt)
	if !errors.Is(err, sqlkit.ErrTooManyRows) {
		t.Fatalf("QueryOptional error = %v, want ErrTooManyRows", err)
	}
}

func TestQueryOptionalStopsAfterDetectingTooManyRows(t *testing.T) {
	db := openStubDB(t, &stubConfig{
		columns: []string{"value"},
		rows:    [][]driver.Value{{int64(1)}, {int64(2)}, {int64(3)}},
	})
	var mapped int32

	_, _, err := sqlkit.QueryOptional(context.Background(), db, "select value", func(rows *sql.Rows) (int, error) {
		mapped++
		return scanInt(rows)
	})
	if !errors.Is(err, sqlkit.ErrTooManyRows) {
		t.Fatalf("QueryOptional error = %v, want ErrTooManyRows", err)
	}
	if mapped != 2 {
		t.Fatalf("mapped rows = %d, want 2", mapped)
	}
}

func TestQueryAllClosesRowsOnScanError(t *testing.T) {
	cfg := &stubConfig{
		columns: []string{"value"},
		rows:    [][]driver.Value{{"not-an-int"}},
	}
	db := openStubDBWithConfig(t, cfg)

	_, err := sqlkit.QueryAll(context.Background(), db, "select value", scanInt)
	if err == nil {
		t.Fatal("QueryAll error = nil, want scan error")
	}
	if got := cfg.closeCount.Load(); got != 1 {
		t.Fatalf("rows close count = %d, want 1", got)
	}
}

func TestQueryAllReportsRowsCloseError(t *testing.T) {
	closeErr := errors.New("close failed")
	db := openStubDB(t, &stubConfig{
		columns:  []string{"value"},
		rows:     [][]driver.Value{{int64(1)}},
		closeErr: closeErr,
	})

	_, err := sqlkit.QueryAll(context.Background(), db, "select value", scanInt)
	if !errors.Is(err, closeErr) {
		t.Fatalf("QueryAll error = %v, want close error", err)
	}
}

func TestQueryAllPropagatesContextCancellation(t *testing.T) {
	db := openStubDB(t, &stubConfig{columns: []string{"value"}})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := sqlkit.QueryAll(ctx, db, "select value", scanInt)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("QueryAll error = %v, want context.Canceled", err)
	}
}

func TestQueryAllValidatesInputs(t *testing.T) {
	db := openStubDB(t, &stubConfig{columns: []string{"value"}})

	if _, err := sqlkit.QueryAll[int](context.Background(), nil, "select value", scanInt); !errors.Is(err, sqlkit.ErrInvalidArgument) {
		t.Fatalf("nil queryer error = %v, want ErrInvalidArgument", err)
	}
	if _, err := sqlkit.QueryAll[int](context.Background(), db, "select value", nil); !errors.Is(err, sqlkit.ErrInvalidArgument) {
		t.Fatalf("nil mapper error = %v, want ErrInvalidArgument", err)
	}
}

func TestScanOneValidatesDestination(t *testing.T) {
	db := openStubDB(t, &stubConfig{
		columns: []string{"value"},
		rows:    [][]driver.Value{{int64(1)}},
	})

	_, err := sqlkit.QueryOne(context.Background(), db, "select value", sqlkit.ScanOne[int](nil))
	if !errors.Is(err, sqlkit.ErrInvalidArgument) {
		t.Fatalf("ScanOne nil destination error = %v, want ErrInvalidArgument", err)
	}
}

func scanInt(rows *sql.Rows) (int, error) {
	var value int
	if err := rows.Scan(&value); err != nil {
		return 0, err
	}
	return value, nil
}

var stubDriverID atomic.Uint64

type stubConfig struct {
	columns    []string
	rows       [][]driver.Value
	queryErr   error
	closeErr   error
	closeCount atomic.Int32
}

func openStubDB(t *testing.T, cfg *stubConfig) *sql.DB {
	t.Helper()
	return openStubDBWithConfig(t, cfg)
}

func openStubDBWithConfig(t *testing.T, cfg *stubConfig) *sql.DB {
	t.Helper()

	name := fmt.Sprintf("sqlkit_stub_%d", stubDriverID.Add(1))
	sql.Register(name, stubDriver{cfg: cfg})
	db, err := sql.Open(name, "")
	if err != nil {
		t.Fatalf("open stub db: %v", err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Fatalf("close stub db: %v", err)
		}
	})
	return db
}

type stubDriver struct {
	cfg *stubConfig
}

func (d stubDriver) Open(string) (driver.Conn, error) {
	return stubConn(d), nil
}

type stubConn struct {
	cfg *stubConfig
}

func (c stubConn) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("prepare is not implemented")
}

func (c stubConn) Close() error {
	return nil
}

func (c stubConn) Begin() (driver.Tx, error) {
	return nil, errors.New("begin is not implemented")
}

func (c stubConn) QueryContext(ctx context.Context, _ string, _ []driver.NamedValue) (driver.Rows, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if c.cfg.queryErr != nil {
		return nil, c.cfg.queryErr
	}
	return &stubRows{cfg: c.cfg}, nil
}

type stubRows struct {
	cfg   *stubConfig
	index int
}

func (r *stubRows) Columns() []string {
	return r.cfg.columns
}

func (r *stubRows) Close() error {
	r.cfg.closeCount.Add(1)
	return r.cfg.closeErr
}

func (r *stubRows) Next(dest []driver.Value) error {
	if r.index >= len(r.cfg.rows) {
		return io.EOF
	}
	copy(dest, r.cfg.rows[r.index])
	r.index++
	return nil
}
