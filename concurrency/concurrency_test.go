package concurrency_test

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/bluetape4k/bluetape-go/concurrency"
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
		workers = 16
		rounds  = 500
	)

	roundRobin, err := concurrency.NewRoundRobin(maximum)
	if err != nil {
		t.Fatalf("NewRoundRobin failed: %v", err)
	}

	var counts [maximum]int64
	var wg sync.WaitGroup
	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range rounds {
				value := roundRobin.Next()
				if value < 0 || value >= maximum {
					t.Errorf("Next() = %d, want value in [0,%d)", value, maximum)
					return
				}
				atomic.AddInt64(&counts[value], 1)
			}
		}()
	}
	wg.Wait()

	var total int64
	for value, count := range counts {
		total += count
		if count != workers*rounds/maximum {
			t.Fatalf("value %d returned %d times, want %d; counts=%v", value, count, workers*rounds/maximum, counts)
		}
	}
	if total != workers*rounds {
		t.Fatalf("total returned values = %d, want %d; counts=%v", total, workers*rounds, counts)
	}
}
