package resilience_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/bluetape4k/bluetape-go/resilience"
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
