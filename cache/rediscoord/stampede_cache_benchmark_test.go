package rediscoord

import (
	"context"
	"errors"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"github.com/bluetape4k/bluetape-go/cache"
	"github.com/redis/go-redis/v9"
	"github.com/testcontainers/testcontainers-go"
	tcredis "github.com/testcontainers/testcontainers-go/modules/redis"
	"github.com/testcontainers/testcontainers-go/wait"
)

func BenchmarkStampedeCacheGetOrLoadHot(b *testing.B) {
	ctx := context.Background()
	client := benchmarkRedisClient(ctx, b)
	coord := benchmarkStampedeCache[int](b, client, "bench-hot")
	if _, err := coord.GetOrLoad(ctx, "item", time.Minute, func(context.Context, string) (int, error) {
		return 42, nil
	}); err != nil {
		b.Fatalf("seed hot value: %v", err)
	}

	b.ReportAllocs()
	b.ResetTimer()

	for range b.N {
		value, err := coord.GetOrLoad(ctx, "item", time.Minute, func(context.Context, string) (int, error) {
			b.Fatal("hot path should not call loader")
			return 0, nil
		})
		if err != nil {
			b.Fatalf("get hot value: %v", err)
		}
		if value != 42 {
			b.Fatalf("value = %d, want 42", value)
		}
	}
}

func BenchmarkStampedeCacheGetOrLoadColdWinner(b *testing.B) {
	ctx := context.Background()
	client := benchmarkRedisClient(ctx, b)
	coord := benchmarkStampedeCache[int](b, client, "bench-cold")
	var loads int64

	b.ReportAllocs()
	b.ResetTimer()

	for i := range b.N {
		key := "item-" + strconv.Itoa(i)
		value, err := coord.GetOrLoad(ctx, key, time.Minute, func(context.Context, string) (int, error) {
			atomic.AddInt64(&loads, 1)
			return i, nil
		})
		if err != nil {
			b.Fatalf("get cold value: %v", err)
		}
		if value != i {
			b.Fatalf("value = %d, want %d", value, i)
		}
	}

	b.StopTimer()
	b.ReportMetric(float64(loads)/float64(b.N), "loads/op")
}

func benchmarkStampedeCache[V any](b *testing.B, client redis.Cmdable, namespace string) *StampedeCache[V] {
	b.Helper()

	coord, err := NewStampedeCache[V](Options[V]{
		Client:       client,
		Cache:        cache.NewMemory[string, V](),
		Namespace:    namespace,
		Codec:        JSONCodec[V]{},
		LockTTL:      time.Second,
		ResultTTL:    time.Second,
		PollInterval: 5 * time.Millisecond,
	})
	if err != nil {
		b.Fatalf("new stampede cache: %v", err)
	}
	return coord
}

func benchmarkRedisClient(ctx context.Context, b *testing.B) *redis.Client {
	b.Helper()

	addr := benchmarkRedisServer(ctx, b)
	client := redis.NewClient(&redis.Options{Addr: addr})
	b.Cleanup(func() {
		_ = client.Close()
	})
	waitBenchmarkRedis(ctx, b, client)
	return client
}

func benchmarkRedisServer(ctx context.Context, b *testing.B) string {
	b.Helper()

	container, err := tcredis.Run(ctx, "redis:7.4-alpine", testcontainers.WithWaitStrategy(
		wait.ForLog("Ready to accept connections"),
	))
	if err != nil {
		b.Fatalf("start redis container: %v", err)
	}
	b.Cleanup(func() {
		if err := container.Terminate(context.Background()); err != nil && !errors.Is(err, context.Canceled) {
			b.Fatalf("terminate redis container: %v", err)
		}
	})

	host, err := container.Host(ctx)
	if err != nil {
		b.Fatalf("redis container host: %v", err)
	}
	port, err := container.MappedPort(ctx, "6379/tcp")
	if err != nil {
		b.Fatalf("redis container port: %v", err)
	}
	return host + ":" + port.Port()
}

func waitBenchmarkRedis(ctx context.Context, b *testing.B, client *redis.Client) {
	b.Helper()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if client.Ping(ctx).Err() == nil {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	b.Fatal("redis did not become ready")
}
