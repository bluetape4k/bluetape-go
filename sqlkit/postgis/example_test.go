package postgis_test

import (
	"context"
	"database/sql"

	"github.com/bluetape4k/bluetape-go/sqlkit/postgis"
)

func ExamplePoint() {
	point, err := postgis.NewPoint(127.0276, 37.4979, 4326)
	if err != nil {
		panic(err)
	}
	value, err := point.Value()
	if err != nil {
		panic(err)
	}
	_ = value
	// Output:
}

func ExampleWithinDistance() {
	ctx := context.Background()
	var db *sql.DB
	point, _ := postgis.NewPoint(127.0276, 37.4979, 4326)
	stmt, _ := postgis.WithinDistance("places", "location", point, 500)
	if db != nil {
		_, _ = db.ExecContext(ctx, stmt.SQL, stmt.Args...)
	}
	// Output:
}
