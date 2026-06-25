package mariadbtestcontainer_test

import (
	"context"
	"database/sql"
	"testing"

	mariadbtestcontainer "github.com/bluetape4k/bluetape-go/testcontainers/mariadb"
	_ "github.com/go-sql-driver/mysql"
)

func TestStartMariaDB(t *testing.T) {
	ctx := context.Background()
	srv := mariadbtestcontainer.StartServer(ctx, t)
	details, err := srv.ConnectionDetails(ctx)
	if err != nil {
		t.Fatalf("mariadb server details: %v", err)
	}
	dsn, err := details.Require(mariadbtestcontainer.DSNKey)
	if err != nil {
		t.Fatalf("mariadb dsn detail: %v", err)
	}

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatalf("open mariadb connection: %v", err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Fatalf("close mariadb connection: %v", err)
		}
	})

	var value int
	if err := db.QueryRowContext(ctx, "select 1").Scan(&value); err != nil {
		t.Fatalf("query mariadb: %v", err)
	}
	if value != 1 {
		t.Fatalf("expected mariadb query result 1, got %d", value)
	}
}

func TestConnectionDetailKey(t *testing.T) {
	if mariadbtestcontainer.DSNKey != "mariadb.dsn" {
		t.Fatalf("DSNKey = %q", mariadbtestcontainer.DSNKey)
	}
}
