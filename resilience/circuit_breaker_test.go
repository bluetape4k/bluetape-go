package resilience_test

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/bluetape4k/bluetape-go/resilience"
)

func TestCircuitBreakerOpensAndRejectsCalls(t *testing.T) {
	now := time.Date(2026, 6, 3, 0, 0, 0, 0, time.UTC)
	var events []resilience.Event
	breaker, err := resilience.NewCircuitBreaker[int](resilience.CircuitBreakerOptions{
		Name:             "payments",
		FailureThreshold: 2,
		OpenTimeout:      time.Second,
		Now: func() time.Time {
			return now
		},
		OnEvent: func(_ context.Context, event resilience.Event) {
			events = append(events, event)
		},
	})
	if err != nil {
		t.Fatalf("NewCircuitBreaker failed: %v", err)
	}

	calls := 0
	operationErr := errors.New("downstream failed")
	operation := func(context.Context) (int, error) {
		calls++
		return 0, operationErr
	}

	for range 2 {
		_, err = resilience.Run(context.Background(), operation, breaker)
		if !errors.Is(err, operationErr) {
			t.Fatalf("expected operation error, got %v", err)
		}
	}
	if breaker.State() != resilience.CircuitStateOpen {
		t.Fatalf("state = %s, want open", breaker.State())
	}

	_, err = resilience.Run(context.Background(), operation, breaker)
	if !errors.Is(err, resilience.ErrCircuitOpen) {
		t.Fatalf("expected ErrCircuitOpen, got %v", err)
	}
	if calls != 2 {
		t.Fatalf("operation calls = %d, want 2", calls)
	}

	var openErr resilience.CircuitOpenError
	if !errors.As(err, &openErr) {
		t.Fatalf("expected CircuitOpenError, got %T", err)
	}
	if openErr.State != resilience.CircuitStateOpen {
		t.Fatalf("rejection state = %s, want open", openErr.State)
	}
	if len(events) < 2 || events[0].Kind != resilience.EventCircuitStateTransition || events[1].Kind != resilience.EventCircuitRejected {
		t.Fatalf("unexpected events: %#v", events)
	}
}

func TestCircuitBreakerHalfOpenSuccessClosesCircuit(t *testing.T) {
	now := time.Date(2026, 6, 3, 0, 0, 0, 0, time.UTC)
	breaker, err := resilience.NewCircuitBreaker[string](resilience.CircuitBreakerOptions{
		FailureThreshold: 1,
		SuccessThreshold: 1,
		OpenTimeout:      time.Second,
		Now: func() time.Time {
			return now
		},
	})
	if err != nil {
		t.Fatalf("NewCircuitBreaker failed: %v", err)
	}

	_, err = resilience.Run(context.Background(), func(context.Context) (string, error) {
		return "", errors.New("open circuit")
	}, breaker)
	if err == nil {
		t.Fatal("expected opening failure")
	}
	if breaker.State() != resilience.CircuitStateOpen {
		t.Fatalf("state = %s, want open", breaker.State())
	}

	now = now.Add(time.Second)
	got, err := resilience.Run(context.Background(), func(context.Context) (string, error) {
		return "ok", nil
	}, breaker)
	if err != nil {
		t.Fatalf("half-open probe failed: %v", err)
	}
	if got != "ok" {
		t.Fatalf("got %q, want ok", got)
	}
	if breaker.State() != resilience.CircuitStateClosed {
		t.Fatalf("state = %s, want closed", breaker.State())
	}
}

func TestCircuitBreakerHalfOpenFailureReopensCircuit(t *testing.T) {
	now := time.Date(2026, 6, 3, 0, 0, 0, 0, time.UTC)
	breaker, err := resilience.NewCircuitBreaker[int](resilience.CircuitBreakerOptions{
		FailureThreshold: 1,
		OpenTimeout:      time.Second,
		Now: func() time.Time {
			return now
		},
	})
	if err != nil {
		t.Fatalf("NewCircuitBreaker failed: %v", err)
	}

	_, _ = resilience.Run(context.Background(), func(context.Context) (int, error) {
		return 0, errors.New("open circuit")
	}, breaker)
	now = now.Add(time.Second)

	_, err = resilience.Run(context.Background(), func(context.Context) (int, error) {
		return 0, errors.New("probe failed")
	}, breaker)
	if err == nil {
		t.Fatal("expected probe failure")
	}
	if breaker.State() != resilience.CircuitStateOpen {
		t.Fatalf("state = %s, want open", breaker.State())
	}
}

func TestCircuitBreakerHalfOpenLimitsConcurrentProbes(t *testing.T) {
	now := time.Date(2026, 6, 3, 0, 0, 0, 0, time.UTC)
	breaker, err := resilience.NewCircuitBreaker[int](resilience.CircuitBreakerOptions{
		FailureThreshold:      1,
		SuccessThreshold:      2,
		OpenTimeout:           time.Second,
		HalfOpenMaxConcurrent: 1,
		Now: func() time.Time {
			return now
		},
	})
	if err != nil {
		t.Fatalf("NewCircuitBreaker failed: %v", err)
	}

	_, _ = resilience.Run(context.Background(), func(context.Context) (int, error) {
		return 0, errors.New("open circuit")
	}, breaker)
	now = now.Add(time.Second)

	entered := make(chan struct{})
	release := make(chan struct{})
	done := make(chan error, 1)
	go func() {
		_, err := resilience.Run(context.Background(), func(context.Context) (int, error) {
			close(entered)
			<-release
			return 1, nil
		}, breaker)
		done <- err
	}()

	<-entered
	_, err = resilience.Run(context.Background(), func(context.Context) (int, error) {
		t.Fatal("second half-open probe must not execute")
		return 0, nil
	}, breaker)
	if !errors.Is(err, resilience.ErrCircuitOpen) {
		t.Fatalf("expected ErrCircuitOpen, got %v", err)
	}

	close(release)
	if err := <-done; err != nil {
		t.Fatalf("first half-open probe failed: %v", err)
	}
}

func TestCircuitBreakerConcurrentCallsAreRaceSafe(t *testing.T) {
	breaker, err := resilience.NewCircuitBreaker[int](resilience.CircuitBreakerOptions{
		FailureThreshold: 1000,
		OpenTimeout:      time.Second,
	})
	if err != nil {
		t.Fatalf("NewCircuitBreaker failed: %v", err)
	}

	var wg sync.WaitGroup
	for range 20 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = resilience.Run(context.Background(), func(context.Context) (int, error) {
				return 1, nil
			}, breaker)
		}()
	}
	wg.Wait()

	if breaker.State() != resilience.CircuitStateClosed {
		t.Fatalf("state = %s, want closed", breaker.State())
	}
}

func TestCircuitBreakerHalfOpenStressLimitsConcurrentProbes(t *testing.T) {
	const (
		halfOpenLimit = int32(4)
		workers       = int32(64)
	)

	now := time.Date(2026, 6, 3, 0, 0, 0, 0, time.UTC)
	breaker, err := resilience.NewCircuitBreaker[int](resilience.CircuitBreakerOptions{
		FailureThreshold:      1,
		SuccessThreshold:      int(halfOpenLimit),
		OpenTimeout:           time.Second,
		HalfOpenMaxConcurrent: int(halfOpenLimit),
		Now: func() time.Time {
			return now
		},
	})
	if err != nil {
		t.Fatalf("NewCircuitBreaker failed: %v", err)
	}

	_, _ = resilience.Run(context.Background(), func(context.Context) (int, error) {
		return 0, errors.New("open circuit")
	}, breaker)
	now = now.Add(time.Second)

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
			}, breaker)

			switch {
			case err == nil:
			case errors.Is(err, resilience.ErrCircuitOpen):
				rejected.Add(1)
			default:
				errs <- err
			}
		}()
	}

	close(start)
	waitForAtomicAtLeast(t, &entered, halfOpenLimit)
	waitForAtomicSumAtLeast(t, &entered, &rejected, workers)

	if got := maxObserved.Load(); got > halfOpenLimit {
		t.Fatalf("max half-open probes = %d, want <= %d", got, halfOpenLimit)
	}
	if got := entered.Load(); got != halfOpenLimit {
		t.Fatalf("entered probes = %d, want %d", got, halfOpenLimit)
	}
	if got := rejected.Load(); got != workers-halfOpenLimit {
		t.Fatalf("rejected probes = %d, want %d", got, workers-halfOpenLimit)
	}

	close(release)
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatalf("unexpected worker error: %v", err)
	}

	if got := current.Load(); got != 0 {
		t.Fatalf("current probes = %d, want 0", got)
	}
	if breaker.State() != resilience.CircuitStateClosed {
		t.Fatalf("state = %s, want closed", breaker.State())
	}
}

func TestCircuitBreakerValidatesOptions(t *testing.T) {
	if _, err := resilience.NewCircuitBreaker[int](resilience.CircuitBreakerOptions{
		FailureThreshold: 0,
		OpenTimeout:      time.Second,
	}); err == nil {
		t.Fatal("expected failure threshold validation error")
	}
	if _, err := resilience.NewCircuitBreaker[int](resilience.CircuitBreakerOptions{
		FailureThreshold: 1,
		OpenTimeout:      0,
	}); err == nil {
		t.Fatal("expected open timeout validation error")
	}
}

func recordAtomicMax(maximum *atomic.Int32, candidate int32) {
	for {
		current := maximum.Load()
		if candidate <= current || maximum.CompareAndSwap(current, candidate) {
			return
		}
	}
}

func waitForAtomicAtLeast(t *testing.T, value *atomic.Int32, minimum int32) {
	t.Helper()

	deadline := time.After(2 * time.Second)
	tick := time.NewTicker(time.Millisecond)
	defer tick.Stop()

	for value.Load() < minimum {
		select {
		case <-deadline:
			t.Fatalf("value = %d, want at least %d", value.Load(), minimum)
		case <-tick.C:
		}
	}
}

func waitForAtomicSumAtLeast(t *testing.T, left, right *atomic.Int32, minimum int32) {
	t.Helper()

	deadline := time.After(2 * time.Second)
	tick := time.NewTicker(time.Millisecond)
	defer tick.Stop()

	for left.Load()+right.Load() < minimum {
		select {
		case <-deadline:
			t.Fatalf("sum = %d, want at least %d", left.Load()+right.Load(), minimum)
		case <-tick.C:
		}
	}
}
