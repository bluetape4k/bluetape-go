package redisleader

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestElectorIgnoresStaleRenewalWorkerState(t *testing.T) {
	oldDone := make(chan struct{})
	currentDone := make(chan struct{})
	elector := &Elector{owned: true, generation: 2, done: currentDone}

	elector.clearOwnershipAfterLoss(1, oldDone, true)

	if !elector.owned || elector.cleanup || elector.done != currentDone {
		t.Fatal("stale renewal worker changed current ownership state")
	}
}

func TestCampaignRetryDelayIsDeterministicAndBounded(t *testing.T) {
	var elapsed time.Duration
	for attempt := uint(0); attempt < 12; attempt++ {
		got := campaignRetryDelay("member:token", attempt)
		if got != campaignRetryDelay("member:token", attempt) {
			t.Fatal("campaign retry delay is not deterministic")
		}
		base := campaignRetryBase << min(attempt, uint(4))
		if base > campaignRetryCap {
			base = campaignRetryCap
		}
		if got < base*80/100 || got > base*120/100 {
			t.Fatalf("campaign retry delay %s is outside jitter bounds for %s", got, base)
		}
		elapsed += got
	}
	if elapsed < time.Second {
		t.Fatalf("twelve retry delays consume only %s, want at least one second", elapsed)
	}
}

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
