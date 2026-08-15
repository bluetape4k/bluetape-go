package resilience_test

import (
	"context"
	"errors"
	"testing"

	"github.com/bluetape4k/bluetape-go/resilience"
)

func TestNonRetryablePreservesCauseAndMarker(t *testing.T) {
	cause := errors.New("response already committed")
	err := resilience.NonRetryable(cause)
	if err == nil {
		t.Fatal("NonRetryable() returned nil")
	}
	if !errors.Is(err, resilience.ErrNonRetryable) {
		t.Fatalf("error = %v, want ErrNonRetryable", err)
	}
	if !errors.Is(err, cause) {
		t.Fatalf("error = %v, want cause in chain", err)
	}
	if !resilience.IsNonRetryable(err) {
		t.Fatal("IsNonRetryable() = false, want true")
	}
	var typed resilience.NonRetryableError
	if !errors.As(err, &typed) {
		t.Fatalf("error type = %T, want NonRetryableError", err)
	}
	if typed.Cause != cause {
		t.Fatalf("Cause = %v, want %v", typed.Cause, cause)
	}
	if resilience.NonRetryable(nil) != nil || resilience.IsNonRetryable(nil) {
		t.Fatal("nil must remain nil and unmarked")
	}
}

func TestRetryStopsNonRetryableBeforeCustomPredicate(t *testing.T) {
	var retryIfCalls int
	var events []resilience.Event
	retry, err := resilience.NewRetry[int](resilience.RetryOptions{
		MaxAttempts: 3,
		RetryIf: func(error) bool {
			retryIfCalls++
			return true
		},
		OnEvent: func(_ context.Context, event resilience.Event) {
			events = append(events, event)
		},
		Sleeper: &fakeSleeper{},
	})
	if err != nil {
		t.Fatalf("NewRetry() error = %v", err)
	}

	_, err = resilience.Run(context.Background(), func(context.Context) (int, error) {
		return 0, resilience.NonRetryable(errors.New("committed"))
	}, retry)
	if !resilience.IsNonRetryable(err) {
		t.Fatalf("Run() error = %v, want non-retryable", err)
	}
	if retryIfCalls != 0 {
		t.Fatalf("RetryIf calls = %d, want 0", retryIfCalls)
	}
	if len(events) != 1 || events[0].Kind != resilience.EventFailure {
		t.Fatalf("events = %#v, want one failure", events)
	}
}

func TestCircuitRecordsNonRetryableCanceledAsFailure(t *testing.T) {
	circuit, err := resilience.NewCircuitBreaker[int](resilience.CircuitBreakerOptions{
		FailureThreshold: 1,
		OpenTimeout:      1,
	})
	if err != nil {
		t.Fatalf("NewCircuitBreaker() error = %v", err)
	}

	_, err = resilience.Run(context.Background(), func(context.Context) (int, error) {
		return 0, resilience.NonRetryable(context.Canceled)
	}, circuit)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Run() error = %v, want context.Canceled", err)
	}
	if circuit.State() != resilience.CircuitStateOpen {
		t.Fatalf("circuit state = %s, want open", circuit.State())
	}
}
