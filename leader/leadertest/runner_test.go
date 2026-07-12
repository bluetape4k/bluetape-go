package leadertest

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/bluetape4k/bluetape-go/leader"
)

const diagnosticMarker = "forbidden-runner-diagnostic-marker"

func TestRunRedactsAdapterDiagnostics(t *testing.T) {
	if mode := os.Getenv("LEADERTEST_DIAGNOSTIC_MODE"); mode != "" {
		h := MemoryHarness()
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

	for _, mode := range []string{"factory", "control", "owner", "provider", "blocking"} {
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

func TestRunMemoryHarness(t *testing.T) {
	Run(t, MemoryHarness())
}

func TestBrokenOperationCountIsDetected(t *testing.T) {
	h := MemoryHarness()
	h.Control = zeroCountControl{Control: h.Control}
	opts, err := caseOptions("broken-count").Normalize()
	if err != nil {
		t.Fatal(err)
	}
	if err := evaluateRenewal(t, h, opts); err == nil {
		t.Fatal("zero-count control passed renewal evaluator")
	}
}

func TestBrokenOwnerInsensitiveResignIsDetected(t *testing.T) {
	h := MemoryHarness()
	opts, err := caseOptions("broken-stale").Normalize()
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
	opts, err := caseOptions("bare-context").Normalize()
	if err != nil {
		t.Fatal(err)
	}
	if err := evaluateCampaignLostResponse(t, h, opts); err == nil {
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
	if err := waitFor(time.Millisecond, func() bool { return false }); err == nil {
		t.Fatal("waitFor returned nil")
	}
}
