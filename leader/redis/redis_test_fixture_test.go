package redisleader_test

import (
	"context"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/bluetape4k/bluetape-go/internal/testcleanup"
	"github.com/redis/go-redis/v9"
	tcredis "github.com/testcontainers/testcontainers-go/modules/redis"
)

var redisFixture struct {
	once      sync.Once
	container *tcredis.RedisContainer
	addr      string
	err       error
}

func redisAddr(ctx context.Context) (string, error) {
	redisFixture.once.Do(func() {
		container, err := tcredis.Run(ctx, "redis:7.4-alpine")
		if err != nil {
			redisFixture.err = err
			return
		}
		addr, err := container.PortEndpoint(ctx, "6379/tcp", "")
		if err != nil {
			redisFixture.err = err
			_ = testcleanup.Terminate(ctx, 0, container)
			return
		}
		redisFixture.container = container
		redisFixture.addr = addr
	})
	return redisFixture.addr, redisFixture.err
}

func newRedisClient(ctx context.Context, t *testing.T) *redis.Client {
	t.Helper()
	addr, err := redisAddr(ctx)
	if err != nil {
		t.Fatalf("start redis container: %v", err)
	}
	client := redis.NewClient(&redis.Options{Addr: addr})
	t.Cleanup(func() {
		_ = client.Close()
	})
	waitForRedis(t, client)
	if err := client.FlushDB(ctx).Err(); err != nil {
		t.Fatalf("flush redis db: %v", err)
	}
	return client
}

func waitForRedis(t *testing.T, client *redis.Client) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	var lastErr error
	for time.Now().Before(deadline) {
		err := client.Ping(context.Background()).Err()
		if err == nil {
			return
		}
		lastErr = err
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatalf("redis did not become ready: %v", lastErr)
}

func TestMain(m *testing.M) {
	code := m.Run()
	if redisFixture.container != nil {
		_ = testcleanup.Terminate(context.Background(), 0, redisFixture.container)
	}
	os.Exit(code)
}
