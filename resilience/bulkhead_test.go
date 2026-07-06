package resilience_test

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/bluetape4k/bluetape-go/resilience"
	concurrencytest "github.com/bluetape4k/bluetape-go/testing/concurrency"
)

func TestBulkheadRejectsBeyondMaxConcurrent(t *testing.T) {
	var events []resilience.Event
	bulkhead, err := resilience.NewBulkhead[int](resilience.BulkheadOptions{
		Name:          "workers",
		MaxConcurrent: 1,
		OnEvent: func(_ context.Context, event resilience.Event) {
			events = append(events, event)
		},
	})
	if err != nil {
		t.Fatalf("NewBulkhead failed: %v", err)
	}

	entered := make(chan struct{})
	release := make(chan struct{})
	done := make(chan error, 1)
	go func() {
		_, err := resilience.Run(context.Background(), func(context.Context) (int, error) {
			close(entered)
			<-release
			return 1, nil
		}, bulkhead)
		done <- err
	}()

	<-entered
	_, err = resilience.Run(context.Background(), func(context.Context) (int, error) {
		t.Fatal("rejected operation must not execute")
		return 0, nil
	}, bulkhead)
	if !errors.Is(err, resilience.ErrBulkheadRejected) {
		t.Fatalf("expected ErrBulkheadRejected, got %v", err)
	}

	close(release)
	if err := <-done; err != nil {
		t.Fatalf("first operation failed: %v", err)
	}

	if len(events) < 2 || events[0].Kind != resilience.EventBulkheadAccepted || events[1].Kind != resilience.EventBulkheadRejected {
		t.Fatalf("unexpected events: %#v", events)
	}
}

func TestBulkheadWaitObeysContextCancellation(t *testing.T) {
	bulkhead, err := resilience.NewBulkhead[int](resilience.BulkheadOptions{
		MaxConcurrent: 1,
		Wait:          true,
	})
	if err != nil {
		t.Fatalf("NewBulkhead failed: %v", err)
	}

	entered := make(chan struct{})
	release := make(chan struct{})
	done := make(chan error, 1)
	go func() {
		_, err := resilience.Run(context.Background(), func(context.Context) (int, error) {
			close(entered)
			<-release
			return 1, nil
		}, bulkhead)
		done <- err
	}()

	<-entered
	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond)
	defer cancel()

	_, err = resilience.Run(ctx, func(context.Context) (int, error) {
		t.Fatal("canceled waiter must not execute")
		return 0, nil
	}, bulkhead)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected context deadline, got %v", err)
	}
	if errors.Is(err, resilience.ErrBulkheadRejected) {
		t.Fatalf("context cancellation must not be reported as bulkhead rejection: %v", err)
	}

	close(release)
	if err := <-done; err != nil {
		t.Fatalf("first operation failed: %v", err)
	}
}

func TestBulkheadReleasesPermitAfterFailure(t *testing.T) {
	bulkhead, err := resilience.NewBulkhead[int](resilience.BulkheadOptions{
		MaxConcurrent: 1,
	})
	if err != nil {
		t.Fatalf("NewBulkhead failed: %v", err)
	}

	operationErr := errors.New("operation failed")
	_, err = resilience.Run(context.Background(), func(context.Context) (int, error) {
		return 0, operationErr
	}, bulkhead)
	if !errors.Is(err, operationErr) {
		t.Fatalf("expected operation error, got %v", err)
	}
	if bulkhead.InFlight() != 0 {
		t.Fatalf("in flight = %d, want 0", bulkhead.InFlight())
	}

	got, err := resilience.Run(context.Background(), func(context.Context) (int, error) {
		return 42, nil
	}, bulkhead)
	if err != nil {
		t.Fatalf("second operation failed: %v", err)
	}
	if got != 42 {
		t.Fatalf("got %d, want 42", got)
	}
}

func TestBulkheadRejectStressNeverExceedsMaxConcurrent(t *testing.T) {
	const (
		maxConcurrent = int32(8)
		workers       = int32(128)
	)

	bulkhead, err := resilience.NewBulkhead[int](resilience.BulkheadOptions{
		MaxConcurrent: int(maxConcurrent),
	})
	if err != nil {
		t.Fatalf("NewBulkhead failed: %v", err)
	}

	start := make(chan struct{})
	release := make(chan struct{})
	errs := make(chan error, workers)

	var entered atomic.Int32
	var rejected atomic.Int32
	var current atomic.Int32
	var maxObserved atomic.Int32
	var wg sync.WaitGroup

	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start

			_, err := resilience.Run(context.Background(), func(context.Context) (int, error) {
				inFlight := current.Add(1)
				recordAtomicMax(&maxObserved, inFlight)
				entered.Add(1)
				<-release
				current.Add(-1)
				return 1, nil
			}, bulkhead)

			switch {
			case err == nil:
			case errors.Is(err, resilience.ErrBulkheadRejected):
				rejected.Add(1)
			default:
				errs <- err
			}
		}()
	}

	close(start)
	waitForAtomicSumAtLeast(t, &entered, &rejected, workers)

	if got := maxObserved.Load(); got > maxConcurrent {
		t.Fatalf("max concurrent = %d, want <= %d", got, maxConcurrent)
	}
	if got := entered.Load(); got != maxConcurrent {
		t.Fatalf("entered operations = %d, want %d", got, maxConcurrent)
	}
	if got := rejected.Load(); got != workers-maxConcurrent {
		t.Fatalf("rejected operations = %d, want %d", got, workers-maxConcurrent)
	}

	close(release)
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatalf("unexpected worker error: %v", err)
	}

	if got := current.Load(); got != 0 {
		t.Fatalf("current operations = %d, want 0", got)
	}
	if got := bulkhead.InFlight(); got != 0 {
		t.Fatalf("in flight = %d, want 0", got)
	}
}

func TestBulkheadWaitStressCyclesPermitsWithoutLeaks(t *testing.T) {
	const (
		maxConcurrent = int32(8)
		workers       = int32(96)
	)

	bulkhead, err := resilience.NewBulkhead[int](resilience.BulkheadOptions{
		MaxConcurrent: int(maxConcurrent),
		Wait:          true,
	})
	if err != nil {
		t.Fatalf("NewBulkhead failed: %v", err)
	}

	start := make(chan struct{})
	release := make(chan struct{})
	errs := make(chan error, workers)

	var entered atomic.Int32
	var current atomic.Int32
	var maxObserved atomic.Int32
	var wg sync.WaitGroup

	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start

			_, err := resilience.Run(context.Background(), func(context.Context) (int, error) {
				inFlight := current.Add(1)
				recordAtomicMax(&maxObserved, inFlight)
				entered.Add(1)
				<-release
				current.Add(-1)
				return 1, nil
			}, bulkhead)
			if err != nil {
				errs <- err
			}
		}()
	}

	close(start)
	waitForAtomicAtLeast(t, &entered, maxConcurrent)

	if got := maxObserved.Load(); got > maxConcurrent {
		t.Fatalf("max concurrent before release = %d, want <= %d", got, maxConcurrent)
	}

	close(release)
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatalf("unexpected worker error: %v", err)
	}

	if got := entered.Load(); got != workers {
		t.Fatalf("entered operations = %d, want %d", got, workers)
	}
	if got := maxObserved.Load(); got > maxConcurrent {
		t.Fatalf("max concurrent = %d, want <= %d", got, maxConcurrent)
	}
	if got := current.Load(); got != 0 {
		t.Fatalf("current operations = %d, want 0", got)
	}
	if got := bulkhead.InFlight(); got != 0 {
		t.Fatalf("in flight = %d, want 0", got)
	}
}

func TestBulkheadWaitStressUsesGoroutineStressTester(t *testing.T) {
	const maxConcurrent = int32(8)

	bulkhead, err := resilience.NewBulkhead[int](resilience.BulkheadOptions{
		MaxConcurrent: int(maxConcurrent),
		Wait:          true,
	})
	if err != nil {
		t.Fatalf("NewBulkhead failed: %v", err)
	}

	var current atomic.Int32
	var maxObserved atomic.Int32
	tester := concurrencytest.NewGoroutineStressTester(concurrencytest.Options{
		Workers:       32,
		RoundsPerTask: 256,
		Timeout:       3 * time.Second,
	})

	report := tester.RunT(t, func(context.Context) error {
		_, err := resilience.Run(context.Background(), func(context.Context) (int, error) {
			inFlight := current.Add(1)
			recordAtomicMax(&maxObserved, inFlight)
			time.Sleep(time.Millisecond)
			current.Add(-1)
			return 1, nil
		}, bulkhead)
		return err
	})
	if report.Completed != 256 {
		t.Fatalf("completed = %d, want 256", report.Completed)
	}
	if got := maxObserved.Load(); got > maxConcurrent {
		t.Fatalf("max concurrent = %d, want <= %d", got, maxConcurrent)
	}
	if got := current.Load(); got != 0 {
		t.Fatalf("current operations = %d, want 0", got)
	}
	if got := bulkhead.InFlight(); got != 0 {
		t.Fatalf("in flight = %d, want 0", got)
	}
}

func TestCircuitBreakerAndBulkheadCompose(t *testing.T) {
	breaker, err := resilience.NewCircuitBreaker[int](resilience.CircuitBreakerOptions{
		FailureThreshold: 1,
		OpenTimeout:      time.Second,
	})
	if err != nil {
		t.Fatalf("NewCircuitBreaker failed: %v", err)
	}
	bulkhead, err := resilience.NewBulkhead[int](resilience.BulkheadOptions{
		MaxConcurrent: 1,
	})
	if err != nil {
		t.Fatalf("NewBulkhead failed: %v", err)
	}

	got, err := resilience.Run(context.Background(), func(context.Context) (int, error) {
		return 7, nil
	}, breaker, bulkhead)
	if err != nil {
		t.Fatalf("composed run failed: %v", err)
	}
	if got != 7 {
		t.Fatalf("got %d, want 7", got)
	}
}

func TestBulkheadValidatesOptions(t *testing.T) {
	if _, err := resilience.NewBulkhead[int](resilience.BulkheadOptions{}); err == nil {
		t.Fatal("expected max concurrent validation error")
	}
}
