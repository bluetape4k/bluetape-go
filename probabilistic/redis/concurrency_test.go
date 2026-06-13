package redisbloom

import (
	"context"
	"errors"
	"runtime"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	concurrencytest "github.com/bluetape4k/bluetape-go/testing/concurrency"
)

func TestGoroutineStressTesterCoversConcurrentRedisBloomCalls(t *testing.T) {
	ctx := context.Background()
	client := newRedisClient(t)
	namespace := testNamespace(t)
	cleanupNamespace(t, client, namespace)
	filter, err := NewStringBloomFilter(ctx, client, namespace, testConfig(t, 50000))
	if err != nil {
		t.Fatalf("NewStringBloomFilter failed: %v", err)
	}

	var sequence atomic.Uint64
	tester := concurrencytest.NewGoroutineStressTester(concurrencytest.Options{
		Workers:       max(32, runtime.GOMAXPROCS(0)*4),
		RoundsPerTask: 100,
		Timeout:       20 * time.Second,
	})
	report := tester.RunT(t,
		func(ctx context.Context) error {
			value := "stress:" + strconv.FormatUint(sequence.Add(1), 10)
			changed, err := filter.Put(ctx, value)
			if err != nil {
				return err
			}
			if !changed {
				return errors.New("first insert did not change bitmap")
			}
			ok, err := filter.MightContain(ctx, value)
			if err != nil {
				return err
			}
			if !ok {
				return errors.New("inserted value produced false negative")
			}
			return nil
		},
		func(ctx context.Context) error {
			_, err := filter.BitCount(ctx)
			return err
		},
		func(ctx context.Context) error {
			_, err := filter.IsEmpty(ctx)
			return err
		},
	)
	if report.MaxConcurrent <= 1 {
		t.Fatalf("MaxConcurrent = %d, want concurrent execution", report.MaxConcurrent)
	}
}

func TestAsyncJobTesterCoversCancellationAndLiveRedisCalls(t *testing.T) {
	ctx := context.Background()
	client := newRedisClient(t)
	namespace := testNamespace(t)
	cleanupNamespace(t, client, namespace)
	filter, err := NewStringBloomFilter(ctx, client, namespace, testConfig(t, 5000))
	if err != nil {
		t.Fatalf("NewStringBloomFilter failed: %v", err)
	}

	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	tester := concurrencytest.NewAsyncJobTester(concurrencytest.Options{
		Workers:       12,
		RoundsPerTask: 40,
		Timeout:       15 * time.Second,
	})
	report, err := tester.Run(ctx,
		func(ctx context.Context) error {
			_, err := filter.Put(ctx, "live:"+strconv.FormatInt(time.Now().UnixNano(), 10))
			return err
		},
		func(ctx context.Context) error {
			_, err := filter.MightContain(ctx, "live")
			return err
		},
		func(context.Context) error {
			_, err := filter.MightContain(cancelled, "cancelled")
			if errors.Is(err, context.Canceled) {
				return nil
			}
			if err == nil {
				return errors.New("cancelled context unexpectedly succeeded")
			}
			return err
		},
	)
	if err != nil {
		t.Fatalf("AsyncJobTester failed after %d/%d completions: %v", report.Completed, report.Started, err)
	}
	if report.MaxConcurrent <= 1 {
		t.Fatalf("MaxConcurrent = %d, want concurrent execution", report.MaxConcurrent)
	}
}
