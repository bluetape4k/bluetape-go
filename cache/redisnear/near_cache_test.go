package redisnear

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/bluetape4k/bluetape-go/cache"
	redistestcontainer "github.com/bluetape4k/bluetape-go/testcontainers/redis"
	bttesting "github.com/bluetape4k/bluetape-go/testing"
	concurrencytest "github.com/bluetape4k/bluetape-go/testing/concurrency"
	"github.com/redis/go-redis/v9"
)

func TestNewPubSubRejectsMissingClient(t *testing.T) {
	if _, err := NewPubSub[string](context.Background(), Options[string]{}); err == nil {
		t.Fatal("missing Redis client should fail")
	}
}

func TestNearCacheInvalidatesPeerEntries(t *testing.T) {
	ctx := context.Background()
	clientA, clientB := redisClients(ctx, t)

	cacheA, err := NewPubSub[string](ctx, Options[string]{
		Client:    clientA,
		Namespace: "peer-invalidation",
		OriginID:  "origin-a",
	})
	if err != nil {
		t.Fatalf("new cache a: %v", err)
	}
	t.Cleanup(func() {
		_ = cacheA.Close()
	})
	cacheB, err := NewPubSub[string](ctx, Options[string]{
		Client:    clientB,
		Namespace: "peer-invalidation",
		OriginID:  "origin-b",
	})
	if err != nil {
		t.Fatalf("new cache b: %v", err)
	}
	t.Cleanup(func() {
		_ = cacheB.Close()
	})

	value, err := cacheB.GetOrLoad(ctx, "item", 0, func(context.Context, string) (string, error) {
		return "stale", nil
	})
	if err != nil {
		t.Fatalf("prime peer cache: %v", err)
	}
	if value != "stale" {
		t.Fatalf("primed value should be stale, got %q", value)
	}

	if err := cacheA.Set(ctx, "item", "fresh", 0); err != nil {
		t.Fatalf("set cache a: %v", err)
	}
	assertEventuallyMiss(t, cacheB, "item")

	value, err = cacheB.GetOrLoad(ctx, "item", 0, func(context.Context, string) (string, error) {
		return "fresh", nil
	})
	if err != nil {
		t.Fatalf("reload peer cache: %v", err)
	}
	if value != "fresh" {
		t.Fatalf("reloaded value should be fresh, got %q", value)
	}

	if err := cacheA.Delete(ctx, "item"); err != nil {
		t.Fatalf("delete cache a: %v", err)
	}
	assertEventuallyMiss(t, cacheB, "item")

	if _, err := cacheB.GetOrLoad(ctx, "other", 0, func(context.Context, string) (string, error) {
		return "other-value", nil
	}); err != nil {
		t.Fatalf("prime second key: %v", err)
	}
	if err := cacheA.Clear(ctx); err != nil {
		t.Fatalf("clear cache a: %v", err)
	}
	assertEventuallyMiss(t, cacheB, "other")
}

func TestNearCacheIgnoresOwnOrigin(t *testing.T) {
	local := cache.NewMemory[string, string]()
	near := &NearCache[string]{
		cfg: config[string]{
			namespace: "own-origin",
			originID:  "origin-a",
			local:     local,
		},
	}
	if err := local.Set(context.Background(), "item", "value", 0); err != nil {
		t.Fatalf("seed local cache: %v", err)
	}
	payload, err := encodeMessage(invalidationMessage{
		Namespace: "own-origin",
		OriginID:  "origin-a",
		Operation: operationDelete,
		Key:       "item",
	})
	if err != nil {
		t.Fatalf("encode message: %v", err)
	}

	near.applyMessage(context.Background(), string(payload))

	value, err := local.Get(context.Background(), "item")
	if err != nil {
		t.Fatalf("own origin should not invalidate local value: %v", err)
	}
	if value != "value" {
		t.Fatalf("local value mismatch: %q", value)
	}
}

func TestNearCacheReportsMalformedMessages(t *testing.T) {
	var reports int32
	local := cache.NewMemory[string, string]()
	near := &NearCache[string]{
		cfg: config[string]{
			namespace: "malformed",
			originID:  "origin-a",
			local:     local,
			onError: func(context.Context, error) {
				atomic.AddInt32(&reports, 1)
			},
		},
	}

	near.applyMessage(context.Background(), "{")

	if atomic.LoadInt32(&reports) != 1 {
		t.Fatalf("malformed message should report once, got %d", reports)
	}
}

func TestNearCacheOnErrorDoesNotBlockSubscriber(t *testing.T) {
	ctx := context.Background()
	clientA, clientB := redisClients(ctx, t)
	const namespace = "onerror-nonblocking"
	blockErrorHandler := make(chan struct{})
	defer close(blockErrorHandler)
	var reports int32

	cacheA, err := NewPubSub[string](ctx, Options[string]{
		Client:    clientA,
		Namespace: namespace,
		OriginID:  "origin-a",
	})
	if err != nil {
		t.Fatalf("new cache a: %v", err)
	}
	t.Cleanup(func() {
		_ = cacheA.Close()
	})
	cacheB, err := NewPubSub[string](ctx, Options[string]{
		Client:    clientB,
		Namespace: namespace,
		OriginID:  "origin-b",
		OnError: func(context.Context, error) {
			atomic.AddInt32(&reports, 1)
			<-blockErrorHandler
		},
	})
	if err != nil {
		t.Fatalf("new cache b: %v", err)
	}
	t.Cleanup(func() {
		_ = cacheB.Close()
	})

	if _, err := cacheB.GetOrLoad(ctx, "item", 0, func(context.Context, string) (string, error) {
		return "stale", nil
	}); err != nil {
		t.Fatalf("prime peer cache: %v", err)
	}
	if err := clientA.Publish(ctx, defaultChannel(namespace), "{").Err(); err != nil {
		t.Fatalf("publish malformed message: %v", err)
	}
	bttesting.Eventually(t, 2*time.Second, func() bool {
		return atomic.LoadInt32(&reports) > 0
	})

	if err := cacheA.Set(ctx, "item", "fresh", 0); err != nil {
		t.Fatalf("set cache a: %v", err)
	}
	assertEventuallyMiss(t, cacheB, "item")
}

func TestNearCacheOnErrorPanicIsRecovered(t *testing.T) {
	var reports int32
	near := &NearCache[string]{
		cfg: config[string]{
			namespace: "panic",
			originID:  "origin-a",
			local:     cache.NewMemory[string, string](),
			onError: func(context.Context, error) {
				atomic.AddInt32(&reports, 1)
				panic("observer failed")
			},
		},
	}

	near.applyMessage(context.Background(), "{")
	near.applyMessage(context.Background(), "{")

	if atomic.LoadInt32(&reports) != 2 {
		t.Fatalf("panic should not stop later error reporting, got %d reports", reports)
	}
}

func TestNearCacheCloseSurfacesBlockedErrorReporter(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	near := &NearCache[string]{
		cfg: config[string]{
			onError: func(context.Context, error) {
				close(started)
				<-release
			},
		},
		cancel:    func() {},
		errorCh:   make(chan errorReport, 1),
		errorDone: make(chan struct{}),
	}
	runCtx, cancel := context.WithCancel(context.Background())
	near.cancel = cancel
	go near.reportErrors(runCtx)
	near.errorCh <- errorReport{ctx: context.Background(), err: errors.New("boom")}

	<-started
	startedAt := time.Now()
	err := near.Close()
	if err == nil || !strings.Contains(err.Error(), "near cache error reporter did not stop") {
		t.Fatalf("Close error = %v, want blocked reporter error", err)
	}
	if elapsed := time.Since(startedAt); elapsed > receiverShutdownBudget+500*time.Millisecond {
		t.Fatalf("Close waited too long for blocked reporter: %s", elapsed)
	}
	close(release)

	bttesting.Eventually(t, 2*time.Second, func() bool {
		select {
		case <-near.errorDone:
			return true
		default:
			return false
		}
	})
}

func TestNearCacheClearsLocalOnReceiveError(t *testing.T) {
	ctx := context.Background()
	client, _ := redisClients(ctx, t)
	var reports int32

	near, err := NewPubSub[string](ctx, Options[string]{
		Client:    client,
		Namespace: "receive-error",
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
		return "value", nil
	}); err != nil {
		t.Fatalf("prime local cache: %v", err)
	}
	if err := client.Close(); err != nil {
		t.Fatalf("close redis client: %v", err)
	}

	bttesting.Eventually(t, 2*time.Second, func() bool {
		return atomic.LoadInt32(&reports) > 0
	})
	assertEventuallyMiss(t, near, "item")
}

func TestNearCacheCloseIsIdempotentAndBlocksOperations(t *testing.T) {
	ctx := context.Background()
	client, _ := redisClients(ctx, t)
	near, err := NewPubSub[string](ctx, Options[string]{
		Client:    client,
		Namespace: "close",
	})
	if err != nil {
		t.Fatalf("new cache: %v", err)
	}

	if err := near.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	if err := near.Close(); err != nil {
		t.Fatalf("second close: %v", err)
	}
	if _, err := near.Get(ctx, "key"); !errors.Is(err, ErrClosed) {
		t.Fatalf("get after close should fail with ErrClosed, got %v", err)
	}
	if err := near.Set(ctx, "key", "value", 0); !errors.Is(err, ErrClosed) {
		t.Fatalf("set after close should fail with ErrClosed, got %v", err)
	}
	if _, err := near.GetOrLoad(ctx, "key", 0, func(context.Context, string) (string, error) {
		return "value", nil
	}); !errors.Is(err, ErrClosed) {
		t.Fatalf("get or load after close should fail with ErrClosed, got %v", err)
	}
}

func TestNearCacheCloseWaitsForInflightOperation(t *testing.T) {
	local := &blockingLocalCache[string]{value: "value", entered: make(chan struct{}), release: make(chan struct{})}
	near := &NearCache[string]{
		cfg: config[string]{
			local: local,
		},
	}

	getDone := make(chan error, 1)
	go func() {
		_, err := near.Get(context.Background(), "key")
		getDone <- err
	}()
	<-local.entered

	closeDone := make(chan error, 1)
	go func() {
		closeDone <- near.Close()
	}()

	bttesting.Consistently(t, 100*time.Millisecond, func() bool {
		return len(closeDone) == 0
	})
	close(local.release)

	if err := <-getDone; err != nil {
		t.Fatalf("inflight get should finish before close: %v", err)
	}
	if err := <-closeDone; err != nil {
		t.Fatalf("close: %v", err)
	}
	if _, err := near.Get(context.Background(), "key"); !errors.Is(err, ErrClosed) {
		t.Fatalf("post-close get should fail with ErrClosed, got %v", err)
	}
}

func TestNewPubSubPropagatesCanceledContext(t *testing.T) {
	client := redis.NewClient(&redis.Options{Addr: "127.0.0.1:0"})
	t.Cleanup(func() {
		_ = client.Close()
	})
	var sequence int32

	task := func(context.Context) error {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		_, err := NewPubSub[string](ctx, Options[string]{
			Client:    client,
			Namespace: fmt.Sprintf("canceled-%d", atomic.AddInt32(&sequence, 1)),
		})
		if !errors.Is(err, context.Canceled) {
			return fmt.Errorf("constructor should preserve context.Canceled, got %w", err)
		}
		return nil
	}

	tester := concurrencytest.NewAsyncJobTester(concurrencytest.Options{
		Workers:       4,
		RoundsPerTask: 4,
		Timeout:       5 * time.Second,
	})
	report := tester.RunT(t, task)
	if report.Completed != 4 {
		t.Fatalf("async cancellation should complete every round, got %+v", report)
	}
}

func TestNearCacheConcurrentStress(t *testing.T) {
	ctx := context.Background()
	clientA, clientB := redisClients(ctx, t)
	nearA, err := NewPubSub[int64](ctx, Options[int64]{
		Client:    clientA,
		Namespace: "stress",
		OriginID:  "origin-a",
	})
	if err != nil {
		t.Fatalf("new cache a: %v", err)
	}
	t.Cleanup(func() {
		_ = nearA.Close()
	})
	nearB, err := NewPubSub[int64](ctx, Options[int64]{
		Client:    clientB,
		Namespace: "stress",
		OriginID:  "origin-b",
	})
	if err != nil {
		t.Fatalf("new cache b: %v", err)
	}
	t.Cleanup(func() {
		_ = nearB.Close()
	})
	var sequence int64

	task := func(ctx context.Context) error {
		current := atomic.AddInt64(&sequence, 1)
		key := fmt.Sprintf("key-%d", current%8)
		switch current % 6 {
		case 0:
			return nearA.Set(ctx, key, current, time.Second)
		case 1:
			return nearB.Set(ctx, key, current, time.Second)
		case 2:
			return nearA.Delete(ctx, key)
		case 3:
			return nearB.Clear(ctx)
		case 4:
			value, err := nearA.GetOrLoad(ctx, key, time.Second, func(context.Context, string) (int64, error) {
				return current, nil
			})
			if err != nil {
				return err
			}
			if value == 0 {
				return fmt.Errorf("loaded value should not be zero")
			}
			return nil
		default:
			value, err := nearB.GetOrLoad(ctx, key, time.Second, func(context.Context, string) (int64, error) {
				return current, nil
			})
			if err != nil {
				return err
			}
			if value == 0 {
				return fmt.Errorf("loaded value should not be zero")
			}
			return nil
		}
	}

	tester := concurrencytest.NewGoroutineStressTester(concurrencytest.Options{
		Workers:       8,
		RoundsPerTask: 32,
		Timeout:       10 * time.Second,
	})
	report := tester.RunT(t, task)
	if report.Completed != 32 {
		t.Fatalf("stress test should complete every task round, got %+v", report)
	}
}

func redisClients(ctx context.Context, t *testing.T) (*redis.Client, *redis.Client) {
	t.Helper()

	addr := redistestcontainer.Start(ctx, t)
	clientA := redis.NewClient(&redis.Options{Addr: addr})
	clientB := redis.NewClient(&redis.Options{Addr: addr})
	t.Cleanup(func() {
		_ = clientA.Close()
		_ = clientB.Close()
	})
	waitForRedis(t, clientA)
	waitForRedis(t, clientB)
	return clientA, clientB
}

func waitForRedis(t *testing.T, client *redis.Client) {
	t.Helper()

	bttesting.Eventually(t, 2*time.Second, func() bool {
		return client.Ping(context.Background()).Err() == nil
	})
}

func assertEventuallyMiss[V any](t *testing.T, c *NearCache[V], key string) {
	t.Helper()

	bttesting.Eventually(t, 2*time.Second, func() bool {
		_, err := c.Get(context.Background(), key)
		return errors.Is(err, cache.ErrCacheMiss)
	})
}

type blockingLocalCache[V any] struct {
	value   V
	entered chan struct{}
	release chan struct{}
	once    sync.Once
}

func (c *blockingLocalCache[V]) Get(context.Context, string) (V, error) {
	c.once.Do(func() {
		close(c.entered)
	})
	<-c.release
	return c.value, nil
}

func (c *blockingLocalCache[V]) Set(context.Context, string, V, time.Duration) error {
	return nil
}

func (c *blockingLocalCache[V]) Delete(context.Context, string) error {
	return nil
}

func (c *blockingLocalCache[V]) Clear(context.Context) error {
	return nil
}

func (c *blockingLocalCache[V]) GetOrLoad(
	ctx context.Context,
	key string,
	_ time.Duration,
	_ cache.Loader[string, V],
) (V, error) {
	return c.Get(ctx, key)
}
