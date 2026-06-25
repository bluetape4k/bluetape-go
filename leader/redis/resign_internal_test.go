package redisleader

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestElectorResignHonorsCallerDeadlineWhileRenewalWorkerIsBlocked(t *testing.T) {
	blockedDone := make(chan struct{})
	elector := &Elector{
		owned: true,
		cancel: func() {
		},
		done: blockedDone,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	started := time.Now()
	err := elector.Resign(ctx)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Resign error = %v, want context deadline", err)
	}
	if elapsed := time.Since(started); elapsed > 250*time.Millisecond {
		t.Fatalf("Resign ignored caller deadline, elapsed=%s", elapsed)
	}
}

func TestGroupElectorResignHonorsCallerDeadlineWhileRenewalWorkerIsBlocked(t *testing.T) {
	blockedDone := make(chan struct{})
	elector := &GroupElector{
		owned: true,
		cancel: func() {
		},
		done: blockedDone,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	started := time.Now()
	err := elector.Resign(ctx)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Resign error = %v, want context deadline", err)
	}
	if elapsed := time.Since(started); elapsed > 250*time.Millisecond {
		t.Fatalf("Resign ignored caller deadline, elapsed=%s", elapsed)
	}
}
