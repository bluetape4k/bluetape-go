package concurrencytest_test

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	concurrencytest "github.com/bluetape4k/bluetape-go/testing/concurrency"
)

func TestExamples(t *testing.T) {
	t.Run("goroutine stress", func(t *testing.T) {
		var count int32
		tester := concurrencytest.NewGoroutineStressTester(concurrencytest.Options{
			Workers:       4,
			RoundsPerTask: 10,
		})

		report := tester.RunT(t, func(context.Context) error {
			atomic.AddInt32(&count, 1)
			return nil
		})
		if report.Completed != 10 {
			t.Fatalf("expected 10 completions, got %+v", report)
		}
	})

	t.Run("async jobs", func(t *testing.T) {
		tester := concurrencytest.NewAsyncJobTester(concurrencytest.Options{
			Workers: 2,
			Timeout: time.Second,
		})

		tester.RunT(t, func(ctx context.Context) error {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
				return nil
			}
		})
	})
}
