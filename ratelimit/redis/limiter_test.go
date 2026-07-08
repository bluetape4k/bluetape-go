package redisratelimit

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/bluetape4k/bluetape-go/ratelimit"
	redistestcontainer "github.com/bluetape4k/bluetape-go/testcontainers/redis"
	bttesting "github.com/bluetape4k/bluetape-go/testing"
	concurrencytest "github.com/bluetape4k/bluetape-go/testing/concurrency"
	"github.com/redis/go-redis/v9"
)

func TestNewRejectsInvalidOptions(t *testing.T) {
	client := redis.NewClient(&redis.Options{Addr: "127.0.0.1:0"})
	t.Cleanup(func() {
		_ = client.Close()
	})

	tests := []struct {
		name    string
		options Options
	}{
		{name: "missing client", options: Options{RatePerSecond: 1, Burst: 1}},
		{name: "blank namespace", options: Options{Client: client, Namespace: "  ", RatePerSecond: 1, Burst: 1}},
		{name: "zero rate", options: Options{Client: client, Burst: 1}},
		{name: "too small rate", options: Options{Client: client, RatePerSecond: 0.0000001, Burst: 1}},
		{name: "zero burst", options: Options{Client: client, RatePerSecond: 1}},
		{name: "negative idle ttl", options: Options{Client: client, RatePerSecond: 1, Burst: 1, IdleTTL: -time.Second}},
		{name: "too short idle ttl", options: Options{Client: client, RatePerSecond: 1, Burst: 10, IdleTTL: time.Second}},
		{name: "negative max key bytes", options: Options{Client: client, RatePerSecond: 1, Burst: 1, MaxKeyBytes: -1}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := New(tt.options); err == nil {
				t.Fatalf("expected invalid options error")
			}
		})
	}
}

func TestNewAppliesDefaults(t *testing.T) {
	client := redis.NewClient(&redis.Options{Addr: "127.0.0.1:0"})
	t.Cleanup(func() {
		_ = client.Close()
	})

	limiter, err := New(Options{Client: client, RatePerSecond: 10, Burst: 5})
	if err != nil {
		t.Fatalf("new limiter: %v", err)
	}
	if limiter.opts.namespace != defaultNamespace {
		t.Fatalf("namespace = %q, want %q", limiter.opts.namespace, defaultNamespace)
	}
	if limiter.opts.maxKeyBytes != defaultMaxKeyBytes {
		t.Fatalf("max key bytes = %d, want %d", limiter.opts.maxKeyBytes, defaultMaxKeyBytes)
	}
	if limiter.opts.idleTTL != minDefaultIdleTTL {
		t.Fatalf("idle ttl = %s, want %s", limiter.opts.idleTTL, minDefaultIdleTTL)
	}
}

func TestLimiterImplementsRootInterface(t *testing.T) {
	client := redis.NewClient(&redis.Options{Addr: "127.0.0.1:0"})
	t.Cleanup(func() {
		_ = client.Close()
	})
	limiter, err := New(Options{Client: client, RatePerSecond: 1, Burst: 1})
	if err != nil {
		t.Fatalf("new limiter: %v", err)
	}
	var _ ratelimit.Limiter = limiter
}

func TestLimiterAllowsBurstAndRejects(t *testing.T) {
	ctx := context.Background()
	client := redisClient(ctx, t)
	limiter := newLimiter(t, client, "burst", Options{RatePerSecond: 1, Burst: 2})

	for i := 0; i < 2; i++ {
		result, err := limiter.Allow(ctx, "user-1", 1)
		if err != nil {
			t.Fatalf("allow %d: %v", i, err)
		}
		if !result.Allowed {
			t.Fatalf("allow %d rejected: %+v", i, result)
		}
	}
	result, err := limiter.Allow(ctx, "user-1", 1)
	if err != nil {
		t.Fatalf("reject path returned error: %v", err)
	}
	if result.Allowed || result.RetryAfter <= 0 {
		t.Fatalf("unexpected rejection result: %+v", result)
	}
}

func TestLimiterRefillsFromRedisServerTime(t *testing.T) {
	ctx := context.Background()
	client := redisClient(ctx, t)
	limiter := newLimiter(t, client, "refill", Options{RatePerSecond: 20, Burst: 1})

	if _, err := limiter.Allow(ctx, "user-1", 1); err != nil {
		t.Fatalf("consume burst: %v", err)
	}
	time.Sleep(75 * time.Millisecond)
	result, err := limiter.Allow(ctx, "user-1", 1)
	if err != nil {
		t.Fatalf("allow after refill: %v", err)
	}
	if !result.Allowed {
		t.Fatalf("expected refill to allow request: %+v", result)
	}
}

func TestLimiterNamespacesAreIsolated(t *testing.T) {
	ctx := context.Background()
	client := redisClient(ctx, t)
	limiterA := newLimiter(t, client, "tenant-a", Options{RatePerSecond: 1, Burst: 1})
	limiterB := newLimiter(t, client, "tenant-b", Options{RatePerSecond: 1, Burst: 1})

	if result, err := limiterA.Allow(ctx, "shared", 1); err != nil || !result.Allowed {
		t.Fatalf("limiter a first allow result=%+v err=%v", result, err)
	}
	if result, err := limiterB.Allow(ctx, "shared", 1); err != nil || !result.Allowed {
		t.Fatalf("limiter b first allow result=%+v err=%v", result, err)
	}
}

func TestLimiterPreservesCallerOwnedKeys(t *testing.T) {
	ctx := context.Background()
	client := redisClient(ctx, t)
	limiter := newLimiter(t, client, "preserve-key", Options{RatePerSecond: 0.001, Burst: 1})

	first, err := limiter.Allow(ctx, "tenant:blue", 1)
	if err != nil {
		t.Fatalf("first allow: %v", err)
	}
	if !first.Allowed {
		t.Fatalf("first key should consume its own bucket: %+v", first)
	}
	second, err := limiter.Allow(ctx, " tenant:blue ", 1)
	if err != nil {
		t.Fatalf("spaced allow: %v", err)
	}
	if !second.Allowed {
		t.Fatalf("spaced key should use a distinct bucket, got %+v", second)
	}
	if exists := client.Exists(ctx, limiter.bucketKey("tenant:blue"), limiter.bucketKey(" tenant:blue ")).Val(); exists != 2 {
		t.Fatalf("expected both exact Redis bucket keys to exist, got %d", exists)
	}
}

func TestLimiterExpiresIdleBucketKey(t *testing.T) {
	ctx := context.Background()
	client := redisClient(ctx, t)
	limiter := newLimiter(t, client, "ttl", Options{
		RatePerSecond: 100,
		Burst:         1,
		IdleTTL:       50 * time.Millisecond,
	})
	key := limiter.bucketKey("user-1")

	if _, err := limiter.Allow(ctx, "user-1", 1); err != nil {
		t.Fatalf("allow: %v", err)
	}
	if ttl, err := client.PTTL(ctx, key).Result(); err != nil || ttl <= 0 {
		t.Fatalf("pttl = %s err=%v", ttl, err)
	}
	bttesting.Eventually(t, time.Second, func() bool {
		return client.Exists(ctx, key).Val() == 0
	})
}

func TestLimiterRejectsInvalidAllowInput(t *testing.T) {
	ctx := context.Background()
	client := redisClient(ctx, t)
	limiter := newLimiter(t, client, "invalid-input", Options{
		RatePerSecond: 1,
		Burst:         1,
		MaxKeyBytes:   4,
	})

	tests := []struct {
		name   string
		key    string
		tokens int64
	}{
		{name: "blank key", key: "  ", tokens: 1},
		{name: "too long key", key: "abcde", tokens: 1},
		{name: "zero tokens", key: "user", tokens: 0},
		{name: "over burst", key: "user", tokens: 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := limiter.Allow(ctx, tt.key, tt.tokens); err == nil {
				t.Fatalf("expected invalid allow input error")
			}
		})
	}
}

func TestLimiterReturnsContextCancellation(t *testing.T) {
	ctx := context.Background()
	client := redisClient(ctx, t)
	limiter := newLimiter(t, client, "cancel", Options{RatePerSecond: 1, Burst: 1})

	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := limiter.Allow(cancelled, "user-1", 1); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
}

func TestLimiterConcurrentClientsDoNotOverAdmitBurst(t *testing.T) {
	ctx := context.Background()
	addr := redistestcontainer.Start(ctx, t)
	clientA := redis.NewClient(&redis.Options{Addr: addr})
	clientB := redis.NewClient(&redis.Options{Addr: addr})
	t.Cleanup(func() {
		_ = clientA.Close()
		_ = clientB.Close()
	})
	waitForRedis(t, clientA)
	waitForRedis(t, clientB)

	limiterA := newLimiter(t, clientA, "stress", Options{RatePerSecond: 0.001, Burst: 10})
	limiterB := newLimiter(t, clientB, "stress", Options{RatePerSecond: 0.001, Burst: 10})
	var sequence int64
	var allowed int64
	task := func(ctx context.Context) error {
		limiter := limiterA
		if atomic.AddInt64(&sequence, 1)%2 == 0 {
			limiter = limiterB
		}
		result, err := limiter.Allow(ctx, "shared", 1)
		if err != nil {
			return err
		}
		if result.Allowed {
			atomic.AddInt64(&allowed, 1)
		}
		return nil
	}

	tester := concurrencytest.NewGoroutineStressTester(concurrencytest.Options{
		Workers:       16,
		RoundsPerTask: 8,
		Timeout:       10 * time.Second,
	})
	report := tester.RunT(t, task, task, task, task, task)
	if report.Completed != 40 {
		t.Fatalf("stress should complete every task, got %+v", report)
	}
	if got := atomic.LoadInt64(&allowed); got != 10 {
		t.Fatalf("allowed = %d, want 10", got)
	}
}

func TestLimiterAsyncJobTesterCoversCancellation(t *testing.T) {
	ctx := context.Background()
	client := redisClient(ctx, t)
	limiter := newLimiter(t, client, "async-cancel", Options{RatePerSecond: 1, Burst: 1})
	tester := concurrencytest.NewAsyncJobTester(concurrencytest.Options{
		Workers: 1,
		Timeout: time.Second,
	})

	report, err := tester.Run(ctx, func(ctx context.Context) error {
		cancelled, cancel := context.WithCancel(ctx)
		cancel()
		if _, err := limiter.Allow(cancelled, "user-1", 1); !errors.Is(err, context.Canceled) {
			return fmt.Errorf("expected context.Canceled, got %w", err)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("async cancellation failed: report=%+v err=%v", report, err)
	}
}

func redisClient(ctx context.Context, t *testing.T) *redis.Client {
	t.Helper()

	client := redis.NewClient(&redis.Options{Addr: redistestcontainer.Start(ctx, t)})
	t.Cleanup(func() {
		_ = client.Close()
	})
	waitForRedis(t, client)
	return client
}

func waitForRedis(t *testing.T, client *redis.Client) {
	t.Helper()

	bttesting.Eventually(t, 5*time.Second, func() bool {
		return client.Ping(context.Background()).Err() == nil
	})
}

func newLimiter(t *testing.T, client redis.Cmdable, namespace string, options Options) *Limiter {
	t.Helper()

	if options.Client == nil {
		options.Client = client
	}
	if options.Namespace == "" {
		options.Namespace = testNamespace(t, namespace)
	}
	limiter, err := New(options)
	if err != nil {
		t.Fatalf("new limiter: %v", err)
	}
	return limiter
}

func testNamespace(t *testing.T, namespace string) string {
	t.Helper()

	name := strings.NewReplacer("/", ":", " ", "-", "_", "-").Replace(t.Name())
	return namespace + ":" + name
}
