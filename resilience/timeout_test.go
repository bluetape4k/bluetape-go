package resilience_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/bluetape4k/bluetape-go/resilience"
	concurrencytest "github.com/bluetape4k/bluetape-go/testing/concurrency"
)

func TestTimeoutWrapsOwnDeadline(t *testing.T) {
	var events []resilience.Event
	timeout, err := resilience.NewTimeout[string](resilience.TimeoutOptions{
		Name:    "backend",
		Timeout: 5 * time.Millisecond,
		OnEvent: func(_ context.Context, event resilience.Event) {
			events = append(events, event)
		},
	})
	if err != nil {
		t.Fatalf("NewTimeout failed: %v", err)
	}

	_, err = resilience.Run(context.Background(), func(ctx context.Context) (string, error) {
		<-ctx.Done()
		return "", ctx.Err()
	}, timeout)
	if !errors.Is(err, resilience.ErrTimeout) {
		t.Fatalf("expected ErrTimeout, got %v", err)
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected context deadline in chain, got %v", err)
	}

	var timeoutErr resilience.TimeoutError
	if !errors.As(err, &timeoutErr) {
		t.Fatalf("expected TimeoutError, got %T", err)
	}
	if timeoutErr.Timeout != 5*time.Millisecond {
		t.Fatalf("timeout = %s, want 5ms", timeoutErr.Timeout)
	}
	if len(events) != 1 || events[0].Kind != resilience.EventTimeout {
		t.Fatalf("events = %v, want one timeout event", events)
	}
}

func TestTimeoutReturnsParentCancellation(t *testing.T) {
	timeout, err := resilience.NewTimeout[int](resilience.TimeoutOptions{
		Timeout: time.Second,
	})
	if err != nil {
		t.Fatalf("NewTimeout failed: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err = resilience.Run(ctx, func(context.Context) (int, error) {
		t.Fatal("operation should not run for a cancelled context")
		return 0, nil
	}, timeout)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
	if errors.Is(err, resilience.ErrTimeout) {
		t.Fatalf("parent cancellation should not be reported as timeout: %v", err)
	}
}

func TestTimeoutParentCancellationUsesAsyncJobTester(t *testing.T) {
	timeout, err := resilience.NewTimeout[int](resilience.TimeoutOptions{
		Timeout: time.Second,
	})
	if err != nil {
		t.Fatalf("NewTimeout failed: %v", err)
	}

	tester := concurrencytest.NewAsyncJobTester(concurrencytest.Options{
		Workers:       2,
		RoundsPerTask: 32,
		Timeout:       time.Second,
	})

	tester.RunT(t, func(context.Context) error {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		_, err := resilience.Run(ctx, func(context.Context) (int, error) {
			return 0, errors.New("operation should not run for a cancelled context")
		}, timeout)
		if !errors.Is(err, context.Canceled) {
			return fmt.Errorf("Run error = %w, want context.Canceled", err)
		}
		if errors.Is(err, resilience.ErrTimeout) {
			return fmt.Errorf("Run error = %w, should not wrap ErrTimeout", err)
		}
		return nil
	})
}
