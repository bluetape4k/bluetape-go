package postgistestcontainer_test

import (
	"context"
	"testing"
	"time"

	postgistestcontainer "github.com/bluetape4k/bluetape-go/testcontainers/postgis"
	"github.com/jackc/pgx/v5"
)

func TestStartPostGIS(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	t.Cleanup(cancel)
	dsn := postgistestcontainer.Start(ctx, t)
	conn, err := pgx.Connect(ctx, dsn)
	if err != nil {
		t.Fatalf("connect postgis: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close(context.Background()) })
	if _, err := conn.Exec(ctx, "CREATE EXTENSION IF NOT EXISTS postgis"); err != nil {
		t.Fatalf("create postgis extension: %v", err)
	}
	var value bool
	if err := conn.QueryRow(ctx, "SELECT postgis_full_version() IS NOT NULL").Scan(&value); err != nil {
		t.Fatalf("query postgis: %v", err)
	}
	if !value {
		t.Fatalf("postgis extension is unavailable")
	}
}

func TestConnectionDetailKey(t *testing.T) {
	if postgistestcontainer.ConnectionStringKey != "postgis.connection-string" {
		t.Fatalf("ConnectionStringKey = %q", postgistestcontainer.ConnectionStringKey)
	}
}
