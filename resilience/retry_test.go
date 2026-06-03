package resilience_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/bluetape4k/bluetape-go/resilience"
)

type fakeSleeper struct {
	delays []time.Duration
	err    error
}

func (s *fakeSleeper) Sleep(_ context.Context, delay time.Duration) error {
	s.delays = append(s.delays, delay)
	return s.err
}

func TestRetryRetriesUntilSuccess(t *testing.T) {
	expected := errors.New("temporary")
	sleeper := &fakeSleeper{}
	var events []resilience.Event
	var attempts int

	retry, err := resilience.NewRetry[string](resilience.RetryOptions{
		Name:        "backend",
		MaxAttempts: 3,
		Backoff:     resilience.ConstantBackoff(10 * time.Millisecond),
		Sleeper:     sleeper,
		OnEvent: func(_ context.Context, event resilience.Event) {
			events = append(events, event)
		},
	})
	if err != nil {
		t.Fatalf("NewRetry failed: %v", err)
	}

	got, err := resilience.Run(context.Background(), func(context.Context) (string, error) {
		attempts++
		if attempts == 1 {
			return "", expected
		}
		return "ok", nil
	}, retry)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	if got != "ok" || attempts != 2 {
		t.Fatalf("got (%q, attempts=%d), want (ok, attempts=2)", got, attempts)
	}
	if len(sleeper.delays) != 1 || sleeper.delays[0] != 10*time.Millisecond {
		t.Fatalf("delays = %v, want [10ms]", sleeper.delays)
	}
	if len(events) != 2 {
		t.Fatalf("events = %v, want retry and success", events)
	}
	if events[0].Kind != resilience.EventRetry || events[0].Attempt != 1 {
		t.Fatalf("first event = %+v, want retry attempt 1", events[0])
	}
	if events[1].Kind != resilience.EventSuccess || events[1].Attempt != 2 {
		t.Fatalf("second event = %+v, want success attempt 2", events[1])
	}
}

func TestRetryReturnsRetryErrorWhenAttemptsExhausted(t *testing.T) {
	expected := errors.New("temporary")
	retry, err := resilience.NewRetry[int](resilience.RetryOptions{
		Name:        "backend",
		MaxAttempts: 2,
		Backoff:     resilience.NoBackoff(),
		Sleeper:     &fakeSleeper{},
	})
	if err != nil {
		t.Fatalf("NewRetry failed: %v", err)
	}

	_, err = resilience.Run(context.Background(), func(context.Context) (int, error) {
		return 0, expected
	}, retry)
	if !errors.Is(err, resilience.ErrRetryExhausted) {
		t.Fatalf("expected ErrRetryExhausted, got %v", err)
	}
	if !errors.Is(err, expected) {
		t.Fatalf("expected original error in chain, got %v", err)
	}

	var retryErr resilience.RetryError
	if !errors.As(err, &retryErr) {
		t.Fatalf("expected RetryError, got %T", err)
	}
	if retryErr.Attempts != 2 {
		t.Fatalf("attempts = %d, want 2", retryErr.Attempts)
	}
}

func TestRetryPredicateCanRejectErrors(t *testing.T) {
	expected := errors.New("do not retry")
	retry, err := resilience.NewRetry[int](resilience.RetryOptions{
		MaxAttempts: 3,
		RetryIf: func(error) bool {
			return false
		},
		Sleeper: &fakeSleeper{},
	})
	if err != nil {
		t.Fatalf("NewRetry failed: %v", err)
	}

	var attempts int
	_, err = resilience.Run(context.Background(), func(context.Context) (int, error) {
		attempts++
		return 0, expected
	}, retry)
	if !errors.Is(err, expected) {
		t.Fatalf("expected original error, got %v", err)
	}
	if attempts != 1 {
		t.Fatalf("attempts = %d, want 1", attempts)
	}
}

func TestRetryStopsWhenBackoffContextIsCanceled(t *testing.T) {
	retry, err := resilience.NewRetry[int](resilience.RetryOptions{
		MaxAttempts: 3,
		Backoff:     resilience.ConstantBackoff(time.Second),
		Sleeper: &fakeSleeper{
			err: context.Canceled,
		},
	})
	if err != nil {
		t.Fatalf("NewRetry failed: %v", err)
	}

	_, err = resilience.Run(context.Background(), func(context.Context) (int, error) {
		return 0, errors.New("temporary")
	}, retry)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
}

func TestRetryDoesNotRetryBareContextDeadline(t *testing.T) {
	retry, err := resilience.NewRetry[int](resilience.RetryOptions{
		MaxAttempts: 3,
		Sleeper:     &fakeSleeper{},
	})
	if err != nil {
		t.Fatalf("NewRetry failed: %v", err)
	}

	var attempts int
	_, err = resilience.Run(context.Background(), func(context.Context) (int, error) {
		attempts++
		return 0, context.DeadlineExceeded
	}, retry)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected context deadline, got %v", err)
	}
	if errors.Is(err, resilience.ErrRetryExhausted) {
		t.Fatalf("bare context deadline should not be wrapped as retry exhaustion: %v", err)
	}
	if attempts != 1 {
		t.Fatalf("attempts = %d, want 1", attempts)
	}
}
