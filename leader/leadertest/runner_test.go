package leadertest

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/bluetape4k/bluetape-go/leader"
)

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
