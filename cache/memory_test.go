package cache

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	concurrencytest "github.com/bluetape4k/bluetape-go/testing/concurrency"
)

type fakeClock struct {
	mu  sync.Mutex
	now time.Time
}

func newFakeClock() *fakeClock {
	return &fakeClock{now: time.Date(2026, 6, 4, 12, 0, 0, 0, time.UTC)}
}

func (c *fakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *fakeClock) Advance(duration time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(duration)
}

func TestMemoryReturnsMissForAbsentKey(t *testing.T) {
	cache := NewMemory[string, string]()

	_, err := cache.Get(context.Background(), "missing")
	if !errors.Is(err, ErrCacheMiss) {
		t.Fatalf("missing key should return ErrCacheMiss, got %v", err)
	}
}

func TestMemoryStoresDeletesAndClearsValues(t *testing.T) {
	ctx := context.Background()
	cache := NewMemory[string, int]()

	if err := cache.Set(ctx, "first", 1, 0); err != nil {
		t.Fatalf("set first: %v", err)
	}
	if err := cache.Set(ctx, "second", 2, 0); err != nil {
		t.Fatalf("set second: %v", err)
	}
	value, err := cache.Get(ctx, "first")
	if err != nil {
		t.Fatalf("get first: %v", err)
	}
	if value != 1 {
		t.Fatalf("first value should be 1, got %d", value)
	}

	if err := cache.Delete(ctx, "first"); err != nil {
		t.Fatalf("delete first: %v", err)
	}
	if _, err := cache.Get(ctx, "first"); !errors.Is(err, ErrCacheMiss) {
		t.Fatalf("deleted key should miss, got %v", err)
	}

	if err := cache.Clear(ctx); err != nil {
		t.Fatalf("clear: %v", err)
	}
	if _, err := cache.Get(ctx, "second"); !errors.Is(err, ErrCacheMiss) {
		t.Fatalf("cleared key should miss, got %v", err)
	}
}

func TestMemoryZeroValueIsUsable(t *testing.T) {
	ctx := context.Background()
	var cache Memory[string, string]

	if _, err := cache.Get(ctx, "missing"); !errors.Is(err, ErrCacheMiss) {
		t.Fatalf("zero-value cache should return miss, got %v", err)
	}
	if err := cache.Set(ctx, "key", "value", 0); err != nil {
		t.Fatalf("zero-value set: %v", err)
	}
	value, err := cache.Get(ctx, "key")
	if err != nil {
		t.Fatalf("zero-value get: %v", err)
	}
	if value != "value" {
		t.Fatalf("zero-value cache stored %q", value)
	}
}

func TestMemoryTTLExpiresAndZeroTTLDoesNotExpire(t *testing.T) {
	ctx := context.Background()
	clock := newFakeClock()
	cache := newMemoryWithClock[string, string](clock.Now)

	if err := cache.Set(ctx, "short", "value", 5*time.Second); err != nil {
		t.Fatalf("set short ttl: %v", err)
	}
	if value, err := cache.Get(ctx, "short"); err != nil || value != "value" {
		t.Fatalf("short ttl should hit before expiry, value=%q err=%v", value, err)
	}
	clock.Advance(6 * time.Second)
	if _, err := cache.Get(ctx, "short"); !errors.Is(err, ErrCacheMiss) {
		t.Fatalf("expired key should miss, got %v", err)
	}

	if err := cache.Set(ctx, "forever", "value", 0); err != nil {
		t.Fatalf("set zero ttl: %v", err)
	}
	clock.Advance(24 * time.Hour)
	if value, err := cache.Get(ctx, "forever"); err != nil || value != "value" {
		t.Fatalf("zero ttl should not expire, value=%q err=%v", value, err)
	}
}

func TestMemoryRejectsNegativeTTL(t *testing.T) {
	ctx := context.Background()
	cache := NewMemory[string, string]()

	if err := cache.Set(ctx, "key", "value", -time.Second); err == nil {
		t.Fatal("set should reject negative ttl")
	}
	if _, err := cache.Get(context.Background(), "key"); !errors.Is(err, ErrCacheMiss) {
		t.Fatalf("negative ttl write should not store value, got %v", err)
	}
	if _, err := cache.GetOrLoad(ctx, "key", -time.Second, func(context.Context, string) (string, error) {
		return "value", nil
	}); err == nil {
		t.Fatal("get or load should reject negative ttl")
	}
}

func TestMemoryGetOrLoadCachesSuccessfulLoader(t *testing.T) {
	ctx := context.Background()
	cache := NewMemory[string, string]()
	var loads int32

	first, err := cache.GetOrLoad(ctx, "key", time.Minute, func(context.Context, string) (string, error) {
		atomic.AddInt32(&loads, 1)
		return "value", nil
	})
	if err != nil {
		t.Fatalf("first get or load: %v", err)
	}
	second, err := cache.GetOrLoad(ctx, "key", time.Minute, func(context.Context, string) (string, error) {
		atomic.AddInt32(&loads, 1)
		return "other", nil
	})
	if err != nil {
		t.Fatalf("second get or load: %v", err)
	}
	if first != "value" || second != "value" {
		t.Fatalf("cached value mismatch: first=%q second=%q", first, second)
	}
	if atomic.LoadInt32(&loads) != 1 {
		t.Fatalf("loader should run once, got %d", loads)
	}
}

func TestMemoryGetOrLoadDoesNotCacheLoaderError(t *testing.T) {
	ctx := context.Background()
	cache := NewMemory[string, string]()
	errLoader := errors.New("loader failed")

	if _, err := cache.GetOrLoad(ctx, "key", time.Minute, func(context.Context, string) (string, error) {
		return "", errLoader
	}); !errors.Is(err, errLoader) {
		t.Fatalf("loader error should be returned, got %v", err)
	}

	value, err := cache.GetOrLoad(ctx, "key", time.Minute, func(context.Context, string) (string, error) {
		return "recovered", nil
	})
	if err != nil {
		t.Fatalf("second loader should recover: %v", err)
	}
	if value != "recovered" {
		t.Fatalf("loader error should not be cached, got %q", value)
	}
}

func TestMemoryGetOrLoadRejectsNilLoader(t *testing.T) {
	cache := NewMemory[string, string]()

	if _, err := cache.GetOrLoad(context.Background(), "key", time.Minute, nil); err == nil {
		t.Fatal("nil loader should fail")
	}
}

func TestMemoryPropagatesCanceledContextBeforeMutation(t *testing.T) {
	cache := NewMemory[string, string]()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if err := cache.Set(ctx, "key", "value", 0); !errors.Is(err, context.Canceled) {
		t.Fatalf("set should preserve canceled context, got %v", err)
	}
	if _, err := cache.Get(context.Background(), "key"); !errors.Is(err, ErrCacheMiss) {
		t.Fatalf("canceled set should not mutate cache, got %v", err)
	}
	if _, err := cache.GetOrLoad(ctx, "key", time.Minute, func(context.Context, string) (string, error) {
		return "value", nil
	}); !errors.Is(err, context.Canceled) {
		t.Fatalf("get or load should preserve canceled context, got %v", err)
	}
}

func TestMemorySameKeyStressRunsOneLoader(t *testing.T) {
	cache := NewMemory[string, string]()
	var loads int32

	task := func(ctx context.Context) error {
		value, err := cache.GetOrLoad(ctx, "shared", time.Minute, func(context.Context, string) (string, error) {
			atomic.AddInt32(&loads, 1)
			time.Sleep(20 * time.Millisecond)
			return "value", nil
		})
		if err != nil {
			return err
		}
		if value != "value" {
			return fmt.Errorf("unexpected value: %q", value)
		}
		return nil
	}

	tester := concurrencytest.NewGoroutineStressTester(concurrencytest.Options{
		Workers:       16,
		RoundsPerTask: 16,
		Timeout:       5 * time.Second,
	})
	report := tester.RunT(t, task)
	if report.Completed != 16 {
		t.Fatalf("stress test should complete every task round, got %+v", report)
	}
	if atomic.LoadInt32(&loads) != 1 {
		t.Fatalf("same-key loader should run once, got %d", loads)
	}
}

func TestMemorySameKeyCanceledOwnerDoesNotCancelLiveWaiter(t *testing.T) {
	cache := NewMemory[string, string]()
	ownerLoaderStarted := make(chan struct{})
	releaseOwnerLoader := make(chan struct{})

	ownerCtx, ownerCancel := context.WithCancel(context.Background())
	defer ownerCancel()
	ownerErr := make(chan error, 1)
	go func() {
		_, err := cache.GetOrLoad(ownerCtx, "shared", time.Minute, func(ctx context.Context, _ string) (string, error) {
			close(ownerLoaderStarted)
			<-releaseOwnerLoader
			return "", ctx.Err()
		})
		ownerErr <- err
	}()

	select {
	case <-ownerLoaderStarted:
	case <-time.After(time.Second):
		t.Fatal("owner loader did not start")
	}

	waiterDone := make(chan struct {
		value string
		err   error
	}, 1)
	waiterLoaderStarted := make(chan struct{}, 1)
	go func() {
		value, err := cache.GetOrLoad(context.Background(), "shared", time.Minute, func(context.Context, string) (string, error) {
			waiterLoaderStarted <- struct{}{}
			return "waiter-value", nil
		})
		waiterDone <- struct {
			value string
			err   error
		}{value: value, err: err}
	}()

	select {
	case result := <-waiterDone:
		t.Fatalf("waiter returned before owner loader was released: value=%q err=%v", result.value, result.err)
	case <-waiterLoaderStarted:
		t.Fatal("waiter started an independent loader while owner loader was in flight")
	case <-time.After(100 * time.Millisecond):
	}

	ownerCancel()
	close(releaseOwnerLoader)

	select {
	case err := <-ownerErr:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("owner should observe its cancellation, got %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("owner did not finish")
	}
	select {
	case result := <-waiterDone:
		if result.err != nil {
			t.Fatalf("live waiter should retry after owner cancellation, got %v", result.err)
		}
		if result.value != "waiter-value" {
			t.Fatalf("waiter value = %q, want waiter-value", result.value)
		}
	case <-time.After(time.Second):
		t.Fatal("waiter did not finish")
	}
}

type collisionKey struct {
	id int
}

func (k collisionKey) String() string {
	return "same"
}

func TestMemoryDifferentKeysDoNotShareFlightResult(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	cache := NewMemory[collisionKey, string]()

	ready := make(chan struct{}, 2)
	release := make(chan struct{})
	load := func(ctx context.Context, key collisionKey) (string, error) {
		ready <- struct{}{}
		select {
		case <-release:
			return fmt.Sprintf("value-%d", key.id), nil
		case <-ctx.Done():
			return "", ctx.Err()
		}
	}

	results := make(chan string, 2)
	errs := make(chan error, 2)
	for _, key := range []collisionKey{{id: 1}, {id: 2}} {
		key := key
		go func() {
			value, err := cache.GetOrLoad(ctx, key, time.Minute, load)
			if err != nil {
				errs <- err
				return
			}
			results <- value
		}()
	}

	for i := 0; i < 2; i++ {
		select {
		case <-ready:
		case <-ctx.Done():
			t.Fatalf("distinct keys should start independent loaders: %v", ctx.Err())
		}
	}
	close(release)

	seen := map[string]bool{}
	for i := 0; i < 2; i++ {
		select {
		case err := <-errs:
			t.Fatalf("get or load failed: %v", err)
		case value := <-results:
			seen[value] = true
		case <-ctx.Done():
			t.Fatalf("timed out waiting for results: %v", ctx.Err())
		}
	}
	if !seen["value-1"] || !seen["value-2"] {
		t.Fatalf("distinct keys should keep distinct values, got %v", seen)
	}
}

func TestMemoryAsyncJobTesterPropagatesLoaderCancellation(t *testing.T) {
	cache := NewMemory[int, string]()
	var sequence int32

	task := func(ctx context.Context) error {
		key := int(atomic.AddInt32(&sequence, 1))
		loadCtx, cancel := context.WithCancel(ctx)
		value, err := cache.GetOrLoad(loadCtx, key, time.Minute, func(ctx context.Context, _ int) (string, error) {
			cancel()
			<-ctx.Done()
			return "", ctx.Err()
		})
		if !errors.Is(err, context.Canceled) {
			return fmt.Errorf("loader cancellation should propagate, value=%q err=%w", value, err)
		}
		if _, err := cache.Get(context.Background(), key); !errors.Is(err, ErrCacheMiss) {
			return fmt.Errorf("canceled loader should not cache key %d: %w", key, err)
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
		t.Fatalf("async tester should complete every task round, got %+v", report)
	}
}
