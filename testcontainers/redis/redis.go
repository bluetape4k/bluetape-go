package redistestcontainer

import (
	"context"
	"testing"

	"github.com/bluetape4k/bluetape-go/internal/testcleanup"
	tcredis "github.com/testcontainers/testcontainers-go/modules/redis"
)

const (
	defaultImage = "redis:7.4-alpine"

	// AddressKey is the documented key for a Redis host:port address.
	AddressKey = "redis.address"
)

// Start launches a Redis test container and returns its connection address.
func Start(ctx context.Context, tb testing.TB) string {
	tb.Helper()

	container, err := tcredis.Run(ctx, defaultImage)
	if err != nil {
		tb.Fatal(testcleanup.FormatStartError("redis", defaultImage, err))
	}

	testcleanup.Register(ctx, tb, "redis", container)

	addr, err := container.PortEndpoint(ctx, "6379/tcp", "")
	if err != nil {
		tb.Fatalf("%s: %v", AddressKey, err)
	}
	return addr
}
