package mysqlgis_test

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/bluetape4k/bluetape-go/sqlkit/mysqlgis"
	mysqltestcontainer "github.com/bluetape4k/bluetape-go/testcontainers/mysql"
	_ "github.com/go-sql-driver/mysql"
)

func TestMySQLPointRoundTripAndSpatialPredicates(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	t.Cleanup(cancel)
	db, err := sql.Open("mysql", mysqltestcontainer.Start(ctx, t))
	if err != nil {
		t.Fatalf("open mysql: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	create, err := mysqlgis.CreateSpatialTableSQL("places", "location", 4326)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, "DROP TABLE IF EXISTS places"); err != nil {
		t.Fatalf("drop table: %v", err)
	}
	if _, err := db.ExecContext(ctx, create); err != nil {
		t.Fatalf("create table: %v", err)
	}
	index, _ := mysqlgis.CreateSpatialIndexSQL("places", "location")
	if _, err := db.ExecContext(ctx, index); err != nil {
		t.Fatalf("create spatial index: %v", err)
	}
	point, _ := mysqlgis.NewPoint(127.0276, 37.4979, 4326)
	insert, _ := mysqlgis.InsertPoint("places", "location", point)
	if _, err := db.ExecContext(ctx, insert.SQL, insert.Args...); err != nil {
		t.Fatalf("insert point: %v", err)
	}
	selectSQL, _ := mysqlgis.SelectPointSQL("places", "location")
	var raw []byte
	var srid int
	if err := db.QueryRowContext(ctx, selectSQL).Scan(&raw, &srid); err != nil {
		t.Fatalf("read point: %v", err)
	}
	var got mysqlgis.Point
	if err := got.ScanWithSRID(raw, srid); err != nil {
		t.Fatalf("scan point: %v", err)
	}
	if got != point || srid != 4326 {
		t.Fatalf("round trip=%#v srid=%d want=%#v/4326", got, srid, point)
	}
	distance, _ := mysqlgis.WithinDistance("places", "location", point, 500)
	assertMySQLRow(t, ctx, db, distance.SQL, distance.Args...)
	bounds, _ := mysqlgis.WithinBounds("places", "location", 126, 37, 128, 38, 4326)
	assertMySQLRow(t, ctx, db, bounds.SQL, bounds.Args...)
}

func assertMySQLRow(t *testing.T, ctx context.Context, db *sql.DB, query string, args ...any) {
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
