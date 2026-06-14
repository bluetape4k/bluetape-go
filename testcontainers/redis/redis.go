package redistestcontainer

import (
	"context"
	"testing"

	"github.com/bluetape4k/bluetape-go/internal/testcleanup"
	tcredis "github.com/testcontainers/testcontainers-go/modules/redis"
)

// Start launches a Redis test container and returns its connection address.
func Start(ctx context.Context, tb testing.TB) string {
	tb.Helper()

	container, err := tcredis.Run(ctx, "redis:7.4-alpine")
	if err != nil {
		tb.Fatalf("start redis container: %v", err)
	}

	testcleanup.Register(ctx, tb, "redis", container)

	addr, err := container.PortEndpoint(ctx, "6379/tcp", "")
	if err != nil {
		tb.Fatalf("redis container endpoint: %v", err)
	}
	return addr
}
