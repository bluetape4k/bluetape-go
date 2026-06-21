package concurrencytest_test

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/bluetape4k/bluetape-go/concurrency"
	concurrencytest "github.com/bluetape4k/bluetape-go/testing/concurrency"
)

func TestGoroutineStressTesterCollectsCompletionsAndBoundsParallelism(t *testing.T) {
	var running int32
	var total int32

	tester := concurrencytest.NewGoroutineStressTester(concurrencytest.Options{
		Workers:       3,
		RoundsPerTask: 5,
	})

	report, err := tester.Run(context.Background(), func(context.Context) error {
		atomic.AddInt32(&total, 1)
		current := atomic.AddInt32(&running, 1)
		defer atomic.AddInt32(&running, -1)

		if current > 3 {
			t.Errorf("concurrency limit exceeded: %d", current)
		}
		time.Sleep(5 * time.Millisecond)
		return nil
	})
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	if report.Completed != 5 || atomic.LoadInt32(&total) != 5 {
		t.Fatalf("expected 5 completions, got report=%+v total=%d", report, total)
	}
	if report.MaxConcurrent > 3 {
		t.Fatalf("expected max concurrency <= 3, got %d", report.MaxConcurrent)
	}
}

func TestGoroutineStressTesterCapturesErrorsAndPanics(t *testing.T) {
	expected := errors.New("bad task")
	tester := concurrencytest.NewGoroutineStressTester(concurrencytest.Options{Workers: 2})

	report, err := tester.Run(
		context.Background(),
		func(context.Context) error { return expected },
		func(context.Context) error {
			panic("boom")
		},
	)

	if report.Failures != 2 || report.Panics != 1 {
		t.Fatalf("unexpected report: %+v", report)
	}
	if !errors.Is(err, expected) {
		t.Fatalf("expected wrapped task error, got %v", err)
	}
	var panicErr concurrency.PanicError
	if !errors.As(err, &panicErr) {
		t.Fatalf("expected PanicError, got %T %v", err, err)
	}
}

func TestAsyncJobTesterReportsTimeout(t *testing.T) {
	tester := concurrencytest.NewAsyncJobTester(concurrencytest.Options{
		Workers: 1,
		Timeout: 25 * time.Millisecond,
	})

	report, err := tester.Run(context.Background(), func(ctx context.Context) error {
		<-ctx.Done()
		return ctx.Err()
	})

	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected deadline exceeded, got %v", err)
	}
	if report.Failures != 1 {
		t.Fatalf("expected one timeout failure, got %+v", report)
	}
}

func TestAsyncJobTesterPropagatesCallerCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	tester := concurrencytest.NewAsyncJobTester(concurrencytest.Options{Workers: 1})
	report, err := tester.Run(ctx, func(context.Context) error {
		t.Fatal("job should not run after cancellation")
		return nil
	})

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
	if report.Started != 0 {
		t.Fatalf("expected no started jobs, got %+v", report)
	}
}

func TestGoroutineStressTesterRejectsInvalidOptionsAndTasks(t *testing.T) {
	t.Run("missing task", func(t *testing.T) {
		tester := concurrencytest.NewGoroutineStressTester(concurrencytest.Options{})
		if _, err := tester.Run(context.Background()); err == nil {
			t.Fatal("expected missing task error")
		}
	})

	t.Run("nil task", func(t *testing.T) {
		tester := concurrencytest.NewGoroutineStressTester(concurrencytest.Options{})
		if _, err := tester.Run(context.Background(), nil); err == nil {
			t.Fatal("expected nil task error")
		}
	})

	t.Run("negative workers", func(t *testing.T) {
		tester := concurrencytest.NewGoroutineStressTester(concurrencytest.Options{Workers: -1})
		if _, err := tester.Run(context.Background(), func(context.Context) error { return nil }); err == nil {
			t.Fatal("expected negative workers error")
		}
	})

	t.Run("negative rounds", func(t *testing.T) {
		tester := concurrencytest.NewGoroutineStressTester(concurrencytest.Options{RoundsPerTask: -1})
		if _, err := tester.Run(context.Background(), func(context.Context) error { return nil }); err == nil {
			t.Fatal("expected negative rounds error")
		}
	})

	t.Run("negative timeout", func(t *testing.T) {
		tester := concurrencytest.NewGoroutineStressTester(concurrencytest.Options{Timeout: -time.Second})
		if _, err := tester.Run(context.Background(), func(context.Context) error { return nil }); err == nil {
			t.Fatal("expected negative timeout error")
		}
	})
}

func TestGoroutineStressTesterPropagatesCallerCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	tester := concurrencytest.NewGoroutineStressTester(concurrencytest.Options{Workers: 1})
	report, err := tester.Run(ctx, func(context.Context) error {
		t.Fatal("task should not run after caller cancellation")
		return nil
	})

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
	if report.Started != 0 {
		t.Fatalf("expected no started tasks, got %+v", report)
	}
}

func TestRunTReportsSuccess(t *testing.T) {
	tester := concurrencytest.NewGoroutineStressTester(concurrencytest.Options{
		Workers:       2,
		RoundsPerTask: 2,
	})

	report := tester.RunT(t, func(context.Context) error { return nil })
	if report.Completed != 2 {
		t.Fatalf("expected two completions, got %+v", report)
	}
}
