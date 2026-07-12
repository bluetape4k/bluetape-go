package leadertest

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/bluetape4k/bluetape-go/leader"
)

func TestHarnessValidation(t *testing.T) {
	if validateHarness(Harness{}) == nil {
		t.Fatal("empty harness passed validation")
	}
	h := MemoryHarness()
	if err := validateHarness(h); err != nil {
		t.Fatalf("MemoryHarness validation error = %v", err)
	}
}

func TestMemoryControlRejectsInvalidCallsWithoutMutation(t *testing.T) {
	h := MemoryHarness()
	opts := leader.Options{Group: "control", MemberID: "member", Lease: 50 * time.Millisecond, RenewInterval: 10 * time.Millisecond}
	before := h.Control.OperationCount(opts, OperationCampaign)

	if err := h.Control.ReplaceOwner(nil, opts, "owner"); !errors.Is(err, leader.ErrInvalidContext) {
		t.Fatalf("ReplaceOwner(nil) error = %v", err)
	}
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	if err := h.Control.FailNext(canceled, opts, OperationCampaign, errors.New("cause")); !errors.Is(err, context.Canceled) {
		t.Fatalf("FailNext(canceled) error = %v", err)
	}
	if err := h.Control.FailNext(context.Background(), opts, Operation("bad"), errors.New("cause")); err == nil {
		t.Fatal("invalid operation passed")
	}
	if err := h.Control.FailNext(context.Background(), opts, OperationCampaign, nil); err == nil {
		t.Fatal("nil cause passed")
	}
	if err := h.Control.ReplaceOwner(context.Background(), opts, " "); err == nil {
		t.Fatal("blank owner passed")
	}
	if count := h.Control.OperationCount(opts, OperationCampaign); count != before {
		t.Fatalf("operation count changed: got %d, want %d", count, before)
	}
	owner, err := h.Control.Owner(context.Background(), opts)
	if err != nil || owner != "" {
		t.Fatalf("owner = %q, %v", owner, err)
	}
}

func TestMemoryCampaignFailureIsPostLinearizationAndOneShot(t *testing.T) {
	h := MemoryHarness()
	opts := leader.Options{Group: "lost-response", MemberID: "member", Lease: time.Second, RenewInterval: 100 * time.Millisecond}
	cause := context.DeadlineExceeded
	if err := h.Control.FailNext(context.Background(), opts, OperationCampaign, cause); err != nil {
		t.Fatalf("FailNext() error = %v", err)
	}
	elector, err := h.New(t, opts)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	err = elector.Campaign(context.Background())
	if !errors.Is(err, cause) || !errors.Is(err, leader.ErrCommitUnknown) {
		t.Fatalf("Campaign() error = %v", err)
	}
	var operationErr *leader.OperationError
	if !errors.As(err, &operationErr) {
		t.Fatalf("Campaign() error type = %T", err)
	}
	owner, err := h.Control.Owner(context.Background(), opts)
	if err != nil || owner == "" {
		t.Fatalf("committed owner = %q, %v", owner, err)
	}
	if count := h.Control.OperationCount(opts, OperationCampaign); count != 1 {
		t.Fatalf("campaign count = %d, want 1", count)
	}
	if err := elector.Resign(context.Background()); err != nil {
		t.Fatalf("cleanup Resign() error = %v", err)
	}
}

func TestMemoryReplaceOwnerAndCountsAreMonotonic(t *testing.T) {
	h := MemoryHarness()
	opts := leader.Options{Group: "replace", MemberID: "member", Lease: time.Second, RenewInterval: 100 * time.Millisecond}
	if err := h.Control.ReplaceOwner(context.Background(), opts, "replacement"); err != nil {
		t.Fatalf("ReplaceOwner() error = %v", err)
	}
	owner, err := h.Control.Owner(context.Background(), opts)
	if err != nil || owner != "replacement" {
		t.Fatalf("Owner() = %q, %v", owner, err)
	}
	elector, err := h.New(t, opts)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if err := elector.Campaign(ctx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Campaign() error = %v", err)
	}
	if count := h.Control.OperationCount(opts, OperationCampaign); count < 1 {
		t.Fatalf("campaign count = %d", count)
	}
}
