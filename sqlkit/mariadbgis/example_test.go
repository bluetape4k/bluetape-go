package mariadbgis_test

import (
	"context"
	"database/sql"

	"github.com/bluetape4k/bluetape-go/sqlkit/mariadbgis"
)

func ExamplePoint() {
	point, _ := mariadbgis.NewWGS84Point(37.4979, 127.0276)
	value, _ := point.Value()
	_ = value
	// Output:
}

func ExampleWithinDistance() {
	ctx := context.Background()
	var db *sql.DB
	point, _ := mariadbgis.NewWGS84Point(37.4979, 127.0276)
	stmt, _ := mariadbgis.WithinDistance("places", "location", point, 500)
	if db != nil {
		_, _ = db.ExecContext(ctx, stmt.SQL, stmt.Args...)
	}
	// Output:
}
