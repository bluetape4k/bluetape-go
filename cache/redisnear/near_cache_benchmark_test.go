package redisnear

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

func BenchmarkNearCacheGetLocalHit(b *testing.B) {
	ctx := context.Background()
	client := benchmarkRedisClient(ctx, b)
	near := benchmarkNearCache[int](ctx, b, client, "bench-hit", "hit")
	if err := near.Set(ctx, "item", 42, time.Minute); err != nil {
		b.Fatalf("seed hit: %v", err)
	}

	b.ReportAllocs()
	b.ResetTimer()

	for range b.N {
		value, err := near.Get(ctx, "item")
		if err != nil {
			b.Fatalf("get hit: %v", err)
		}
		if value != 42 {
			b.Fatalf("value = %d, want 42", value)
		}
	}
}

func BenchmarkNearCacheGetLocalMiss(b *testing.B) {
	ctx := context.Background()
	client := benchmarkRedisClient(ctx, b)
	near := benchmarkNearCache[int](ctx, b, client, "bench-miss", "miss")

	b.ReportAllocs()
	b.ResetTimer()

	for range b.N {
		if _, err := near.Get(ctx, "missing"); !errors.Is(err, cache.ErrCacheMiss) {
			b.Fatalf("get miss: %v", err)
		}
	}
}

func BenchmarkNearCacheSetPublish(b *testing.B) {
	ctx := context.Background()
	client := benchmarkRedisClient(ctx, b)
	near := benchmarkNearCache[int](ctx, b, client, "bench-set", "set")

	b.ReportAllocs()
	b.ResetTimer()

	for i := range b.N {
		if err := near.Set(ctx, "item", i, time.Minute); err != nil {
			b.Fatalf("set publish: %v", err)
		}
	}
}

func BenchmarkNearCacheDeletePublish(b *testing.B) {
	ctx := context.Background()
	client := benchmarkRedisClient(ctx, b)
	near := benchmarkNearCache[int](ctx, b, client, "bench-delete", "delete")

	b.ReportAllocs()
	b.ResetTimer()

	for range b.N {
		if err := near.Delete(ctx, "item"); err != nil {
			b.Fatalf("delete publish: %v", err)
		}
	}
}

func BenchmarkNearCacheClearPublish(b *testing.B) {
	ctx := context.Background()
	client := benchmarkRedisClient(ctx, b)
	near := benchmarkNearCache[int](ctx, b, client, "bench-clear", "clear")

	b.ReportAllocs()
	b.ResetTimer()

	for range b.N {
		if err := near.Clear(ctx); err != nil {
			b.Fatalf("clear publish: %v", err)
		}
	}
}

func BenchmarkNearCachePeerInvalidation(b *testing.B) {
	ctx := context.Background()
	addr := benchmarkRedisServer(ctx, b)
	clientA, clientB := benchmarkRedisClientAt(ctx, b, addr), benchmarkRedisClientAt(ctx, b, addr)
	nearA := benchmarkNearCache[int](ctx, b, clientA, "bench-peer", "origin-a")
	nearB := benchmarkNearCache[int](ctx, b, clientB, "bench-peer", "origin-b")
	keys := benchmarkNearCacheKeys("peer", 1024)

	b.ReportAllocs()
	b.ResetTimer()

	for i := range b.N {
		key := keys[i&1023]
		if _, err := nearB.GetOrLoad(ctx, key, time.Minute, func(context.Context, string) (int, error) {
			return -1, nil
		}); err != nil {
			b.Fatalf("prime peer key: %v", err)
		}
		if err := nearA.Set(ctx, key, i, time.Minute); err != nil {
			b.Fatalf("publish peer invalidation: %v", err)
		}
		if err := waitBenchmarkMiss(ctx, nearB, key); err != nil {
			b.Fatalf("wait peer invalidation: %v", err)
		}
	}
}

func BenchmarkNearCacheGetOrLoadUnderInvalidation(b *testing.B) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	addr := benchmarkRedisServer(ctx, b)
	clientA, clientB := benchmarkRedisClientAt(ctx, b, addr), benchmarkRedisClientAt(ctx, b, addr)
	nearA := benchmarkNearCache[int64](ctx, b, clientA, "bench-pressure", "origin-a")
	nearB := benchmarkNearCache[int64](ctx, b, clientB, "bench-pressure", "origin-b")
	keys := benchmarkNearCacheKeys("pressure", 16)
	var loads int64
	var sequence int64

	done := make(chan struct{})
	go func() {
		defer close(done)
		index := 0
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}
			key := keys[index&15]
			if index%4 == 0 {
				_ = nearA.Clear(context.Background())
			} else {
				_ = nearA.Delete(context.Background(), key)
			}
			index++
			time.Sleep(100 * time.Microsecond)
		}
	}()

	b.Cleanup(func() {
		cancel()
		<-done
	})

	b.ReportAllocs()
	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			current := atomic.AddInt64(&sequence, 1)
			key := keys[int(current)&15]
			value, err := nearB.GetOrLoad(ctx, key, time.Millisecond, func(context.Context, string) (int64, error) {
				atomic.AddInt64(&loads, 1)
				return current, nil
			})
			if err != nil {
				b.Fatalf("get or load under invalidation: %v", err)
			}
			if value == 0 {
				b.Fatal("loaded value should not be zero")
			}
		}
	})

	b.StopTimer()
	b.ReportMetric(float64(loads)/float64(b.N), "loads/op")
}

func benchmarkRedisClient(ctx context.Context, b *testing.B) *redis.Client {
	b.Helper()

	addr := benchmarkRedisServer(ctx, b)
	return benchmarkRedisClientAt(ctx, b, addr)
}

func benchmarkRedisClientAt(ctx context.Context, b *testing.B, addr string) *redis.Client {
	b.Helper()

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

func benchmarkNearCache[V any](
	ctx context.Context,
	b *testing.B,
	client *redis.Client,
	namespace string,
	originID string,
) *NearCache[V] {
	b.Helper()

	near, err := NewPubSub[V](ctx, Options[V]{
		Client:    client,
		Namespace: namespace,
		OriginID:  originID,
	})
	if err != nil {
		b.Fatalf("new near cache: %v", err)
	}
	b.Cleanup(func() {
		_ = near.Close()
	})
	return near
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

func waitBenchmarkMiss[V any](ctx context.Context, near *NearCache[V], key string) error {
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		_, err := near.Get(ctx, key)
		if errors.Is(err, cache.ErrCacheMiss) {
			return nil
		}
		if err != nil {
			return err
		}
		time.Sleep(100 * time.Microsecond)
	}
	return errors.New("near cache entry was not invalidated")
}

func benchmarkNearCacheKeys(prefix string, size int) []string {
	keys := make([]string, size)
	for i := range keys {
		keys[i] = prefix + "-" + strconv.Itoa(i)
	}
	return keys
}
