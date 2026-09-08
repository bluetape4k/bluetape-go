package mysqlgis_test

import (
	"context"
	"database/sql"

	"github.com/bluetape4k/bluetape-go/sqlkit/mysqlgis"
)

func ExamplePoint() {
	point, _ := mysqlgis.NewWGS84Point(37.4979, 127.0276)
	value, _ := point.Value()
	_ = value
	// Output:
}

func ExampleWithinDistance() {
	ctx := context.Background()
	var db *sql.DB
	point, _ := mysqlgis.NewWGS84Point(37.4979, 127.0276)
	stmt, _ := mysqlgis.WithinDistance("places", "location", point, 500)
	if db != nil {
		_, _ = db.ExecContext(ctx, stmt.SQL, stmt.Args...)
	}
	// Output:
}
