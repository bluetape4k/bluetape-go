package jwt

import (
	"context"
	"os"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/bluetape4k/bluetape-go/cache"
	"github.com/bluetape4k/bluetape-go/internal/testcleanup"
	"github.com/redis/go-redis/v9"
	tcredis "github.com/testcontainers/testcontainers-go/modules/redis"
)

const (
	benchmarkJWTRedisEnv   = "BLUETAPE_JWT_REDIS_BENCH"
	benchmarkJWTRedisImage = "redis:7.4-alpine"
)

func BenchmarkRedisRepositoryFind(b *testing.B) {
	ctx := context.Background()
	client := benchmarkJWTRedisClient(ctx, b)
	repo := benchmarkRedisRepository(b, client, "find", RedisRepositoryOptions{})
	now := time.Date(2026, 6, 12, 10, 0, 0, 0, time.UTC)
	key := benchmarkHMACKey(b, "find", now, time.Hour)
	benchmarkSeedRedisKeyChain(ctx, b, repo, key, true)

	b.ReportAllocs()
	b.ResetTimer()

	for range b.N {
		found, err := repo.Find(ctx, "find", now)
		if err != nil {
			b.Fatalf("Find() error = %v", err)
		}
		if found.KID() != "find" {
			b.Fatalf("Find() kid = %q, want find", found.KID())
		}
	}
}

func BenchmarkRedisRepositoryFindParallel(b *testing.B) {
	ctx := context.Background()
	client := benchmarkJWTRedisClient(ctx, b)
	repo := benchmarkRedisRepository(b, client, "find-parallel", RedisRepositoryOptions{})
	now := time.Date(2026, 6, 12, 10, 0, 0, 0, time.UTC)
	key := benchmarkHMACKey(b, "find-parallel", now, time.Hour)
	benchmarkSeedRedisKeyChain(ctx, b, repo, key, true)

	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			found, err := repo.Find(ctx, "find-parallel", now)
			if err != nil {
				b.Fatalf("Find() error = %v", err)
			}
			if found.KID() != "find-parallel" {
				b.Fatalf("Find() kid = %q, want find-parallel", found.KID())
			}
		}
	})
}

func BenchmarkRedisRepositoryFindRetainedParallel(b *testing.B) {
	ctx := context.Background()
	client := benchmarkJWTRedisClient(ctx, b)
	repo := benchmarkRedisRepository(b, client, "find-retained-parallel", RedisRepositoryOptions{Capacity: 1000})
	now := time.Date(2026, 6, 12, 10, 0, 0, 0, time.UTC)
	retained := benchmarkHMACKey(b, "retained", now.Add(-time.Minute), time.Hour)
	benchmarkSeedRedisKeyChain(ctx, b, repo, retained, false)
	benchmarkSeedRedisKeyChain(ctx, b, repo, benchmarkHMACKey(b, "current", now, time.Hour), true)

	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			found, err := repo.Find(ctx, "retained", now)
			if err != nil {
				b.Fatalf("Find() error = %v", err)
			}
			if found.KID() != "retained" {
				b.Fatalf("Find() kid = %q, want retained", found.KID())
			}
		}
	})
}

func BenchmarkRedisRepositoryRotateCurrentHit(b *testing.B) {
	ctx := context.Background()
	client := benchmarkJWTRedisClient(ctx, b)
	repo := benchmarkRedisRepository(b, client, "rotate-current", RedisRepositoryOptions{})
	now := time.Date(2026, 6, 12, 10, 0, 0, 0, time.UTC)
	benchmarkSeedRedisKeyChain(ctx, b, repo, benchmarkHMACKey(b, "current", now, time.Hour), true)

	b.ReportAllocs()
	b.ResetTimer()

	for range b.N {
		key, err := repo.Rotate(ctx, func() (*KeyChain, error) {
			b.Fatal("create must not run on current-hit rotate")
			return nil, nil
		}, now)
		if err != nil {
			b.Fatalf("Rotate() error = %v", err)
		}
		if key.KID() != "current" {
			b.Fatalf("Rotate() kid = %q, want current", key.KID())
		}
	}
}

func BenchmarkRedisRepositoryRotateExpired(b *testing.B) {
	ctx := context.Background()
	client := benchmarkJWTRedisClient(ctx, b)
	repo := benchmarkRedisRepository(b, client, "rotate-expired", RedisRepositoryOptions{})
	now := time.Date(2026, 6, 12, 10, 0, 0, 0, time.UTC)

	b.ReportAllocs()
	b.ResetTimer()

	for i := range b.N {
		b.StopTimer()
		if err := repo.DeleteAll(ctx); err != nil {
			b.Fatalf("DeleteAll() error = %v", err)
		}
		expired := benchmarkHMACKey(b, "expired", now.Add(-2*time.Hour), time.Hour)
		benchmarkSeedRedisKeyChain(ctx, b, repo, expired, true)
		candidate := "candidate-" + strconv.Itoa(i)
		b.StartTimer()

		key, err := repo.Rotate(ctx, func() (*KeyChain, error) {
			return benchmarkHMACKey(b, candidate, now, time.Hour), nil
		}, now)
		if err != nil {
			b.Fatalf("Rotate() error = %v", err)
		}
		if key.KID() != candidate {
			b.Fatalf("Rotate() kid = %q, want %q", key.KID(), candidate)
		}
	}
}

func BenchmarkRedisRepositoryForcedRotate(b *testing.B) {
	ctx := context.Background()
	client := benchmarkJWTRedisClient(ctx, b)
	repo := benchmarkRedisRepository(b, client, "forced-rotate", RedisRepositoryOptions{Capacity: 1000})
	now := time.Date(2026, 6, 12, 10, 0, 0, 0, time.UTC)

	b.ReportAllocs()
	b.ResetTimer()

	for i := range b.N {
		kid := "forced-" + strconv.Itoa(i)
		key, err := repo.ForcedRotate(ctx, func() (*KeyChain, error) {
			return benchmarkHMACKey(b, kid, now.Add(time.Duration(i)*time.Nanosecond), time.Hour), nil
		}, now)
		if err != nil {
			b.Fatalf("ForcedRotate() error = %v", err)
		}
		if key.KID() != kid {
			b.Fatalf("ForcedRotate() kid = %q, want %q", key.KID(), kid)
		}
	}
}

func BenchmarkRedisRepositoryForcedRotateParallel(b *testing.B) {
	ctx := context.Background()
	client := benchmarkJWTRedisClient(ctx, b)
	repo := benchmarkRedisRepository(b, client, "forced-rotate-parallel", RedisRepositoryOptions{Capacity: 1000})
	now := time.Date(2026, 6, 12, 10, 0, 0, 0, time.UTC)
	var next atomic.Int64

	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			id := next.Add(1)
			kid := "forced-parallel-" + strconv.FormatInt(id, 10)
			key, err := repo.ForcedRotate(ctx, func() (*KeyChain, error) {
				return benchmarkHMACKey(b, kid, now.Add(time.Duration(id)*time.Nanosecond), time.Hour), nil
			}, now)
			if err != nil {
				b.Fatalf("ForcedRotate() error = %v", err)
			}
			if key.KID() != kid {
				b.Fatalf("ForcedRotate() kid = %q, want %q", key.KID(), kid)
			}
		}
	})
}

func BenchmarkRedisDistributedProviderComposeContext(b *testing.B) {
	ctx := context.Background()
	client := benchmarkJWTRedisClient(ctx, b)
	now := time.Date(2026, 6, 12, 10, 0, 0, 0, time.UTC)
	repo := benchmarkRedisRepository(b, client, "compose", RedisRepositoryOptions{Capacity: 1000})
	provider, err := NewDistributedHMACProvider(ctx, repo, HS256,
		WithClock(func() time.Time { return now }),
		WithKeyIDGenerator(sequenceKID("compose")),
		WithEntropy(repeatingReader('c')),
	)
	if err != nil {
		b.Fatalf("NewDistributedHMACProvider() error = %v", err)
	}

	b.ReportAllocs()
	b.ResetTimer()

	for range b.N {
		token, err := provider.ComposeContext(ctx, WithClaim("bench", "compose"), WithExpiresAfter(time.Hour))
		if err != nil {
			b.Fatalf("ComposeContext() error = %v", err)
		}
		if token == "" {
			b.Fatal("ComposeContext() returned empty token")
		}
	}
}

func BenchmarkRedisDistributedProviderComposeContextParallel(b *testing.B) {
	ctx := context.Background()
	client := benchmarkJWTRedisClient(ctx, b)
	now := time.Date(2026, 6, 12, 10, 0, 0, 0, time.UTC)
	repo := benchmarkRedisRepository(b, client, "compose-parallel", RedisRepositoryOptions{Capacity: 1000})
	provider, err := NewDistributedHMACProvider(ctx, repo, HS256,
		WithClock(func() time.Time { return now }),
		WithKeyIDGenerator(sequenceKID("compose-parallel")),
		WithEntropy(repeatingReader('c')),
	)
	if err != nil {
		b.Fatalf("NewDistributedHMACProvider() error = %v", err)
	}

	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			token, err := provider.ComposeContext(ctx, WithClaim("bench", "compose-parallel"), WithExpiresAfter(time.Hour))
			if err != nil {
				b.Fatalf("ComposeContext() error = %v", err)
			}
			if token == "" {
				b.Fatal("ComposeContext() returned empty token")
			}
		}
	})
}

func BenchmarkRedisDistributedProviderParseContext(b *testing.B) {
	ctx := context.Background()
	client := benchmarkJWTRedisClient(ctx, b)
	now := time.Date(2026, 6, 12, 10, 0, 0, 0, time.UTC)
	repo := benchmarkRedisRepository(b, client, "parse", RedisRepositoryOptions{Capacity: 1000})
	provider, err := NewDistributedHMACProvider(ctx, repo, HS256,
		WithClock(func() time.Time { return now }),
		WithKeyIDGenerator(staticKID("parse")),
		WithEntropy(repeatingReader('p')),
	)
	if err != nil {
		b.Fatalf("NewDistributedHMACProvider() error = %v", err)
	}
	token, err := provider.ComposeContext(ctx, WithClaim("bench", "parse"), WithExpiresAfter(time.Hour))
	if err != nil {
		b.Fatalf("ComposeContext() error = %v", err)
	}

	b.ReportAllocs()
	b.ResetTimer()

	for range b.N {
		reader, err := provider.ParseContext(ctx, token, WithParseClock(func() time.Time { return now }))
		if err != nil {
			b.Fatalf("ParseContext() error = %v", err)
		}
		if value, ok := reader.ClaimString("bench"); !ok || value != "parse" {
			b.Fatal("ParseContext() claim mismatch")
		}
	}
}

func BenchmarkRedisDistributedProviderParseContextParallel(b *testing.B) {
	ctx := context.Background()
	client := benchmarkJWTRedisClient(ctx, b)
	now := time.Date(2026, 6, 12, 10, 0, 0, 0, time.UTC)
	repo := benchmarkRedisRepository(b, client, "parse-parallel", RedisRepositoryOptions{Capacity: 1000})
	provider, err := NewDistributedHMACProvider(ctx, repo, HS256,
		WithClock(func() time.Time { return now }),
		WithKeyIDGenerator(staticKID("parse-parallel")),
		WithEntropy(repeatingReader('p')),
	)
	if err != nil {
		b.Fatalf("NewDistributedHMACProvider() error = %v", err)
	}
	token, err := provider.ComposeContext(ctx, WithClaim("bench", "parse-parallel"), WithExpiresAfter(time.Hour))
	if err != nil {
		b.Fatalf("ComposeContext() error = %v", err)
	}

	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			reader, err := provider.ParseContext(ctx, token, WithParseClock(func() time.Time { return now }))
			if err != nil {
				b.Fatalf("ParseContext() error = %v", err)
			}
			if value, ok := reader.ClaimString("bench"); !ok || value != "parse-parallel" {
				b.Fatal("ParseContext() claim mismatch")
			}
		}
	})
}

func BenchmarkRedisCachedDistributedProviderParseContextWarmHit(b *testing.B) {
	ctx := context.Background()
	client := benchmarkJWTRedisClient(ctx, b)
	now := time.Date(2026, 6, 12, 10, 0, 0, 0, time.UTC)
	repo := benchmarkRedisRepository(b, client, "cached-parse", RedisRepositoryOptions{Capacity: 1000})
	provider, err := NewDistributedHMACProvider(ctx, repo, HS256,
		WithClock(func() time.Time { return now }),
		WithKeyIDGenerator(staticKID("cached-parse")),
		WithEntropy(repeatingReader('c')),
	)
	if err != nil {
		b.Fatalf("NewDistributedHMACProvider() error = %v", err)
	}
	readerCache := cache.NewMemory[string, *Reader]()
	cached, err := NewCachedDistributedProvider(provider, readerCache,
		WithCacheClock(func() time.Time { return now }),
		WithCacheTrustScope("redis-benchmark"),
	)
	if err != nil {
		b.Fatalf("NewCachedDistributedProvider() error = %v", err)
	}
	token, err := cached.ComposeContext(ctx, WithClaim("bench", "cached-parse"), WithExpiresAfter(time.Hour))
	if err != nil {
		b.Fatalf("ComposeContext() error = %v", err)
	}
	if _, err := cached.ParseContext(ctx, token); err != nil {
		b.Fatalf("warmup ParseContext() error = %v", err)
	}

	b.ReportAllocs()
	b.ResetTimer()

	for range b.N {
		reader, err := cached.ParseContext(ctx, token)
		if err != nil {
			b.Fatalf("ParseContext() error = %v", err)
		}
		if value, ok := reader.ClaimString("bench"); !ok || value != "cached-parse" {
			b.Fatal("ParseContext() claim mismatch")
		}
	}
}

func BenchmarkRedisCachedDistributedProviderParseContextWarmHitParallel(b *testing.B) {
	ctx := context.Background()
	client := benchmarkJWTRedisClient(ctx, b)
	now := time.Date(2026, 6, 12, 10, 0, 0, 0, time.UTC)
	repo := benchmarkRedisRepository(b, client, "cached-parse-parallel", RedisRepositoryOptions{Capacity: 1000})
	provider, err := NewDistributedHMACProvider(ctx, repo, HS256,
		WithClock(func() time.Time { return now }),
		WithKeyIDGenerator(staticKID("cached-parse-parallel")),
		WithEntropy(repeatingReader('c')),
	)
	if err != nil {
		b.Fatalf("NewDistributedHMACProvider() error = %v", err)
	}
	readerCache := cache.NewMemory[string, *Reader]()
	cached, err := NewCachedDistributedProvider(provider, readerCache,
		WithCacheClock(func() time.Time { return now }),
		WithCacheTrustScope("redis-benchmark-parallel"),
	)
	if err != nil {
		b.Fatalf("NewCachedDistributedProvider() error = %v", err)
	}
	token, err := cached.ComposeContext(ctx, WithClaim("bench", "cached-parse-parallel"), WithExpiresAfter(time.Hour))
	if err != nil {
		b.Fatalf("ComposeContext() error = %v", err)
	}
	if _, err := cached.ParseContext(ctx, token); err != nil {
		b.Fatalf("warmup ParseContext() error = %v", err)
	}

	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			reader, err := cached.ParseContext(ctx, token)
			if err != nil {
				b.Fatalf("ParseContext() error = %v", err)
			}
			if value, ok := reader.ClaimString("bench"); !ok || value != "cached-parse-parallel" {
				b.Fatal("ParseContext() claim mismatch")
			}
		}
	})
}

func benchmarkRedisRepository(b *testing.B, client redis.Cmdable, namespace string, options RedisRepositoryOptions) *RedisRepository {
	b.Helper()
	if options.Client == nil {
		options.Client = client
	}
	if options.Namespace == "" {
		options.Namespace = benchmarkRedisNamespace(b, namespace)
	}
	repo, err := NewRedisRepository(options)
	if err != nil {
		b.Fatalf("NewRedisRepository() error = %v", err)
	}
	return repo
}

func benchmarkJWTRedisClient(ctx context.Context, b *testing.B) *redis.Client {
	b.Helper()
	requireJWTRedisBenchmark(b)
	container, err := tcredis.Run(ctx, benchmarkJWTRedisImage)
	if err != nil {
		b.Fatalf("start redis container: %v", err)
	}
	testcleanup.Register(ctx, b, "redis", container)
	addr, err := container.PortEndpoint(ctx, "6379/tcp", "")
	if err != nil {
		b.Fatalf("redis container endpoint: %v", err)
	}
	client := redis.NewClient(&redis.Options{Addr: addr})
	b.Cleanup(func() { _ = client.Close() })
	waitBenchmarkJWTRedis(ctx, b, client)
	return client
}

func requireJWTRedisBenchmark(b *testing.B) {
	b.Helper()
	if os.Getenv(benchmarkJWTRedisEnv) != "1" {
		b.Skipf("set %s=1 to run Redis/Testcontainers JWT benchmarks serially", benchmarkJWTRedisEnv)
	}
}

func waitBenchmarkJWTRedis(ctx context.Context, b *testing.B, client *redis.Client) {
	b.Helper()
	deadline := time.Now().Add(10 * time.Second)
	var lastErr error
	for time.Now().Before(deadline) {
		err := client.Ping(ctx).Err()
		if err == nil {
			return
		}
		lastErr = err
		time.Sleep(10 * time.Millisecond)
	}
	b.Fatalf("redis did not become ready: %v", lastErr)
}

func benchmarkRedisNamespace(b *testing.B, namespace string) string {
	b.Helper()
	name := strings.NewReplacer("/", "-", " ", "-", "_", "-", "(", "-", ")", "-").Replace(b.Name())
	return namespace + "-" + name
}

func benchmarkHMACKey(b *testing.B, kid string, createdAt time.Time, ttl time.Duration) *KeyChain {
	b.Helper()
	key, err := newHMACKeyChain(kid, HS256, bytesOf('b', 32), createdAt, ttl)
	if err != nil {
		b.Fatalf("newHMACKeyChain() error = %v", err)
	}
	return key
}

func benchmarkSeedRedisKeyChain(ctx context.Context, b *testing.B, repo *RedisRepository, key *KeyChain, current bool) {
	b.Helper()
	payload, err := encodeRedisKeyChain(key)
	if err != nil {
		b.Fatalf("encodeRedisKeyChain() error = %v", err)
	}
	if err := repo.client.HSet(ctx, repo.opts.keysKey(), key.KID(), string(payload)).Err(); err != nil {
		b.Fatalf("HSet() error = %v", err)
	}
	if err := repo.client.ZAdd(ctx, repo.opts.orderKey(), redis.Z{Score: float64(key.CreatedAt().UnixNano()), Member: key.KID()}).Err(); err != nil {
		b.Fatalf("ZAdd() error = %v", err)
	}
	if err := repo.client.HSet(ctx, repo.opts.metaKey(), "version", redisKeyVersion, "algorithm", string(key.Algorithm())).Err(); err != nil {
		b.Fatalf("HSet(meta) error = %v", err)
	}
	if current {
		if err := repo.client.Set(ctx, repo.opts.currentKey(), key.KID(), 0).Err(); err != nil {
			b.Fatalf("Set(current) error = %v", err)
		}
	}
}
