package leadertest

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/bluetape4k/bluetape-go/leader"
)

var runIdentity atomic.Uint64

// Run executes every mandatory single-elector conformance case.
func Run(t *testing.T, harness Harness) {
	t.Helper()
	if err := validateHarness(harness); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name string
		run  func(*testing.T, Harness, leader.Options) error
	}{
		{"acquire-observe", evaluateAcquireObserve},
		{"owned-duplicate", evaluateOwnedDuplicate},
		{"campaign-in-progress", evaluateCampaignInProgress},
		{"contention-cancel", evaluateContentionCancel},
		{"campaign-lost-response", evaluateCampaignLostResponse},
		{"renewal", evaluateRenewal},
		{"renew-failure", evaluateRenewFailure},
		{"owner-loss", evaluateOwnerLoss},
		{"expiry-takeover", evaluateExpiryTakeover},
		{"resign-idempotent", evaluateResignIdempotent},
		{"resign-retry", evaluateResignRetry},
		{"stale-resign", evaluateStaleResign},
		{"exact-contention", evaluateExactContention},
		{"nil-context", evaluateNilContext},
		{"redaction", evaluateRedaction},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			opts := caseOptions(tc.name)
			normalized, err := opts.Normalize()
			if err != nil {
				t.Fatalf("normalize case options: %v", err)
			}
			if err := tc.run(t, harness, normalized); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func caseOptions(name string) leader.Options {
	id := runIdentity.Add(1)
	return leader.Options{
		Group:         fmt.Sprintf("leadertest-%s-%d", name, id),
		MemberID:      fmt.Sprintf("member-%d", id),
		Lease:         180 * time.Millisecond,
		RenewInterval: 30 * time.Millisecond,
		KeyPrefix:     "leadertest",
	}
}

func newElector(t *testing.T, h Harness, opts leader.Options) (leader.Elector, error) {
	t.Helper()
	elector, err := h.New(t, opts)
	if err != nil {
		return nil, fmt.Errorf("construct elector: %w", err)
	}
	if elector == nil {
		return nil, errors.New("factory returned nil elector")
	}
	return elector, nil
}

func evaluateAcquireObserve(t *testing.T, h Harness, opts leader.Options) error {
	elector, err := newElector(t, h, opts)
	if err != nil {
		return err
	}
	if err := elector.Campaign(context.Background()); err != nil {
		return fmt.Errorf("campaign: %w", err)
	}
	defer boundedResign(elector)
	if !elector.IsLeader() {
		return errors.New("elector did not report leadership")
	}
	owner, err := elector.Leader(context.Background())
	if err != nil || owner == "" {
		return fmt.Errorf("observe owner: owner=%q err=%w", owner, err)
	}
	return nil
}

func evaluateOwnedDuplicate(t *testing.T, h Harness, opts leader.Options) error {
	elector, err := newElector(t, h, opts)
	if err != nil {
		return err
	}
	if err := elector.Campaign(context.Background()); err != nil {
		return err
	}
	defer boundedResign(elector)
	if err := elector.Campaign(context.Background()); !errors.Is(err, leader.ErrAlreadyLeader) {
		return fmt.Errorf("duplicate campaign error = %v", err)
	}
	return nil
}

func evaluateCampaignInProgress(t *testing.T, h Harness, opts leader.Options) error {
	if err := h.Control.ReplaceOwner(context.Background(), opts, "blocking-owner"); err != nil {
		return err
	}
	elector, err := newElector(t, h, opts)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 80*time.Millisecond)
	defer cancel()
	result := make(chan error, 1)
	go func() { result <- elector.Campaign(ctx) }()
	if err := waitFor(60*time.Millisecond, func() bool {
		return h.Control.OperationCount(opts, OperationCampaign) > 0
	}); err != nil {
		return err
	}
	if err := elector.Campaign(context.Background()); !errors.Is(err, leader.ErrCampaignInProgress) {
		return fmt.Errorf("concurrent campaign error = %v", err)
	}
	if err := <-result; !errors.Is(err, context.DeadlineExceeded) {
		return fmt.Errorf("blocked campaign error = %v", err)
	}
	return nil
}

func evaluateContentionCancel(t *testing.T, h Harness, opts leader.Options) error {
	if err := h.Control.ReplaceOwner(context.Background(), opts, "live-owner"); err != nil {
		return err
	}
	elector, err := newElector(t, h, opts)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Millisecond)
	defer cancel()
	if err := elector.Campaign(ctx); !errors.Is(err, context.DeadlineExceeded) {
		return fmt.Errorf("contention error = %v", err)
	}
	owner, err := h.Control.Owner(context.Background(), opts)
	if err != nil || owner != "live-owner" {
		return fmt.Errorf("contention mutated owner: %q, %v", owner, err)
	}
	return nil
}

func evaluateCampaignLostResponse(t *testing.T, h Harness, opts leader.Options) error {
	cause := context.DeadlineExceeded
	if err := h.Control.FailNext(context.Background(), opts, OperationCampaign, cause); err != nil {
		return err
	}
	elector, err := newElector(t, h, opts)
	if err != nil {
		return err
	}
	err = elector.Campaign(context.Background())
	owner, probeErr := h.Control.Owner(context.Background(), opts)
	if probeErr != nil || owner == "" {
		return fmt.Errorf("lost response did not commit owner: %q, %v", owner, probeErr)
	}
	if err == nil {
		defer boundedResign(elector)
		return nil
	}
	var operationErr *leader.OperationError
	if !errors.As(err, &operationErr) || !errors.Is(err, cause) {
		return fmt.Errorf("lost response is not typed: %v", err)
	}
	if !errors.Is(err, leader.ErrCommitUnknown) {
		return fmt.Errorf("probe-indeterminate response lacks ErrCommitUnknown: %v", err)
	}
	if err := elector.Resign(context.Background()); err != nil {
		return fmt.Errorf("cleanup after lost response: %w", err)
	}
	return nil
}

func evaluateRenewal(t *testing.T, h Harness, opts leader.Options) error {
	elector, err := newElector(t, h, opts)
	if err != nil {
		return err
	}
	if err := elector.Campaign(context.Background()); err != nil {
		return err
	}
	defer boundedResign(elector)
	baseline := h.Control.OperationCount(opts, OperationRenew)
	return waitFor(3*opts.RenewInterval, func() bool {
		return h.Control.OperationCount(opts, OperationRenew) > baseline && elector.IsLeader()
	})
}

func evaluateRenewFailure(t *testing.T, h Harness, opts leader.Options) error {
	elector, err := newElector(t, h, opts)
	if err != nil {
		return err
	}
	if err := elector.Campaign(context.Background()); err != nil {
		return err
	}
	if err := h.Control.FailNext(context.Background(), opts, OperationRenew, errors.New("renew-failure")); err != nil {
		return err
	}
	if err := waitFor(4*opts.RenewInterval, func() bool { return !elector.IsLeader() }); err != nil {
		return err
	}
	count := h.Control.OperationCount(opts, OperationRenew)
	time.Sleep(2 * opts.RenewInterval)
	if after := h.Control.OperationCount(opts, OperationRenew); after != count {
		return fmt.Errorf("renew traffic continued: %d -> %d", count, after)
	}
	return boundedResign(elector)
}

func evaluateOwnerLoss(t *testing.T, h Harness, opts leader.Options) error {
	elector, err := newElector(t, h, opts)
	if err != nil {
		return err
	}
	if err := elector.Campaign(context.Background()); err != nil {
		return err
	}
	if err := h.Control.ReplaceOwner(context.Background(), opts, "replacement-owner"); err != nil {
		return err
	}
	if err := waitFor(4*opts.RenewInterval, func() bool { return !elector.IsLeader() }); err != nil {
		return err
	}
	if err := elector.Resign(context.Background()); err != nil {
		return err
	}
	owner, err := h.Control.Owner(context.Background(), opts)
	if err != nil || owner != "replacement-owner" {
		return fmt.Errorf("stale cleanup removed replacement: %q, %v", owner, err)
	}
	return nil
}

func evaluateExpiryTakeover(t *testing.T, h Harness, opts leader.Options) error {
	if err := h.Control.ReplaceOwner(context.Background(), opts, "expiring-owner"); err != nil {
		return err
	}
	elector, err := newElector(t, h, opts)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*opts.Lease)
	defer cancel()
	if err := elector.Campaign(ctx); err != nil {
		return fmt.Errorf("takeover after expiry: %w", err)
	}
	return boundedResign(elector)
}

func evaluateResignIdempotent(t *testing.T, h Harness, opts leader.Options) error {
	elector, err := newElector(t, h, opts)
	if err != nil {
		return err
	}
	if err := elector.Campaign(context.Background()); err != nil {
		return err
	}
	if err := elector.Resign(context.Background()); err != nil {
		return err
	}
	if err := elector.Resign(context.Background()); err != nil {
		return fmt.Errorf("second resign: %w", err)
	}
	return nil
}

func evaluateResignRetry(t *testing.T, h Harness, opts leader.Options) error {
	elector, err := newElector(t, h, opts)
	if err != nil {
		return err
	}
	if err := elector.Campaign(context.Background()); err != nil {
		return err
	}
	if err := h.Control.FailNext(context.Background(), opts, OperationResign, context.DeadlineExceeded); err != nil {
		return err
	}
	err = elector.Resign(context.Background())
	if err == nil || !errors.Is(err, leader.ErrCommitUnknown) {
		return fmt.Errorf("first resign error = %v", err)
	}
	if err := elector.Resign(context.Background()); err != nil {
		return fmt.Errorf("retry resign: %w", err)
	}
	return nil
}

func evaluateStaleResign(t *testing.T, h Harness, opts leader.Options) error {
	elector, err := newElector(t, h, opts)
	if err != nil {
		return err
	}
	if err := elector.Campaign(context.Background()); err != nil {
		return err
	}
	if err := h.Control.ReplaceOwner(context.Background(), opts, "new-owner"); err != nil {
		return err
	}
	if err := elector.Resign(context.Background()); err != nil {
		return err
	}
	owner, err := h.Control.Owner(context.Background(), opts)
	if err != nil || owner != "new-owner" {
		return fmt.Errorf("stale resign owner = %q, %v", owner, err)
	}
	return nil
}

func evaluateExactContention(t *testing.T, h Harness, opts leader.Options) error {
	const workers = 6
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	var wg sync.WaitGroup
	var successes atomic.Int64
	var winnerMu sync.Mutex
	var winner leader.Elector
	for range workers {
		elector, err := newElector(t, h, opts)
		if err != nil {
			return err
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := elector.Campaign(ctx); err == nil {
				successes.Add(1)
				winnerMu.Lock()
				winner = elector
				winnerMu.Unlock()
			}
		}()
	}
	wg.Wait()
	if successes.Load() != 1 {
		return fmt.Errorf("contention winners = %d, want 1", successes.Load())
	}
	if winner == nil {
		return errors.New("contention winner missing")
	}
	if err := winner.Resign(context.Background()); err != nil {
		return err
	}
	takeover, err := newElector(t, h, opts)
	if err != nil {
		return err
	}
	ctx2, cancel2 := context.WithTimeout(context.Background(), opts.Lease)
	defer cancel2()
	if err := takeover.Campaign(ctx2); err != nil {
		return fmt.Errorf("takeover: %w", err)
	}
	return boundedResign(takeover)
}

func evaluateNilContext(t *testing.T, h Harness, opts leader.Options) error {
	elector, err := newElector(t, h, opts)
	if err != nil {
		return err
	}
	before := h.Control.OperationCount(opts, OperationCampaign)
	if err := elector.Campaign(nil); !errors.Is(err, leader.ErrInvalidContext) {
		return fmt.Errorf("Campaign(nil) error = %v", err)
	}
	if _, err := elector.Leader(nil); !errors.Is(err, leader.ErrInvalidContext) {
		return fmt.Errorf("Leader(nil) error = %v", err)
	}
	if err := elector.Resign(nil); !errors.Is(err, leader.ErrInvalidContext) {
		return fmt.Errorf("Resign(nil) error = %v", err)
	}
	if after := h.Control.OperationCount(opts, OperationCampaign); after != before {
		return fmt.Errorf("nil context dispatched operation: %d -> %d", before, after)
	}
	return nil
}

func evaluateRedaction(t *testing.T, h Harness, opts leader.Options) error {
	const marker = "raw-secret-marker"
	if err := h.Control.FailNext(context.Background(), opts, OperationCampaign, errors.New(marker)); err != nil {
		return err
	}
	elector, err := newElector(t, h, opts)
	if err != nil {
		return err
	}
	err = elector.Campaign(context.Background())
	if err == nil {
		boundedResign(elector)
		return errors.New("redaction failure injection returned nil")
	}
	if strings.Contains(err.Error(), marker) || strings.Contains(err.Error(), opts.Group) {
		return errors.New("provider error leaked a forbidden marker")
	}
	if !errors.Is(err, leader.ErrCommitUnknown) {
		return fmt.Errorf("redaction error lacks commit-unknown: %v", err)
	}
	return boundedResign(elector)
}

func boundedResign(elector leader.Elector) error {
	ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
	defer cancel()
	return elector.Resign(ctx)
}

func waitFor(timeout time.Duration, condition func() bool) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if condition() {
			return nil
		}
		time.Sleep(time.Millisecond)
	}
	return errors.New("timed out waiting for conformance condition")
}
