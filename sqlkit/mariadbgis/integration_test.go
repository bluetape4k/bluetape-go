package mariadbgis_test

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/bluetape4k/bluetape-go/sqlkit/mariadbgis"
	mariadbtestcontainer "github.com/bluetape4k/bluetape-go/testcontainers/mariadb"
	_ "github.com/go-sql-driver/mysql"
)

func TestMariaDBPointRoundTripAndSpatialPredicates(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	t.Cleanup(cancel)
	db, err := sql.Open("mysql", mariadbtestcontainer.Start(ctx, t))
	if err != nil {
		t.Fatalf("open mariadb: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	create, err := mariadbgis.CreateSpatialTableSQL("places", "location", 4326)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, "DROP TABLE IF EXISTS places"); err != nil {
		t.Fatalf("drop table: %v", err)
	}
	if _, err := db.ExecContext(ctx, create); err != nil {
		t.Fatalf("create table: %v", err)
	}
	index, _ := mariadbgis.CreateSpatialIndexSQL("places", "location")
	if _, err := db.ExecContext(ctx, index); err != nil {
		t.Fatalf("create spatial index: %v", err)
	}
	point, _ := mariadbgis.NewPoint(127.0276, 37.4979, 4326)
	insert, _ := mariadbgis.InsertPoint("places", "location", point)
	if _, err := db.ExecContext(ctx, insert.SQL, insert.Args...); err != nil {
		t.Fatalf("insert point: %v", err)
	}
	selectSQL, _ := mariadbgis.SelectPointSQL("places", "location")
	var raw []byte
	var srid int
	if err := db.QueryRowContext(ctx, selectSQL).Scan(&raw, &srid); err != nil {
		t.Fatalf("read point: %v", err)
	}
	var got mariadbgis.Point
	if err := got.ScanWithSRID(raw, srid); err != nil {
		t.Fatalf("scan point: %v", err)
	}
	if got != point || srid != 4326 {
		t.Fatalf("round trip=%#v srid=%d want=%#v/4326", got, srid, point)
	}
	distance, _ := mariadbgis.WithinDistance("places", "location", point, 500)
	assertMariaDBRow(t, ctx, db, distance.SQL, distance.Args...)
	bounds, _ := mariadbgis.WithinBounds("places", "location", 126, 37, 128, 38, 4326)
	assertMariaDBRow(t, ctx, db, bounds.SQL, bounds.Args...)
}

func assertMariaDBRow(t *testing.T, ctx context.Context, db *sql.DB, query string, args ...any) {
	t.Helper()
	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		t.Fatalf("query %q: %v", query, err)
	}
	defer rows.Close()
	if !rows.Next() {
		t.Fatalf("query %q returned no rows", query)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("query rows: %v", err)
	}
}
