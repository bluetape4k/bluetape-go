package redisnear

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	bttesting "github.com/bluetape4k/bluetape-go/testing"
	"github.com/redis/go-redis/v9"
	"github.com/testcontainers/testcontainers-go"
	tcredis "github.com/testcontainers/testcontainers-go/modules/redis"
	"github.com/testcontainers/testcontainers-go/wait"
)

func TestNearCacheClearsLocalOnRedisOutage(t *testing.T) {
	ctx := context.Background()
	addr, terminateRedis := redisServer(ctx, t)
	client := redis.NewClient(&redis.Options{Addr: addr})
	t.Cleanup(func() {
		_ = client.Close()
	})
	waitForRedis(t, client)

	var reports int32
	near, err := NewPubSub[string](ctx, Options[string]{
		Client:    client,
		Namespace: "redis-outage",
		OnError: func(context.Context, error) {
			atomic.AddInt32(&reports, 1)
		},
	})
	if err != nil {
		t.Fatalf("new cache: %v", err)
	}
	t.Cleanup(func() {
		_ = near.Close()
	})

	if _, err := near.GetOrLoad(ctx, "item", 0, func(context.Context, string) (string, error) {
		return "stale", nil
	}); err != nil {
		t.Fatalf("prime local cache: %v", err)
	}

	terminateRedis()

	bttesting.Eventually(t, 5*time.Second, func() bool {
		return atomic.LoadInt32(&reports) > 0
	})
	assertEventuallyMiss(t, near, "item")
}

func TestNearCacheRecreateAfterRedisOutageRestoresPeerInvalidation(t *testing.T) {
	ctx := context.Background()
	addr, terminateRedis := redisServer(ctx, t)
	client := redis.NewClient(&redis.Options{Addr: addr})
	waitForRedis(t, client)

	var reports int32
	near, err := NewPubSub[string](ctx, Options[string]{
		Client:    client,
		Namespace: "recreate-before",
		OnError: func(context.Context, error) {
			atomic.AddInt32(&reports, 1)
		},
	})
	if err != nil {
		t.Fatalf("new cache before outage: %v", err)
	}
	if _, err := near.GetOrLoad(ctx, "item", 0, func(context.Context, string) (string, error) {
		return "stale", nil
	}); err != nil {
		t.Fatalf("prime cache before outage: %v", err)
	}

	terminateRedis()
	bttesting.Eventually(t, 5*time.Second, func() bool {
		return atomic.LoadInt32(&reports) > 0
	})
	assertEventuallyMiss(t, near, "item")
	_ = near.Close()
	_ = client.Close()

	nextAddr, _ := redisServer(ctx, t)
	clientA := redis.NewClient(&redis.Options{Addr: nextAddr})
	clientB := redis.NewClient(&redis.Options{Addr: nextAddr})
	t.Cleanup(func() {
		_ = clientA.Close()
		_ = clientB.Close()
	})
	waitForRedis(t, clientA)
	waitForRedis(t, clientB)

	cacheA, err := NewPubSub[string](ctx, Options[string]{
		Client:    clientA,
		Namespace: "recreate-after",
		OriginID:  "origin-a",
	})
	if err != nil {
		t.Fatalf("new cache a after outage: %v", err)
	}
	t.Cleanup(func() {
		_ = cacheA.Close()
	})
	cacheB, err := NewPubSub[string](ctx, Options[string]{
		Client:    clientB,
		Namespace: "recreate-after",
		OriginID:  "origin-b",
	})
	if err != nil {
		t.Fatalf("new cache b after outage: %v", err)
	}
	t.Cleanup(func() {
		_ = cacheB.Close()
	})

	if _, err := cacheB.GetOrLoad(ctx, "item", 0, func(context.Context, string) (string, error) {
		return "stale", nil
	}); err != nil {
		t.Fatalf("prime recreated peer cache: %v", err)
	}
	if err := cacheA.Set(ctx, "item", "fresh", 0); err != nil {
		t.Fatalf("set recreated cache: %v", err)
	}
	assertEventuallyMiss(t, cacheB, "item")

	value, err := cacheB.GetOrLoad(ctx, "item", 0, func(context.Context, string) (string, error) {
		return "fresh", nil
	})
	if err != nil {
		t.Fatalf("reload recreated peer cache: %v", err)
	}
	if value != "fresh" {
		t.Fatalf("recreated peer value = %q, want fresh", value)
	}
}

func redisServer(ctx context.Context, t *testing.T) (string, func()) {
	t.Helper()

	container, err := tcredis.Run(ctx, "redis:7.4-alpine", testcontainers.WithWaitStrategy(
		wait.ForLog("Ready to accept connections"),
	))
	if err != nil {
		t.Fatalf("start redis container: %v", err)
	}

	host, err := container.Host(ctx)
	if err != nil {
		t.Fatalf("redis container host: %v", err)
	}
	port, err := container.MappedPort(ctx, "6379/tcp")
	if err != nil {
		t.Fatalf("redis container port: %v", err)
	}

	var once sync.Once
	terminate := func() {
		once.Do(func() {
			if err := container.Terminate(context.Background()); err != nil && !errors.Is(err, context.Canceled) {
				t.Fatalf("terminate redis container: %v", err)
			}
		})
	}
	t.Cleanup(terminate)
	return host + ":" + port.Port(), terminate
}
