package ratelimit

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	concurrencytest "github.com/bluetape4k/bluetape-go/testing/concurrency"
)

type manualClock struct {
	value atomic.Int64
}

func newManualClock(start time.Time) *manualClock {
	clock := &manualClock{}
	clock.value.Store(start.UnixNano())
	return clock
}

func (c *manualClock) Now() time.Time {
	return time.Unix(0, c.value.Load())
}

func (c *manualClock) Advance(duration time.Duration) {
	c.value.Add(int64(duration))
}

func TestTokenBucketRejectsInvalidOptions(t *testing.T) {
	tests := []struct {
		name    string
		options Options
	}{
		{name: "zero rate", options: Options{Burst: 1}},
		{name: "zero burst", options: Options{RatePerSecond: 1}},
		{name: "negative idle ttl", options: Options{RatePerSecond: 1, Burst: 1, IdleTTL: -time.Second}},
		{name: "too short idle ttl", options: Options{RatePerSecond: 1, Burst: 10, IdleTTL: time.Second}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := New(tt.options); err == nil {
				t.Fatalf("expected invalid options error")
			}
		})
	}
}

func TestTokenBucketDefaultsIdleTTL(t *testing.T) {
	limiter, err := New(Options{RatePerSecond: 10, Burst: 5})
	if err != nil {
		t.Fatalf("new limiter: %v", err)
	}
	if limiter.opts.idleTTL != minDefaultIdleTTL {
		t.Fatalf("idle ttl = %v, want %v", limiter.opts.idleTTL, minDefaultIdleTTL)
	}
}

func TestTokenBucketAllowsBurstAndReportsRejection(t *testing.T) {
	clock := newManualClock(time.Unix(0, 0))
	limiter, err := newWithClock(Options{RatePerSecond: 1, Burst: 2}, clock.Now)
	if err != nil {
		t.Fatalf("new limiter: %v", err)
	}

	ctx := context.Background()
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
	if result.Allowed {
		t.Fatalf("third request should be rejected: %+v", result)
	}
	if result.Remaining != 0 || result.RetryAfter != time.Second {
		t.Fatalf("unexpected rejection result: %+v", result)
	}
}

func TestTokenBucketRefillsWithClock(t *testing.T) {
	clock := newManualClock(time.Unix(0, 0))
	limiter, err := newWithClock(Options{RatePerSecond: 2, Burst: 2}, clock.Now)
	if err != nil {
		t.Fatalf("new limiter: %v", err)
	}

	ctx := context.Background()
	if _, err := limiter.Allow(ctx, "user-1", 2); err != nil {
		t.Fatalf("consume burst: %v", err)
	}
	clock.Advance(500 * time.Millisecond)

	result, err := limiter.Allow(ctx, "user-1", 1)
	if err != nil {
		t.Fatalf("allow after refill: %v", err)
	}
	if !result.Allowed || result.Remaining != 0 {
		t.Fatalf("unexpected refill result: %+v", result)
	}
}

func TestTokenBucketPrunesIdleKeys(t *testing.T) {
	clock := newManualClock(time.Unix(0, 0))
	limiter, err := newWithClock(Options{
		RatePerSecond: 10,
		Burst:         10,
		IdleTTL:       time.Second,
	}, clock.Now)
	if err != nil {
		t.Fatalf("new limiter: %v", err)
	}

	if _, err := limiter.Allow(context.Background(), "idle", 1); err != nil {
		t.Fatalf("allow idle key: %v", err)
	}
	clock.Advance(time.Second)
	if _, err := limiter.Allow(context.Background(), "active", 1); err != nil {
		t.Fatalf("allow active key: %v", err)
	}

	limiter.mu.Lock()
	defer limiter.mu.Unlock()
	if _, ok := limiter.buckets["idle"]; ok {
		t.Fatalf("idle key was not pruned")
	}
	if _, ok := limiter.buckets["active"]; !ok {
		t.Fatalf("active key missing after prune")
	}
}

func TestTokenBucketReturnsContextCancellation(t *testing.T) {
	limiter, err := New(Options{RatePerSecond: 1, Burst: 1})
	if err != nil {
		t.Fatalf("new limiter: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if _, err := limiter.Allow(ctx, "user-1", 1); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
}

func TestTokenBucketRejectsOverBurstRequest(t *testing.T) {
	limiter, err := New(Options{RatePerSecond: 1, Burst: 1})
	if err != nil {
		t.Fatalf("new limiter: %v", err)
	}
	if _, err := limiter.Allow(context.Background(), "user-1", 2); err == nil {
		t.Fatalf("expected over-burst request error")
	}
}

func TestTokenBucketStressDoesNotOverAdmitBurst(t *testing.T) {
	clock := newManualClock(time.Unix(0, 0))
	limiter, err := newWithClock(Options{RatePerSecond: 10, Burst: 10}, clock.Now)
	if err != nil {
		t.Fatalf("new limiter: %v", err)
	}

	var allowed atomic.Int64
	tester := concurrencytest.NewGoroutineStressTester(concurrencytest.Options{
		Workers:       16,
		RoundsPerTask: 4,
		Timeout:       5 * time.Second,
	})
	task := func(ctx context.Context) error {
		result, err := limiter.Allow(ctx, "shared", 1)
		if err != nil {
			return err
		}
		if result.Allowed {
			allowed.Add(1)
		}
		return nil
	}
	report, err := tester.Run(context.Background(), task, task, task, task, task)
	if err != nil {
		t.Fatalf("stress failed: report=%+v err=%v", report, err)
	}
	if got := allowed.Load(); got != 10 {
		t.Fatalf("allowed = %d, want 10", got)
	}
}

func TestTokenBucketAsyncJobTesterCoversCancellation(t *testing.T) {
	limiter, err := New(Options{RatePerSecond: 1, Burst: 1})
	if err != nil {
		t.Fatalf("new limiter: %v", err)
	}
	tester := concurrencytest.NewAsyncJobTester(concurrencytest.Options{
		Workers: 1,
		Timeout: time.Second,
	})

	report, err := tester.Run(context.Background(), func(ctx context.Context) error {
		canceled, cancel := context.WithCancel(ctx)
		cancel()
		if _, err := limiter.Allow(canceled, "user-1", 1); !errors.Is(err, context.Canceled) {
			return fmt.Errorf("expected context.Canceled, got %w", err)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("async cancellation failed: report=%+v err=%v", report, err)
	}
}
