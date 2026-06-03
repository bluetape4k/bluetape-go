package resilience_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/bluetape4k/bluetape-go/resilience"
)

func TestComposeAppliesFirstPolicyAsOutermost(t *testing.T) {
	retry, err := resilience.NewRetry[int](resilience.RetryOptions{
		MaxAttempts: 2,
		Backoff:     resilience.NoBackoff(),
		Sleeper:     &fakeSleeper{},
	})
	if err != nil {
		t.Fatalf("NewRetry failed: %v", err)
	}
	timeout, err := resilience.NewTimeout[int](resilience.TimeoutOptions{
		Timeout: 5 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("NewTimeout failed: %v", err)
	}

	var attempts int
	_, err = resilience.Run(context.Background(), func(ctx context.Context) (int, error) {
		attempts++
		<-ctx.Done()
		return 0, ctx.Err()
	}, retry, timeout)
	if !errors.Is(err, resilience.ErrRetryExhausted) {
		t.Fatalf("expected retry exhaustion, got %v", err)
	}
	if !errors.Is(err, resilience.ErrTimeout) {
		t.Fatalf("expected timeout error in chain, got %v", err)
	}
	if attempts != 2 {
		t.Fatalf("attempts = %d, want 2", attempts)
	}
}
