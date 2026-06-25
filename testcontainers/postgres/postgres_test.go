package postgrestestcontainer_test

import (
	"context"
	"testing"
	"time"

	postgrestestcontainer "github.com/bluetape4k/bluetape-go/testcontainers/postgres"
	"github.com/jackc/pgx/v5"
)

func TestStartPostgres(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	t.Cleanup(cancel)
	srv := postgrestestcontainer.StartServer(ctx, t)
	details, err := srv.ConnectionDetails(ctx)
	if err != nil {
		t.Fatalf("postgres server details: %v", err)
	}
	connString, err := details.Require(postgrestestcontainer.ConnectionStringKey)
	if err != nil {
		t.Fatalf("postgres connection detail: %v", err)
	}

	conn, err := pgx.Connect(ctx, connString)
	if err != nil {
		t.Fatalf("connect postgres: %v", err)
	}
	t.Cleanup(func() {
		if err := conn.Close(context.Background()); err != nil {
			t.Fatalf("close postgres connection: %v", err)
		}
	})

	var value int
	if err := conn.QueryRow(ctx, "select 1").Scan(&value); err != nil {
		t.Fatalf("query postgres: %v", err)
	}
	if value != 1 {
		t.Fatalf("expected postgres query result 1, got %d", value)
	}
}

func TestConnectionDetailKey(t *testing.T) {
	if postgrestestcontainer.ConnectionStringKey != "postgres.connection-string" {
		t.Fatalf("ConnectionStringKey = %q", postgrestestcontainer.ConnectionStringKey)
	}
}
