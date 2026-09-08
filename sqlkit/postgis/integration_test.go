package postgis_test

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/bluetape4k/bluetape-go/sqlkit"
	"github.com/bluetape4k/bluetape-go/sqlkit/postgis"
	postgistestcontainer "github.com/bluetape4k/bluetape-go/testcontainers/postgis"
	_ "github.com/jackc/pgx/v5/stdlib"
)

func TestPostGISPointRoundTripAndIndexedPredicates(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	t.Cleanup(cancel)
	db, err := sql.Open("pgx", postgistestcontainer.Start(ctx, t))
	if err != nil {
		t.Fatalf("open postgis: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	if _, err := db.ExecContext(ctx, postgis.CreateExtensionSQL()); err != nil {
		t.Fatalf("create extension: %v", err)
	}
	create, err := postgis.CreateSpatialTableSQL("places", "location", 4326)
	if err != nil {
		t.Fatalf("create table SQL: %v", err)
	}
	if _, err := db.ExecContext(ctx, create); err != nil {
		t.Fatalf("create table: %v", err)
	}
	index, err := postgis.CreateSpatialIndexSQL("places", "location")
	if err != nil {
		t.Fatalf("create index SQL: %v", err)
	}
	if _, err := db.ExecContext(ctx, index); err != nil {
		t.Fatalf("create index: %v", err)
	}

	point, err := postgis.NewPoint(127.0276, 37.4979, 4326)
	if err != nil {
		t.Fatalf("new point: %v", err)
	}
	insert, err := postgis.InsertPoint("places", "location", point)
	if err != nil {
		t.Fatalf("insert SQL: %v", err)
	}
	if _, err := insert.Exec(ctx, db); err != nil {
		t.Fatalf("insert point: %v", err)
	}

	selectSQL, err := postgis.SelectPointSQL("places", "location")
	if err != nil {
		t.Fatalf("select SQL: %v", err)
	}
	var raw []byte
	var srid int
	if err := db.QueryRowContext(ctx, selectSQL).Scan(&raw, &srid); err != nil {
		t.Fatalf("read point: %v", err)
	}
	var got postgis.Point
	if err := got.Scan(raw); err != nil {
		t.Fatalf("scan point: %v", err)
	}
	if got != point || srid != 4326 {
		t.Fatalf("round trip = %#v, srid=%d, want %#v/4326", got, srid, point)
	}

	distance, err := postgis.WithinDistance("places", "location", point, 500)
	if err != nil {
		t.Fatalf("distance SQL: %v", err)
	}
	assertOneRow(ctx, t, db, distance)
	bounds, err := postgis.WithinBounds("places", "location", 126, 37, 128, 38, 4326)
	if err != nil {
		t.Fatalf("bounds SQL: %v", err)
	}
	assertOneRow(ctx, t, db, bounds)
}

func assertOneRow(ctx context.Context, t *testing.T, db *sql.DB, stmt sqlkit.Statement) {
	t.Helper()
	rows, err := db.QueryContext(ctx, stmt.SQL, stmt.Args...)
	if err != nil {
		t.Fatalf("query %q: %v", stmt.SQL, err)
	}
	defer func() { _ = rows.Close() }()
	if !rows.Next() {
		t.Fatalf("query %q returned no rows", stmt.SQL)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("query rows: %v", err)
	}
}
