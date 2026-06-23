package postgrestestcontainer

import (
	"context"
	"testing"

	"github.com/bluetape4k/bluetape-go/internal/testcleanup"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
)

const (
	defaultImage    = "postgres:16-alpine"
	defaultDatabase = "bluetape"
	defaultUsername = "bluetape"
	defaultPassword = "bluetape"

	// ConnectionStringKey is the documented key for a PostgreSQL connection string.
	ConnectionStringKey = "postgres.connection-string"
)

// Start launches a PostgreSQL test container and returns its connection string.
func Start(ctx context.Context, tb testing.TB) string {
	tb.Helper()

	container, err := tcpostgres.Run(
		ctx,
		defaultImage,
		tcpostgres.WithDatabase(defaultDatabase),
		tcpostgres.WithUsername(defaultUsername),
		tcpostgres.WithPassword(defaultPassword),
		tcpostgres.BasicWaitStrategies(),
	)
	if err != nil {
		tb.Fatal(testcleanup.FormatStartError("postgres", defaultImage, err))
	}

	testcleanup.Register(ctx, tb, "postgres", container)

	connString, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		tb.Fatalf("%s: %v", ConnectionStringKey, err)
	}
	return connString
}
