package mysqltestcontainer_test

import (
	"context"
	"database/sql"
	"testing"

	mysqltestcontainer "github.com/bluetape4k/bluetape-go/testcontainers/mysql"
	_ "github.com/go-sql-driver/mysql"
)

func TestStartMySQL(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	dsn := mysqltestcontainer.Start(ctx, t)

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatalf("open mysql connection: %v", err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Fatalf("close mysql connection: %v", err)
		}
	})

	var value int
	if err := db.QueryRowContext(ctx, "select 1").Scan(&value); err != nil {
		t.Fatalf("query mysql: %v", err)
	}
	if value != 1 {
		t.Fatalf("expected mysql query result 1, got %d", value)
	}
}
