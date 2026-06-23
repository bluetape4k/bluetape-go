package postgrestestcontainer_test

import (
	"context"
	"testing"

	postgrestestcontainer "github.com/bluetape4k/bluetape-go/testcontainers/postgres"
	"github.com/jackc/pgx/v5"
)

func TestStartPostgres(t *testing.T) {
	ctx := context.Background()
	connString := postgrestestcontainer.Start(ctx, t)

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
