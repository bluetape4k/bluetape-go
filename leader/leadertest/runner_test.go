package leadertest

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/bluetape4k/bluetape-go/leader"
)

const diagnosticMarker = "forbidden-runner-diagnostic-marker"

func TestRunRedactsAdapterDiagnostics(t *testing.T) {
	if mode := os.Getenv("LEADERTEST_DIAGNOSTIC_MODE"); mode != "" {
		h := MemoryHarness()
		if mode == "abort" {
			release := make(chan struct{})
			h.New = func(testing.TB, leader.Options) (leader.Elector, error) {
				return abortDiagnosticElector{release: release}, nil
			}
			RunWithConfig(t, h, Config{
				Timing: containmentTestTiming(),
				Abort: func(context.Context, leader.Options) error {
					close(release)
					return errors.New(diagnosticMarker)
				},
			})
			return
		}
		switch mode {
		case "factory":
			h.New = func(testing.TB, leader.Options) (leader.Elector, error) {
				return nil, errors.New(diagnosticMarker)
			}
		case "control":
			h.Control = diagnosticControl{Control: h.Control, failControl: true}
		case "owner":
			h.Control = diagnosticControl{Control: h.Control, failOwner: true}
		case "provider":
			h.New = func(testing.TB, leader.Options) (leader.Elector, error) {
				return diagnosticElector{}, nil
			}
		case "blocking":
			h.New = func(testing.TB, leader.Options) (leader.Elector, error) {
				return blockingElector{}, nil
			}
		}
		Run(t, h)
		return
	}

	for _, mode := range []string{"factory", "control", "owner", "provider", "blocking", "abort"} {
		t.Run(mode, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestRunRedactsAdapterDiagnostics$")
			cmd.Env = append(os.Environ(), "LEADERTEST_DIAGNOSTIC_MODE="+mode)
			output, err := cmd.CombinedOutput()
			if err == nil {
				t.Fatal("broken adapter unexpectedly passed conformance")
			}
			if strings.Contains(string(output), diagnosticMarker) {
				t.Fatal("conformance diagnostics exposed an adapter marker")
			}
		})
	}
}

type diagnosticControl struct {
	Control
	failControl bool
	failOwner   bool
}

func (c diagnosticControl) ReplaceOwner(ctx context.Context, opts leader.Options, owner string) error {
	if c.failControl {
		return errors.New(diagnosticMarker)
	}
	return c.Control.ReplaceOwner(ctx, opts, owner)
}

func (c diagnosticControl) FailNext(ctx context.Context, opts leader.Options, operation Operation, cause error) error {
	if c.failControl {
		return errors.New(diagnosticMarker)
	}
	return c.Control.FailNext(ctx, opts, operation, cause)
}

func (c diagnosticControl) Owner(ctx context.Context, opts leader.Options) (string, error) {
	if c.failOwner {
		return diagnosticMarker, errors.New(diagnosticMarker)
	}
	return c.Control.Owner(ctx, opts)
}

type diagnosticElector struct{}

func (diagnosticElector) Campaign(context.Context) error { return errors.New(diagnosticMarker) }
func (diagnosticElector) Resign(context.Context) error   { return errors.New(diagnosticMarker) }
func (diagnosticElector) IsLeader() bool                 { return false }
func (diagnosticElector) Leader(context.Context) (string, error) {
	return diagnosticMarker, errors.New(diagnosticMarker)
}

type blockingElector struct{}

func (blockingElector) Campaign(context.Context) error { select {} }
func (blockingElector) Resign(context.Context) error   { select {} }
func (blockingElector) IsLeader() bool                 { return false }
func (blockingElector) Leader(context.Context) (string, error) {
	select {}
}

type abortDiagnosticElector struct{ release <-chan struct{} }

func (e abortDiagnosticElector) Campaign(ctx context.Context) error {
	<-ctx.Done()
	<-e.release
	return ctx.Err()
}
func (abortDiagnosticElector) Resign(context.Context) error { return nil }
func (abortDiagnosticElector) IsLeader() bool               { return false }
func (abortDiagnosticElector) Leader(context.Context) (string, error) {
	return "", nil
}

func TestRunMemoryHarness(t *testing.T) {
	Run(t, MemoryHarness())
}

func TestRunWithConfigCustomTiming(t *testing.T) {
	RunWithConfig(t, MemoryHarness(), Config{Timing: Timing{
		Lease:         200 * time.Millisecond,
		RenewInterval: 30 * time.Millisecond,
		CaseTimeout:   4 * time.Second,
		WaitTimeout:   time.Second,
		ResignTimeout: 200 * time.Millisecond,
	}})
}

func TestRunWithConfigCustomTimingRejectsBrokenProvider(t *testing.T) {
	if os.Getenv("LEADERTEST_CUSTOM_BROKEN") != "" {
		h := MemoryHarness()
		h.Control = zeroCountControl{Control: h.Control}
		RunWithConfig(t, h, Config{Timing: Timing{
			Lease:         100 * time.Millisecond,
			RenewInterval: 20 * time.Millisecond,
			CaseTimeout:   time.Second,
			WaitTimeout:   200 * time.Millisecond,
			ResignTimeout: 50 * time.Millisecond,
		}})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestRunWithConfigCustomTimingRejectsBrokenProvider$")
	cmd.Env = append(os.Environ(), "LEADERTEST_CUSTOM_BROKEN=1")
	output, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("broken provider passed custom timing profile:\n%s", output)
	}
	if text := string(output); !strings.Contains(text, "campaign-in-progress") || !strings.Contains(text, "conformance case failed") {
		t.Fatalf("custom profile did not fail the broken operation-count assertion:\n%s", output)
	}
}

func TestBrokenOperationCountIsDetected(t *testing.T) {
	h := MemoryHarness()
	h.Control = zeroCountControl{Control: h.Control}
	timing := defaultTestTiming(t)
	opts, err := caseOptions("broken-count", timing).Normalize()
	if err != nil {
		t.Fatal(err)
	}
	if err := evaluateRenewal(context.Background(), t, h, opts, timing); err == nil {
		t.Fatal("zero-count control passed renewal evaluator")
	}
}

func TestBrokenOwnerInsensitiveResignIsDetected(t *testing.T) {
	h := MemoryHarness()
	opts, err := caseOptions("broken-stale", defaultTestTiming(t)).Normalize()
	if err != nil {
		t.Fatal(err)
	}
	elector, err := h.New(t, opts)
	if err != nil {
		t.Fatal(err)
	}
	if err := elector.Campaign(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := h.Control.ReplaceOwner(context.Background(), opts, "replacement"); err != nil {
		t.Fatal(err)
	}
	if err := elector.Resign(context.Background()); err != nil {
		t.Fatal(err)
	}
	owner, err := h.Control.Owner(context.Background(), opts)
	if err != nil || owner != "replacement" {
		t.Fatalf("reference stale resign contract failed: %q, %v", owner, err)
	}
}

func TestEvaluateCampaignLostResponseRejectsBareContextError(t *testing.T) {
	h := MemoryHarness()
	h.New = func(testing.TB, leader.Options) (leader.Elector, error) {
		return bareContextElector{}, nil
	}
	timing := defaultTestTiming(t)
	opts, err := caseOptions("bare-context", timing).Normalize()
	if err != nil {
		t.Fatal(err)
	}
	if err := evaluateCampaignLostResponse(context.Background(), t, h, opts, timing); err == nil {
		t.Fatal("bare context error passed lost-response evaluator")
	}
}

type zeroCountControl struct{ Control }

func (zeroCountControl) OperationCount(leader.Options, Operation) int64 { return 0 }

type bareContextElector struct{}

func (bareContextElector) Campaign(context.Context) error { return context.DeadlineExceeded }
func (bareContextElector) Resign(context.Context) error   { return nil }
func (bareContextElector) IsLeader() bool                 { return false }
func (bareContextElector) Leader(context.Context) (string, error) {
	return "", errors.New("unavailable")
}

func TestWaitForTimesOut(t *testing.T) {
	if err := waitFor(context.Background(), time.Millisecond, func() bool { return false }); err == nil {
		t.Fatal("waitFor returned nil")
	}
}

func TestWaitForRechecksConditionAtDeadline(t *testing.T) {
	var calls atomic.Int64
	if err := waitFor(context.Background(), time.Millisecond, func() bool {
		return calls.Add(1) == 2
	}); err != nil {
		t.Fatalf("waitFor missed a condition at the deadline: %v", err)
	}
}

func TestRunWithConfigCancelsJoinsAndStillContainsTimedOutCase(t *testing.T) {
	var mu sync.Mutex
	var events []string
	record := func(event string) {
		mu.Lock()
		defer mu.Unlock()
		events = append(events, event)
	}

	err := runEvaluator(t, MemoryHarness(), Config{
		Timing: containmentTestTiming(),
		Abort: func(context.Context, leader.Options) error {
			record("abort")
			return nil
		},
	}, "cancel-joins", func(ctx context.Context, _ *testing.T, _ Harness, _ leader.Options, _ Timing) error {
		<-ctx.Done()
		record("cancel")
		record("joined")
		return ctx.Err()
	})
	if !errors.Is(err, errConformanceCaseTimedOut) {
		t.Fatalf("runEvaluator error = %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	if !slices.Equal(events, []string{"cancel", "joined", "abort"}) {
		t.Fatalf("events = %v", events)
	}
}

func TestRunWithConfigTimedOutEvaluatorUsesFreshCleanupContext(t *testing.T) {
	elector := &timeoutCleanupElector{resigned: make(chan struct{})}
	h := MemoryHarness()
	h.New = func(testing.TB, leader.Options) (leader.Elector, error) { return elector, nil }

	err := runEvaluator(t, h, Config{Timing: containmentTestTiming()}, "fresh-cleanup", func(
		ctx context.Context,
		t *testing.T,
		h Harness,
		opts leader.Options,
		timing Timing,
	) error {
		candidate, createErr := newElector(t, h, opts)
		if createErr != nil {
			return createErr
		}
		if campaignErr := candidate.Campaign(ctx); campaignErr != nil {
			return campaignErr
		}
		defer func() { _ = boundedResign(ctx, candidate, timing) }()
		<-ctx.Done()
		return ctx.Err()
	})
	if !errors.Is(err, errConformanceCaseTimedOut) {
		t.Fatalf("runEvaluator error = %v", err)
	}
	select {
	case <-elector.resigned:
	default:
		t.Fatal("timed-out evaluator skipped cleanup after root cancellation")
	}
}

type timeoutCleanupElector struct {
	owned    atomic.Bool
	resigned chan struct{}
}

func (e *timeoutCleanupElector) Campaign(context.Context) error {
	e.owned.Store(true)
	return nil
}

func (e *timeoutCleanupElector) Resign(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if e.owned.CompareAndSwap(true, false) {
		close(e.resigned)
	}
	return nil
}

func (e *timeoutCleanupElector) IsLeader() bool { return e.owned.Load() }

func (*timeoutCleanupElector) Leader(context.Context) (string, error) { return "member", nil }

func TestRunWithConfigSuccessfulAbortJoinsInOrder(t *testing.T) {
	var mu sync.Mutex
	var events []string
	release := make(chan struct{})
	record := func(event string) {
		mu.Lock()
		defer mu.Unlock()
		events = append(events, event)
	}

	err := runEvaluator(t, MemoryHarness(), Config{
		Timing: containmentTestTiming(),
		Abort: func(context.Context, leader.Options) error {
			record("abort")
			close(release)
			return nil
		},
	}, "abort-joins", func(ctx context.Context, _ *testing.T, _ Harness, _ leader.Options, _ Timing) error {
		<-ctx.Done()
		record("cancel")
		<-release
		record("joined")
		return ctx.Err()
	})
	if !errors.Is(err, errConformanceCaseTimedOut) {
		t.Fatalf("runEvaluator error = %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	if !slices.Equal(events, []string{"cancel", "abort", "joined"}) {
		t.Fatalf("events = %v", events)
	}
}

func TestRunWithConfigPreservesAbortErrorAfterJoin(t *testing.T) {
	abortFailure := errors.New("abort containment failed")
	release := make(chan struct{})
	err := runEvaluator(t, MemoryHarness(), Config{
		Timing: containmentTestTiming(),
		Abort: func(context.Context, leader.Options) error {
			close(release)
			return abortFailure
		},
	}, "abort-error", func(ctx context.Context, _ *testing.T, _ Harness, _ leader.Options, _ Timing) error {
		<-ctx.Done()
		<-release
		return ctx.Err()
	})
	if !errors.Is(err, errConformanceCaseTimedOut) || !errors.Is(err, abortFailure) {
		t.Fatalf("runEvaluator error = %v, want timeout joined with abort failure", err)
	}
}

func TestRunWithConfigExactContentionJoinsEveryWorker(t *testing.T) {
	const workers = 6
	started := make(chan struct{}, workers)
	release := make(chan struct{})
	var joined atomic.Int64
	h := MemoryHarness()
	h.New = func(testing.TB, leader.Options) (leader.Elector, error) {
		return workerBlockingElector{started: started, release: release, joined: &joined}, nil
	}
	timing := defaultTestTiming(t)
	opts := caseOptions("worker-join", timing)

	ctx, cancel := context.WithCancel(context.Background())
	result := make(chan error, 1)
	go func() {
		result <- evaluateExactContention(ctx, t, h, opts, timing)
	}()
	for range workers {
		<-started
	}
	cancel()
	select {
	case err := <-result:
		t.Fatalf("evaluator returned before workers joined: %v", err)
	case <-time.After(20 * time.Millisecond):
	}
	close(release)
	select {
	case <-result:
	case <-time.After(time.Second):
		t.Fatal("evaluator did not join released workers")
	}
	if got := joined.Load(); got != workers {
		t.Fatalf("joined workers = %d, want %d", got, workers)
	}
}

type workerBlockingElector struct {
	started chan<- struct{}
	release <-chan struct{}
	joined  *atomic.Int64
}

func (e workerBlockingElector) Campaign(ctx context.Context) error {
	e.started <- struct{}{}
	<-ctx.Done()
	<-e.release
	e.joined.Add(1)
	return ctx.Err()
}

func (workerBlockingElector) Resign(context.Context) error { return nil }
func (workerBlockingElector) IsLeader() bool               { return false }
func (workerBlockingElector) Leader(context.Context) (string, error) {
	return "", nil
}

func TestRunWithConfigActualNestedBlockingProvider(t *testing.T) {
	mode := os.Getenv("LEADERTEST_NESTED_BLOCKING")
	if mode != "" {
		release := make(chan struct{})
		h := nestedBlockingHarness(release)
		config := Config{Timing: Timing{
			Lease:         50 * time.Millisecond,
			RenewInterval: 10 * time.Millisecond,
			CaseTimeout:   120 * time.Millisecond,
			WaitTimeout:   40 * time.Millisecond,
			ResignTimeout: 10 * time.Millisecond,
		}}
		if mode != "nil-abort" {
			config.Abort = func(context.Context, leader.Options) error {
				fmt.Fprintln(os.Stderr, "nested-event:abort")
				if mode == "successful-abort" {
					close(release)
					return nil
				}
				return errors.New("nested abort failed")
			}
		}
		RunWithConfig(t, h, config)
		t.Fatal("later case reached")
		return
	}

	for _, childMode := range []string{"successful-abort", "nil-abort", "failing-abort"} {
		t.Run(childMode, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestRunWithConfigActualNestedBlockingProvider$", "-test.timeout=700ms")
			cmd.Env = append(os.Environ(), "LEADERTEST_NESTED_BLOCKING="+childMode)
			output, err := cmd.CombinedOutput()
			if err == nil {
				t.Fatal("blocking provider unexpectedly passed conformance")
			}
			text := string(output)
			cancelAt := strings.Index(text, "nested-event:cancel")
			abortAt := strings.Index(text, "nested-event:abort")
			joinedAt := strings.Index(text, "nested-event:joined")
			switch childMode {
			case "successful-abort":
				if cancelAt < 0 || abortAt < cancelAt || joinedAt < abortAt {
					t.Fatalf("event order = cancel:%d abort:%d joined:%d\n%s", cancelAt, abortAt, joinedAt, text)
				}
				if strings.Contains(text, "test timed out") {
					t.Fatalf("successful abort reached outer fail-stop:\n%s", text)
				}
			default:
				if !strings.Contains(text, "test timed out") {
					t.Fatalf("unjoined nested provider did not fail-stop:\n%s", text)
				}
				if joinedAt >= 0 || strings.Contains(text, "later case reached") {
					t.Fatalf("unjoined provider escaped containment:\n%s", text)
				}
			}
		})
	}
}

type nestedBlockingElector struct {
	leader.Elector
	calls   atomic.Int64
	release <-chan struct{}
}

func (e *nestedBlockingElector) Campaign(ctx context.Context) error {
	switch e.calls.Add(1) {
	case 1:
		<-ctx.Done()
		fmt.Fprintln(os.Stderr, "nested-event:cancel")
		<-e.release
		fmt.Fprintln(os.Stderr, "nested-event:joined")
		return ctx.Err()
	case 2:
		return leader.ErrCampaignInProgress
	default:
		return e.Elector.Campaign(ctx)
	}
}

type nestedBlockingControl struct{ Control }

func (c nestedBlockingControl) OperationCount(opts leader.Options, operation Operation) int64 {
	if operation == OperationCampaign && strings.Contains(opts.Group, "campaign-in-progress") {
		return 1
	}
	return c.Control.OperationCount(opts, operation)
}

func nestedBlockingHarness(release <-chan struct{}) Harness {
	h := MemoryHarness()
	newElector := h.New
	h.New = func(tb testing.TB, opts leader.Options) (leader.Elector, error) {
		elector, err := newElector(tb, opts)
		if err != nil || !strings.Contains(opts.Group, "campaign-in-progress") {
			return elector, err
		}
		return &nestedBlockingElector{Elector: elector, release: release}, nil
	}
	h.Control = nestedBlockingControl{Control: h.Control}
	return h
}

func TestRunContainmentFailStopsUnjoinedWork(t *testing.T) {
	mode := os.Getenv("LEADERTEST_CONTAINMENT_MODE")
	if mode != "" {
		config := Config{Timing: containmentTestTiming()}
		if mode == "failing-abort" {
			config.Abort = func(context.Context, leader.Options) error {
				return errors.New("abort failed")
			}
		}
		runConformanceCase(t, MemoryHarness(), config, "unjoined", func(context.Context, *testing.T, Harness, leader.Options, Timing) error {
			select {}
		})
		t.Fatal("later case reached")
		return
	}

	for _, childMode := range []string{"nil-abort", "failing-abort"} {
		t.Run(childMode, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestRunContainmentFailStopsUnjoinedWork$", "-test.timeout=300ms")
			cmd.Env = append(os.Environ(), "LEADERTEST_CONTAINMENT_MODE="+childMode)
			output, err := cmd.CombinedOutput()
			if err == nil {
				t.Fatal("unjoined evaluator returned from containment")
			}
			if !strings.Contains(string(output), "test timed out") {
				t.Fatalf("child did not fail-stop at go test timeout:\n%s", output)
			}
			if strings.Contains(string(output), "later case reached") {
				t.Fatal("containment proceeded after unjoined work")
			}
		})
	}
}

func containmentTestTiming() Timing {
	return Timing{
		Lease:         50 * time.Millisecond,
		RenewInterval: 10 * time.Millisecond,
		CaseTimeout:   40 * time.Millisecond,
		WaitTimeout:   5 * time.Millisecond,
		ResignTimeout: 5 * time.Millisecond,
	}
}

func defaultTestTiming(t *testing.T) Timing {
	t.Helper()
	config, err := normalizeConfig(Config{})
	if err != nil {
		t.Fatal(err)
	}
	return config.Timing
}
