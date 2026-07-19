package leadertest

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/bluetape4k/bluetape-go/leader"
)

var runIdentity atomic.Uint64

var (
	errConformanceCaseTimedOut = errors.New("leadertest: conformance case timed out")
	errConformanceAbortFailed  = errors.New("leadertest: abort containment failed")
)

type evaluator func(context.Context, *testing.T, Harness, leader.Options, Timing) error

// Run executes every mandatory single-elector conformance case with default timing.
func Run(t *testing.T, harness Harness) {
	RunWithConfig(t, harness, Config{})
}

// RunWithConfig executes every mandatory single-elector conformance case with config.
func RunWithConfig(t *testing.T, harness Harness, config Config) {
	t.Helper()
	normalized, err := normalizeConfig(config)
	if err != nil {
		t.Fatal("leadertest: invalid config")
	}
	if err := validateHarness(harness); err != nil {
		t.Fatal("leadertest: invalid harness")
	}
	cases := []struct {
		name string
		run  evaluator
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
		passed := t.Run(tc.name, func(t *testing.T) {
			runConformanceCase(t, harness, normalized, tc.name, tc.run)
		})
		if !passed {
			return
		}
	}
}

func runConformanceCase(t *testing.T, harness Harness, config Config, name string, run evaluator) {
	t.Helper()
	normalized, err := normalizeConfig(config)
	if err != nil {
		t.Fatal("leadertest: invalid config")
	}
	if err := validateHarness(harness); err != nil {
		t.Fatal("leadertest: invalid harness")
	}
	if err := runEvaluator(t, harness, normalized, name, run); err != nil {
		if errors.Is(err, errConformanceCaseTimedOut) {
			if errors.Is(err, errConformanceAbortFailed) {
				t.Fatal("leadertest: conformance case timed out after abort containment failure")
			}
			t.Fatal("leadertest: conformance case timed out")
		}
		t.Fatalf("leadertest: conformance case failed: %s", conformanceFailureReason(err))
	}
}

func runEvaluator(t *testing.T, harness Harness, config Config, name string, run evaluator) error {
	t.Helper()
	opts := caseOptions(name, config.Timing)
	normalized, err := opts.Normalize()
	if err != nil {
		return errors.New("leadertest: invalid case options")
	}

	root, cancel := context.WithCancel(context.Background())
	defer cancel()
	result := make(chan error, 1)
	go func() { result <- run(root, t, harness, normalized, config.Timing) }()

	timer := time.NewTimer(config.Timing.CaseTimeout)
	defer timer.Stop()
	select {
	case err := <-result:
		return err
	case <-timer.C:
	}

	cancel()
	joinGrace := min(config.Timing.ResignTimeout, config.Timing.CaseTimeout/10)
	joinTimer := time.NewTimer(joinGrace)
	joined := false
	select {
	case <-result:
		joinTimer.Stop()
		joined = true
	case <-joinTimer.C:
	}

	var abortErr error
	if config.Abort != nil {
		abortBudget := min(config.Timing.ResignTimeout, time.Second)
		abortCtx, abortCancel := context.WithTimeout(context.Background(), abortBudget)
		abortErr = config.Abort(abortCtx, normalized)
		abortCancel()
	}
	if !joined {
		<-result
	}
	if abortErr != nil {
		return errors.Join(errConformanceCaseTimedOut, errConformanceAbortFailed, abortErr)
	}
	return errConformanceCaseTimedOut
}

func conformanceFailureReason(err error) (reason string) {
	reason = "contract"
	defer func() { _ = recover() }()
	switch {
	case errors.Is(err, leader.ErrCommitUnknown):
		return "commit-unknown"
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		return "context"
	case errors.Is(err, leader.ErrAlreadyLeader), errors.Is(err, leader.ErrCampaignInProgress), errors.Is(err, leader.ErrCleanupPending), errors.Is(err, leader.ErrInvalidContext):
		return "state"
	default:
		var operationErr *leader.OperationError
		if errors.As(err, &operationErr) {
			return "provider"
		}
		return reason
	}
}

func caseOptions(name string, timing Timing) leader.Options {
	id := runIdentity.Add(1)
	return leader.Options{
		Group:         fmt.Sprintf("leadertest-%s-%d", name, id),
		MemberID:      fmt.Sprintf("member-%d", id),
		Lease:         timing.Lease,
		RenewInterval: timing.RenewInterval,
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

func evaluateAcquireObserve(ctx context.Context, t *testing.T, h Harness, opts leader.Options, timing Timing) error {
	elector, err := newElector(t, h, opts)
	if err != nil {
		return err
	}
	if err := elector.Campaign(ctx); err != nil {
		return fmt.Errorf("campaign: %w", err)
	}
	defer func() { _ = boundedResign(ctx, elector, timing) }()
	if !elector.IsLeader() {
		return errors.New("elector did not report leadership")
	}
	owner, err := elector.Leader(ctx)
	if err != nil || owner == "" {
		return fmt.Errorf("observe owner: owner=%q err=%w", owner, err)
	}
	return nil
}

func evaluateOwnedDuplicate(ctx context.Context, t *testing.T, h Harness, opts leader.Options, timing Timing) error {
	elector, err := newElector(t, h, opts)
	if err != nil {
		return err
	}
	if err := elector.Campaign(ctx); err != nil {
		return err
	}
	defer func() { _ = boundedResign(ctx, elector, timing) }()
	if err := elector.Campaign(ctx); !errors.Is(err, leader.ErrAlreadyLeader) {
		return fmt.Errorf("duplicate campaign error: %w", err)
	}
	return nil
}

func evaluateCampaignInProgress(ctx context.Context, t *testing.T, h Harness, opts leader.Options, timing Timing) error {
	if err := h.Control.ReplaceOwner(ctx, opts, "blocking-owner"); err != nil {
		return err
	}
	elector, err := newElector(t, h, opts)
	if err != nil {
		return err
	}
	campaignCtx, cancel := context.WithCancel(ctx)
	result := make(chan error, 1)
	go func() { result <- elector.Campaign(campaignCtx) }()
	joined := false
	defer func() {
		cancel()
		if !joined {
			<-result
		}
	}()
	if err := waitFor(ctx, timing.WaitTimeout, func() bool {
		return h.Control.OperationCount(opts, OperationCampaign) > 0
	}); err != nil {
		return err
	}
	if err := elector.Campaign(ctx); !errors.Is(err, leader.ErrCampaignInProgress) {
		return fmt.Errorf("concurrent campaign error: %w", err)
	}
	cancel()
	err = <-result
	joined = true
	if !errors.Is(err, context.Canceled) {
		return fmt.Errorf("blocked campaign error: %w", err)
	}
	return nil
}

func evaluateContentionCancel(ctx context.Context, t *testing.T, h Harness, opts leader.Options, timing Timing) error {
	if err := h.Control.ReplaceOwner(ctx, opts, "live-owner"); err != nil {
		return err
	}
	elector, err := newElector(t, h, opts)
	if err != nil {
		return err
	}
	campaignCtx, cancel := context.WithTimeout(ctx, scaledDuration(opts.RenewInterval, 2, timing.WaitTimeout))
	defer cancel()
	if err := elector.Campaign(campaignCtx); !errors.Is(err, context.DeadlineExceeded) {
		return fmt.Errorf("contention error: %w", err)
	}
	owner, err := h.Control.Owner(ctx, opts)
	if err != nil {
		return fmt.Errorf("observe contention owner: %w", err)
	}
	if owner != "live-owner" {
		return fmt.Errorf("contention mutated owner: %q", owner)
	}
	return nil
}

func evaluateCampaignLostResponse(ctx context.Context, t *testing.T, h Harness, opts leader.Options, timing Timing) error {
	cause := context.DeadlineExceeded
	if err := h.Control.FailNext(ctx, opts, OperationCampaign, cause); err != nil {
		return err
	}
	elector, err := newElector(t, h, opts)
	if err != nil {
		return err
	}
	err = elector.Campaign(ctx)
	owner, probeErr := h.Control.Owner(ctx, opts)
	if probeErr != nil {
		return fmt.Errorf("probe lost-response owner: %w", probeErr)
	}
	if owner == "" {
		return errors.New("lost response did not commit owner")
	}
	if err == nil {
		defer func() { _ = boundedResign(ctx, elector, timing) }()
		return nil
	}
	var operationErr *leader.OperationError
	if !errors.As(err, &operationErr) || !errors.Is(err, cause) {
		return fmt.Errorf("lost response is not typed: %w", err)
	}
	if !errors.Is(err, leader.ErrCommitUnknown) {
		return fmt.Errorf("probe-indeterminate response lacks ErrCommitUnknown: %w", err)
	}
	if err := elector.Resign(ctx); err != nil {
		return fmt.Errorf("cleanup after lost response: %w", err)
	}
	return nil
}

func evaluateRenewal(ctx context.Context, t *testing.T, h Harness, opts leader.Options, timing Timing) error {
	elector, err := newElector(t, h, opts)
	if err != nil {
		return err
	}
	if err := elector.Campaign(ctx); err != nil {
		return err
	}
	defer func() { _ = boundedResign(ctx, elector, timing) }()
	baseline := h.Control.OperationCount(opts, OperationRenew)
	return waitFor(ctx, scaledDuration(opts.RenewInterval, 3, timing.WaitTimeout), func() bool {
		return h.Control.OperationCount(opts, OperationRenew) > baseline && elector.IsLeader()
	})
}

func evaluateRenewFailure(ctx context.Context, t *testing.T, h Harness, opts leader.Options, timing Timing) error {
	elector, err := newElector(t, h, opts)
	if err != nil {
		return err
	}
	if err := elector.Campaign(ctx); err != nil {
		return err
	}
	if err := h.Control.FailNext(ctx, opts, OperationRenew, errors.New("renew-failure")); err != nil {
		return err
	}
	if err := waitFor(ctx, scaledDuration(opts.RenewInterval, 4, timing.WaitTimeout), func() bool { return !elector.IsLeader() }); err != nil {
		return err
	}
	count := h.Control.OperationCount(opts, OperationRenew)
	if err := sleepContext(ctx, scaledDuration(opts.RenewInterval, 2, timing.WaitTimeout)); err != nil {
		return err
	}
	if after := h.Control.OperationCount(opts, OperationRenew); after != count {
		return fmt.Errorf("renew traffic continued: %d -> %d", count, after)
	}
	return boundedResign(ctx, elector, timing)
}

func evaluateOwnerLoss(ctx context.Context, t *testing.T, h Harness, opts leader.Options, timing Timing) error {
	elector, err := newElector(t, h, opts)
	if err != nil {
		return err
	}
	if err := elector.Campaign(ctx); err != nil {
		return err
	}
	if err := h.Control.ReplaceOwner(ctx, opts, "replacement-owner"); err != nil {
		return err
	}
	if err := waitFor(ctx, scaledDuration(opts.RenewInterval, 4, timing.WaitTimeout), func() bool { return !elector.IsLeader() }); err != nil {
		return err
	}
	if err := elector.Resign(ctx); err != nil {
		return err
	}
	owner, err := h.Control.Owner(ctx, opts)
	if err != nil {
		return fmt.Errorf("observe replacement owner: %w", err)
	}
	if owner != "replacement-owner" {
		return fmt.Errorf("stale cleanup removed replacement: %q", owner)
	}
	return nil
}

func evaluateExpiryTakeover(ctx context.Context, t *testing.T, h Harness, opts leader.Options, timing Timing) error {
	if err := h.Control.ReplaceOwner(ctx, opts, "expiring-owner"); err != nil {
		return err
	}
	elector, err := newElector(t, h, opts)
	if err != nil {
		return err
	}
	campaignCtx, cancel := context.WithTimeout(ctx, scaledDuration(opts.Lease, 3, timing.WaitTimeout))
	defer cancel()
	if err := elector.Campaign(campaignCtx); err != nil {
		return fmt.Errorf("takeover after expiry: %w", err)
	}
	return boundedResign(ctx, elector, timing)
}

func evaluateResignIdempotent(ctx context.Context, t *testing.T, h Harness, opts leader.Options, _ Timing) error {
	elector, err := newElector(t, h, opts)
	if err != nil {
		return err
	}
	if err := elector.Campaign(ctx); err != nil {
		return err
	}
	if err := elector.Resign(ctx); err != nil {
		return err
	}
	if err := elector.Resign(ctx); err != nil {
		return fmt.Errorf("second resign: %w", err)
	}
	return nil
}

func evaluateResignRetry(ctx context.Context, t *testing.T, h Harness, opts leader.Options, _ Timing) error {
	elector, err := newElector(t, h, opts)
	if err != nil {
		return err
	}
	if err := elector.Campaign(ctx); err != nil {
		return err
	}
	if err := h.Control.FailNext(ctx, opts, OperationResign, context.DeadlineExceeded); err != nil {
		return err
	}
	err = elector.Resign(ctx)
	if err == nil || !errors.Is(err, leader.ErrCommitUnknown) {
		return fmt.Errorf("first resign error: %w", err)
	}
	if err := elector.Resign(ctx); err != nil {
		return fmt.Errorf("retry resign: %w", err)
	}
	return nil
}

func evaluateStaleResign(ctx context.Context, t *testing.T, h Harness, opts leader.Options, _ Timing) error {
	elector, err := newElector(t, h, opts)
	if err != nil {
		return err
	}
	if err := elector.Campaign(ctx); err != nil {
		return err
	}
	if err := h.Control.ReplaceOwner(ctx, opts, "new-owner"); err != nil {
		return err
	}
	if err := elector.Resign(ctx); err != nil {
		return err
	}
	owner, err := h.Control.Owner(ctx, opts)
	if err != nil {
		return fmt.Errorf("observe stale resign owner: %w", err)
	}
	if owner != "new-owner" {
		return fmt.Errorf("stale resign owner = %q", owner)
	}
	return nil
}

func evaluateExactContention(ctx context.Context, t *testing.T, h Harness, opts leader.Options, timing Timing) error {
	const workers = 6
	campaignCtx, cancel := context.WithCancel(ctx)
	type campaignResult struct {
		elector leader.Elector
		err     error
	}
	results := make(chan campaignResult, workers)
	var winner leader.Elector
	started := 0
	joined := 0
	defer func() {
		cancel()
		for joined < started {
			<-results
			joined++
		}
	}()
	for range workers {
		elector, err := newElector(t, h, opts)
		if err != nil {
			return err
		}
		go func() {
			results <- campaignResult{elector: elector, err: elector.Campaign(campaignCtx)}
		}()
		started++
	}
	successes := 0
	waitCtx, waitCancel := context.WithTimeout(ctx, timing.WaitTimeout)
	defer waitCancel()
	for range workers {
		select {
		case result := <-results:
			joined++
			if result.err == nil {
				successes++
				winner = result.elector
				cancel()
			}
		case <-waitCtx.Done():
			return errors.New("contention workers did not return")
		}
	}
	if successes != 1 {
		return fmt.Errorf("contention winners = %d, want 1", successes)
	}
	if winner == nil {
		return errors.New("contention winner missing")
	}
	if err := winner.Resign(ctx); err != nil {
		return err
	}
	takeover, err := newElector(t, h, opts)
	if err != nil {
		return err
	}
	takeoverCtx, takeoverCancel := context.WithTimeout(ctx, min(opts.Lease, timing.WaitTimeout))
	defer takeoverCancel()
	if err := takeover.Campaign(takeoverCtx); err != nil {
		return fmt.Errorf("takeover: %w", err)
	}
	return boundedResign(ctx, takeover, timing)
}

func evaluateNilContext(_ context.Context, t *testing.T, h Harness, opts leader.Options, _ Timing) error {
	elector, err := newElector(t, h, opts)
	if err != nil {
		return err
	}
	before := h.Control.OperationCount(opts, OperationCampaign)
	if err := elector.Campaign(nil); !errors.Is(err, leader.ErrInvalidContext) { //nolint:staticcheck // nil is the contract input under test.
		return fmt.Errorf("Campaign(nil) error: %w", err)
	}
	if _, err := elector.Leader(nil); !errors.Is(err, leader.ErrInvalidContext) { //nolint:staticcheck // nil is the contract input under test.
		return fmt.Errorf("Leader(nil) error: %w", err)
	}
	if err := elector.Resign(nil); !errors.Is(err, leader.ErrInvalidContext) { //nolint:staticcheck // nil is the contract input under test.
		return fmt.Errorf("Resign(nil) error: %w", err)
	}
	if after := h.Control.OperationCount(opts, OperationCampaign); after != before {
		return fmt.Errorf("nil context dispatched operation: %d -> %d", before, after)
	}
	return nil
}

func evaluateRedaction(ctx context.Context, t *testing.T, h Harness, opts leader.Options, timing Timing) error {
	const marker = "raw-secret-marker"
	if err := h.Control.FailNext(ctx, opts, OperationCampaign, errors.New(marker)); err != nil {
		return err
	}
	elector, err := newElector(t, h, opts)
	if err != nil {
		return err
	}
	err = elector.Campaign(ctx)
	if err == nil {
		_ = boundedResign(ctx, elector, timing)
		return errors.New("redaction failure injection returned nil")
	}
	if strings.Contains(err.Error(), marker) || strings.Contains(err.Error(), opts.Group) {
		return errors.New("provider error leaked a forbidden marker")
	}
	if !errors.Is(err, leader.ErrCommitUnknown) {
		return fmt.Errorf("redaction error lacks commit-unknown: %w", err)
	}
	return boundedResign(ctx, elector, timing)
}

func boundedResign(ctx context.Context, elector leader.Elector, timing Timing) error {
	_ = ctx // Case cancellation must not cancel provider cleanup.
	resignCtx, cancel := context.WithTimeout(context.Background(), timing.ResignTimeout)
	defer cancel()
	return elector.Resign(resignCtx)
}

func waitFor(ctx context.Context, timeout time.Duration, condition func() bool) error {
	waitCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	ticker := time.NewTicker(15 * time.Millisecond)
	defer ticker.Stop()
	for {
		if condition() {
			return nil
		}
		select {
		case <-waitCtx.Done():
			if condition() {
				return nil
			}
			if err := ctx.Err(); err != nil {
				return err
			}
			return errors.New("timed out waiting for conformance condition")
		case <-ticker.C:
		}
	}
}

func sleepContext(ctx context.Context, duration time.Duration) error {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func scaledDuration(duration time.Duration, factor int, limit time.Duration) time.Duration {
	if duration > limit/time.Duration(factor) {
		return limit
	}
	return min(time.Duration(factor)*duration, limit)
}
