package redistestcontainer

import (
	"context"
	"testing"

	"github.com/testcontainers/testcontainers-go"
	tcredis "github.com/testcontainers/testcontainers-go/modules/redis"
	"github.com/testcontainers/testcontainers-go/wait"
)

// Start launches a Redis test container and returns its connection address.
func Start(ctx context.Context, t *testing.T) string {
	t.Helper()

	container, err := tcredis.Run(ctx, "redis:7.4-alpine", testcontainers.WithWaitStrategy(
		wait.ForLog("Ready to accept connections"),
	))
	if err != nil {
		t.Fatalf("start redis container: %v", err)
	}

	t.Cleanup(func() {
		if err := container.Terminate(context.Background()); err != nil {
			t.Fatalf("terminate redis container: %v", err)
		}
	})

	host, err := container.Host(ctx)
	if err != nil {
		t.Fatalf("redis container host: %v", err)
	}
	port, err := container.MappedPort(ctx, "6379/tcp")
	if err != nil {
		t.Fatalf("redis container port: %v", err)
	}
	return host + ":" + port.Port()
}
