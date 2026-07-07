package concurrency_test

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/bluetape4k/bluetape-go/concurrency"
	concurrencytest "github.com/bluetape4k/bluetape-go/testing/concurrency"
)

func TestGoCapturesPanicAsError(t *testing.T) {
	err := <-concurrency.Go(context.Background(), func(context.Context) error {
		panic("boom")
	})

	var panicErr concurrency.PanicError
	if !errors.As(err, &panicErr) {
		t.Fatalf("expected PanicError, got %T %v", err, err)
	}
	if panicErr.Value != "boom" {
		t.Fatalf("unexpected panic value: %v", panicErr.Value)
	}
}

func TestGroupPropagatesErrorAndCancelsContext(t *testing.T) {
	expected := errors.New("stop")
	group := concurrency.NewGroup(context.Background())

	ready := make(chan struct{})
	cancelled := make(chan struct{})
	group.Go(func(ctx context.Context) error {
		close(ready)
		<-ctx.Done()
		close(cancelled)
		return nil
	})

	<-ready
	group.Go(func(context.Context) error {
		return expected
	})

	if err := group.Wait(); !errors.Is(err, expected) {
		t.Fatalf("expected %v, got %v", expected, err)
	}

	select {
	case <-cancelled:
	case <-time.After(time.Second):
		t.Fatal("group context was not cancelled")
	}
}

func TestForEachHonorsConcurrencyLimit(t *testing.T) {
	var running int32
	var maxRunning int32

	err := concurrency.ForEach(context.Background(), []int{1, 2, 3, 4, 5, 6}, 2, func(context.Context, int) error {
		current := atomic.AddInt32(&running, 1)
		defer atomic.AddInt32(&running, -1)

		for {
			observed := atomic.LoadInt32(&maxRunning)
			if current <= observed || atomic.CompareAndSwapInt32(&maxRunning, observed, current) {
				break
			}
		}

		time.Sleep(20 * time.Millisecond)
		return nil
	})
	if err != nil {
		t.Fatalf("ForEach failed: %v", err)
	}
	if maxRunning > 2 {
		t.Fatalf("expected at most 2 concurrent tasks, saw %d", maxRunning)
	}
}

func TestMapReturnsResultsInInputOrder(t *testing.T) {
	got, err := concurrency.Map(context.Background(), []int{1, 2, 3}, 2, func(_ context.Context, value int) (int, error) {
		return value * value, nil
	})
	if err != nil {
		t.Fatalf("Map failed: %v", err)
	}

	want := []int{1, 4, 9}
	for index := range want {
		if got[index] != want[index] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
}

func TestMapPropagatesCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := concurrency.Map(ctx, []int{1}, 1, func(context.Context, int) (int, error) {
		t.Fatal("mapper should not run for a cancelled context")
		return 0, nil
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
}

func TestMapCancellationUsesAsyncJobTester(t *testing.T) {
	tester := concurrencytest.NewAsyncJobTester(concurrencytest.Options{
		Workers:       2,
		RoundsPerTask: 32,
		Timeout:       time.Second,
	})

	tester.RunT(t, func(context.Context) error {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		_, err := concurrency.Map(ctx, []int{1}, 1, func(context.Context, int) (int, error) {
			return 0, errors.New("mapper should not run for a cancelled context")
		})
		if !errors.Is(err, context.Canceled) {
			return fmt.Errorf("Map error = %w, want context.Canceled", err)
		}
		return nil
	})
}

func TestWorkerPoolProcessesJobsAndStopsOnError(t *testing.T) {
	expected := errors.New("bad job")
	var processed int32

	pool, err := concurrency.NewWorkerPool[int](2, func(_ context.Context, value int) error {
		if value == 3 {
			return expected
		}
		atomic.AddInt32(&processed, 1)
		return nil
	})
	if err != nil {
		t.Fatalf("NewWorkerPool failed: %v", err)
	}

	jobs := make(chan int)
	go func() {
		defer close(jobs)
		for _, value := range []int{1, 2, 3} {
			jobs <- value
		}
	}()

	if err := pool.Run(context.Background(), jobs); !errors.Is(err, expected) {
		t.Fatalf("expected %v, got %v", expected, err)
	}
	if atomic.LoadInt32(&processed) == 0 {
		t.Fatal("expected at least one processed job before error")
	}
}

func TestGroupCancellationUsesAsyncJobTester(t *testing.T) {
	expected := errors.New("stop")
	tester := concurrencytest.NewAsyncJobTester(concurrencytest.Options{
		Workers:       2,
		RoundsPerTask: 32,
		Timeout:       2 * time.Second,
	})

	tester.RunT(t, func(context.Context) error {
		group := concurrency.NewGroup(context.Background())

		ready := make(chan struct{})
		cancelled := make(chan struct{})
		group.Go(func(ctx context.Context) error {
			close(ready)
			<-ctx.Done()
			close(cancelled)
			return nil
		})

		<-ready
		group.Go(func(context.Context) error {
			return expected
		})

		if err := group.Wait(); !errors.Is(err, expected) {
			return fmt.Errorf("Group.Wait error = %w, want %q", err, expected.Error())
		}

		select {
		case <-cancelled:
			return nil
		case <-time.After(100 * time.Millisecond):
			return errors.New("group context was not cancelled")
		}
	})
}

func TestRoundRobinCyclesAndValidatesState(t *testing.T) {
	if _, err := concurrency.NewRoundRobin(0); err == nil {
		t.Fatal("expected invalid maximum to fail")
	}

	roundRobin, err := concurrency.NewRoundRobin(3)
	if err != nil {
		t.Fatalf("NewRoundRobin failed: %v", err)
	}
	if got := roundRobin.Get(); got != 0 {
		t.Fatalf("initial Get() = %d, want 0", got)
	}

	for _, want := range []int{1, 2, 0, 1} {
		if got := roundRobin.Next(); got != want {
			t.Fatalf("Next() = %d, want %d", got, want)
		}
	}

	if err := roundRobin.Set(2); err != nil {
		t.Fatalf("Set failed: %v", err)
	}
	if got := roundRobin.Get(); got != 2 {
		t.Fatalf("Get() after Set = %d, want 2", got)
	}
	if got := roundRobin.Next(); got != 0 {
		t.Fatalf("Next() after Set = %d, want 0", got)
	}

	for _, value := range []int{-1, 3} {
		if err := roundRobin.Set(value); err == nil {
			t.Fatalf("expected Set(%d) to fail", value)
		}
	}
}

func TestRoundRobinConcurrentNextStaysInRange(t *testing.T) {
	const (
		maximum = 4
		runs    = 8000
	)

	roundRobin, err := concurrency.NewRoundRobin(maximum)
	if err != nil {
		t.Fatalf("NewRoundRobin failed: %v", err)
	}

	var counts [maximum]int64
	tester := concurrencytest.NewGoroutineStressTester(concurrencytest.Options{
		Workers:       16,
		RoundsPerTask: runs,
		Timeout:       2 * time.Second,
	})
	report := tester.RunT(t, func(context.Context) error {
		value := roundRobin.Next()
		if value < 0 || value >= maximum {
			return errors.New("round robin returned value out of range")
		}
		atomic.AddInt64(&counts[value], 1)
		return nil
	})
	if report.Completed != runs {
		t.Fatalf("completed = %d, want %d", report.Completed, runs)
	}
	if report.MaxConcurrent > 16 {
		t.Fatalf("max concurrent = %d, want <= 16", report.MaxConcurrent)
	}

	var total int64
	for value, count := range counts {
		total += count
		if count != runs/maximum {
			t.Fatalf("value %d returned %d times, want %d; counts=%v", value, count, runs/maximum, counts)
		}
	}
	if total != runs {
		t.Fatalf("total returned values = %d, want %d; counts=%v", total, runs, counts)
	}
}

func TestParallelHelpersUseGoroutineStressTester(t *testing.T) {
	tester := concurrencytest.NewGoroutineStressTester(concurrencytest.Options{
		Workers:       4,
		RoundsPerTask: 64,
		Timeout:       2 * time.Second,
	})

	tester.RunT(t, func(context.Context) error {
		var running int32
		var maxRunning int32
		if err := concurrency.ForEach(context.Background(), []int{1, 2, 3, 4, 5, 6}, 2, func(context.Context, int) error {
			current := atomic.AddInt32(&running, 1)
			defer atomic.AddInt32(&running, -1)
			for {
				observed := atomic.LoadInt32(&maxRunning)
				if current <= observed || atomic.CompareAndSwapInt32(&maxRunning, observed, current) {
					break
				}
			}
			time.Sleep(time.Millisecond)
			return nil
		}); err != nil {
			return err
		}
		if maxRunning > 2 {
			return errors.New("ForEach exceeded concurrency limit")
		}

		values, err := concurrency.Map(context.Background(), []int{1, 2, 3, 4}, 2, func(_ context.Context, value int) (int, error) {
			return value * value, nil
		})
		if err != nil {
			return err
		}
		for index, want := range []int{1, 4, 9, 16} {
			if values[index] != want {
				return errors.New("Map did not preserve input order")
			}
		}
		return nil
	})
}
