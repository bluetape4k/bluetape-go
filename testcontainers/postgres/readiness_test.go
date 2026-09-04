package postgrestestcontainer

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestWaitForReadyRetriesTransientConnectionFailures(t *testing.T) {
	var attempts int
	wantErr := errors.New("postgres is still starting")

	err := waitForReady(context.Background(), func(context.Context) error {
		attempts++
		if attempts < 3 {
			return wantErr
		}
		return nil
	})

	if err != nil {
		t.Fatalf("waitForReady: %v", err)
	}
	if attempts != 3 {
		t.Fatalf("attempts = %d, want 3", attempts)
	}
}

func TestWaitForReadyReturnsContextErrorAfterLastAttempt(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	err := waitForReady(ctx, func(context.Context) error {
		return errors.New("postgres is still starting")
	})

	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("waitForReady error = %v, want context deadline exceeded", err)
	}
}
