package workreport

import (
	"context"
	"errors"
	"testing"
)

func TestConstructorsSetStatusAndFields(t *testing.T) {
	expectedErr := errors.New("downstream failed")

	tests := []struct {
		name   string
		report Report
		status Status
		err    error
		reason string
	}{
		{name: "completed", report: Completed("load"), status: StatusCompleted},
		{name: "failed", report: Failed("load", expectedErr), status: StatusFailed, err: expectedErr},
		{name: "partial", report: Partial("batch", Completed("a"), Failed("b", expectedErr)), status: StatusPartial},
		{name: "aborted", report: Aborted("batch", "policy stopped"), status: StatusAborted, reason: "policy stopped"},
		{name: "cancelled", report: Cancelled("batch", context.Canceled), status: StatusCancelled, err: context.Canceled},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.report.Status != tt.status {
				t.Fatalf("status = %q, want %q", tt.report.Status, tt.status)
			}
			if tt.report.Name == "" {
				t.Fatal("name should be set")
			}
			if tt.report.StartedAt.IsZero() || tt.report.EndedAt.IsZero() {
				t.Fatalf("timestamps should be set: %+v", tt.report)
			}
			if tt.report.EndedAt.Before(tt.report.StartedAt) {
				t.Fatalf("ended before started: %+v", tt.report)
			}
			if !errors.Is(tt.report.Err, tt.err) {
				t.Fatalf("err = %v, want %v", tt.report.Err, tt.err)
			}
			if tt.report.Reason != tt.reason {
				t.Fatalf("reason = %q, want %q", tt.report.Reason, tt.reason)
			}
		})
	}
}

func TestPredicatesAndZeroValue(t *testing.T) {
	var zero Report
	if zero.IsSuccess() || zero.IsFailure() || zero.IsTerminal() {
		t.Fatalf("zero report should be unknown and non-terminal: %+v", zero)
	}

	tests := []struct {
		name      string
		report    Report
		success   bool
		failure   bool
		failed    bool
		partial   bool
		aborted   bool
		cancelled bool
		terminal  bool
	}{
		{name: "completed", report: Completed("ok"), success: true, terminal: true},
		{name: "failed", report: Failed("fail", errors.New("bad")), failure: true, failed: true, terminal: true},
		{name: "partial", report: Partial("partial", Completed("a")), failure: true, partial: true, terminal: true},
		{name: "aborted", report: Aborted("abort", "stop"), failure: true, aborted: true, terminal: true},
		{name: "cancelled", report: Cancelled("cancel", context.Canceled), failure: true, cancelled: true, terminal: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.report.IsSuccess() != tt.success {
				t.Fatalf("IsSuccess = %v, want %v", tt.report.IsSuccess(), tt.success)
			}
			if tt.report.IsFailure() != tt.failure {
				t.Fatalf("IsFailure = %v, want %v", tt.report.IsFailure(), tt.failure)
			}
			if tt.report.IsFailed() != tt.failed {
				t.Fatalf("IsFailed = %v, want %v", tt.report.IsFailed(), tt.failed)
			}
			if tt.report.IsPartial() != tt.partial {
				t.Fatalf("IsPartial = %v, want %v", tt.report.IsPartial(), tt.partial)
			}
			if tt.report.IsAborted() != tt.aborted {
				t.Fatalf("IsAborted = %v, want %v", tt.report.IsAborted(), tt.aborted)
			}
			if tt.report.IsCancelled() != tt.cancelled {
				t.Fatalf("IsCancelled = %v, want %v", tt.report.IsCancelled(), tt.cancelled)
			}
			if tt.report.IsTerminal() != tt.terminal {
				t.Fatalf("IsTerminal = %v, want %v", tt.report.IsTerminal(), tt.terminal)
			}
		})
	}
}

func TestAggregateStopOnFailureStopsAtFirstNonCompletedChild(t *testing.T) {
	expectedErr := errors.New("branch failed")
	report, err := Aggregate(
		"workflow",
		StopOnFailure,
		Completed("a"),
		Failed("b", expectedErr),
		Completed("c"),
	)
	if err != nil {
		t.Fatalf("Aggregate failed: %v", err)
	}
	if report.Status != StatusFailed {
		t.Fatalf("status = %q, want failed", report.Status)
	}
	if !errors.Is(report.Err, expectedErr) {
		t.Fatalf("parent err = %v, want %v", report.Err, expectedErr)
	}
	if len(report.Children) != 2 {
		t.Fatalf("children = %d, want 2", len(report.Children))
	}
	if report.Children[0].Name != "a" || report.Children[1].Name != "b" {
		t.Fatalf("child order not preserved: %+v", report.Children)
	}
}

func TestAggregateContinueOnFailurePreservesAllChildren(t *testing.T) {
	expectedErr := errors.New("branch failed")
	report, err := Aggregate(
		"workflow",
		ContinueOnFailure,
		Completed("a"),
		Failed("b", expectedErr),
		Cancelled("c", context.Canceled),
	)
	if err != nil {
		t.Fatalf("Aggregate failed: %v", err)
	}
	if report.Status != StatusPartial {
		t.Fatalf("status = %q, want partial", report.Status)
	}
	if len(report.Children) != 3 {
		t.Fatalf("children = %d, want 3", len(report.Children))
	}
	if !errors.Is(report.Children[1].Err, expectedErr) {
		t.Fatalf("child failure err = %v, want %v", report.Children[1].Err, expectedErr)
	}
	if !errors.Is(report.Children[2].Err, context.Canceled) {
		t.Fatalf("child cancellation err = %v, want context.Canceled", report.Children[2].Err)
	}
}

func TestAggregateCompletedAndEmptyChildren(t *testing.T) {
	empty, err := Aggregate("empty", StopOnFailure)
	if err != nil {
		t.Fatalf("Aggregate empty failed: %v", err)
	}
	if empty.Status != StatusCompleted || len(empty.Children) != 0 {
		t.Fatalf("empty aggregate = %+v", empty)
	}

	allDone, err := Aggregate("all", ContinueOnFailure, Completed("a"), Completed("b"))
	if err != nil {
		t.Fatalf("Aggregate all completed failed: %v", err)
	}
	if allDone.Status != StatusCompleted {
		t.Fatalf("status = %q, want completed", allDone.Status)
	}
	if len(allDone.Children) != 2 {
		t.Fatalf("children = %d, want 2", len(allDone.Children))
	}
}

func TestAggregateUnknownFailurePolicy(t *testing.T) {
	_, err := Aggregate("bad", FailurePolicy(99), Completed("a"))
	if !errors.Is(err, ErrUnknownFailurePolicy) {
		t.Fatalf("expected ErrUnknownFailurePolicy, got %v", err)
	}

	var policyErr FailurePolicyError
	if !errors.As(err, &policyErr) {
		t.Fatalf("expected FailurePolicyError, got %T", err)
	}
	if policyErr.Policy != FailurePolicy(99) {
		t.Fatalf("policy = %d, want 99", policyErr.Policy)
	}
}

func TestChildrenAreCopied(t *testing.T) {
	children := []Report{Completed("a"), Completed("b")}
	report := Partial("parent", children...)
	children[0] = Failed("mutated", errors.New("bad"))

	if report.Children[0].Name != "a" || report.Children[0].Status != StatusCompleted {
		t.Fatalf("constructor should copy children, got %+v", report.Children[0])
	}

	report.Children[0] = Failed("external", errors.New("bad"))
	aggregated, err := Aggregate("aggregate", ContinueOnFailure, children...)
	if err != nil {
		t.Fatalf("Aggregate failed: %v", err)
	}
	children[0] = Completed("changed-again")
	if aggregated.Children[0].Name != "mutated" {
		t.Fatalf("aggregate should preserve copied child order/content, got %+v", aggregated.Children[0])
	}
}
