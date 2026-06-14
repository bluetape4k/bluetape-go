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
)

// Start 는 PostgreSQL test container를 시작하고 connection string을 반환한다.
func Start(ctx context.Context, t *testing.T) string {
	t.Helper()

	container, err := tcpostgres.Run(
		ctx,
		defaultImage,
		tcpostgres.WithDatabase(defaultDatabase),
		tcpostgres.WithUsername(defaultUsername),
		tcpostgres.WithPassword(defaultPassword),
		tcpostgres.BasicWaitStrategies(),
	)
	if err != nil {
		t.Fatalf("start postgres container: %v", err)
	}

	testcleanup.Register(ctx, t, "postgres", container)

	connString, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("postgres connection string: %v", err)
	}
	return connString
}
