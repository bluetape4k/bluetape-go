package workflow

import (
	"context"
	"errors"
	"testing"

	"github.com/bluetape4k/bluetape-go/workreport"
)

func TestSequentialStopOnFailureStopsAtFirstFailure(t *testing.T) {
	expectedErr := errors.New("write failed")
	ranAfterFailure := false

	report := Sequential(
		"import",
		workreport.StopOnFailure,
		func(context.Context) workreport.Report { return workreport.Completed("read") },
		func(context.Context) workreport.Report { return workreport.Failed("write", expectedErr) },
		func(context.Context) workreport.Report {
			ranAfterFailure = true
			return workreport.Completed("cleanup")
		},
	).Run(context.Background())

	if ranAfterFailure {
		t.Fatal("sequential runner should stop before work after failure")
	}
	if report.Status != workreport.StatusFailed {
		t.Fatalf("status = %q, want failed", report.Status)
	}
	if !errors.Is(report.Err, expectedErr) {
		t.Fatalf("err = %v, want %v", report.Err, expectedErr)
	}
	if len(report.Children) != 2 {
		t.Fatalf("children = %d, want 2", len(report.Children))
	}
}

func TestSequentialContinueOnFailurePreservesAllChildren(t *testing.T) {
	expectedErr := errors.New("write failed")

	report := Sequential(
		"import",
		workreport.ContinueOnFailure,
		func(context.Context) workreport.Report { return workreport.Completed("read") },
		func(context.Context) workreport.Report { return workreport.Failed("write", expectedErr) },
		func(context.Context) workreport.Report { return workreport.Completed("cleanup") },
	).Run(context.Background())

	if report.Status != workreport.StatusPartial {
		t.Fatalf("status = %q, want partial", report.Status)
	}
	if len(report.Children) != 3 {
		t.Fatalf("children = %d, want 3", len(report.Children))
	}
	if !errors.Is(report.Children[1].Err, expectedErr) {
		t.Fatalf("child err = %v, want %v", report.Children[1].Err, expectedErr)
	}
}

func TestSequentialCancelledStopsRegardlessOfPolicy(t *testing.T) {
	ranAfterCancel := false

	report := Sequential(
		"sync",
		workreport.ContinueOnFailure,
		func(context.Context) workreport.Report { return workreport.Completed("prepare") },
		func(context.Context) workreport.Report { return workreport.Cancelled("wait", context.Canceled) },
		func(context.Context) workreport.Report {
			ranAfterCancel = true
			return workreport.Completed("after")
		},
	).Run(context.Background())

	if ranAfterCancel {
		t.Fatal("sequential runner should stop after cancellation")
	}
	if report.Status != workreport.StatusCancelled {
		t.Fatalf("status = %q, want cancelled", report.Status)
	}
	if len(report.Children) != 2 {
		t.Fatalf("children = %d, want 2", len(report.Children))
	}
}

func TestSequentialAbortedStopsRegardlessOfPolicy(t *testing.T) {
	ranAfterAbort := false

	report := Sequential(
		"deploy",
		workreport.ContinueOnFailure,
		func(context.Context) workreport.Report { return workreport.Aborted("gate", "manual stop") },
		func(context.Context) workreport.Report {
			ranAfterAbort = true
			return workreport.Completed("after")
		},
	).Run(context.Background())

	if ranAfterAbort {
		t.Fatal("sequential runner should stop after abort")
	}
	if report.Status != workreport.StatusAborted {
		t.Fatalf("status = %q, want aborted", report.Status)
	}
	if report.Reason != "manual stop" {
		t.Fatalf("reason = %q, want manual stop", report.Reason)
	}
}

func TestSequentialNilWorkAndUnknownReportStatus(t *testing.T) {
	nilWork := Sequential("nil", workreport.StopOnFailure, nil).Run(context.Background())
	if nilWork.Status != workreport.StatusFailed || !errors.Is(nilWork.Err, ErrNilWork) {
		t.Fatalf("nil work report = %+v", nilWork)
	}

	unknown := Sequential(
		"unknown",
		workreport.StopOnFailure,
		func(context.Context) workreport.Report { return workreport.Report{} },
	).Run(context.Background())
	if unknown.Status != workreport.StatusFailed || !errors.Is(unknown.Err, ErrUnknownReportStatus) {
		t.Fatalf("unknown status report = %+v", unknown)
	}
}

func TestSequentialUnknownFailurePolicyDoesNotRunWork(t *testing.T) {
	ran := false
	report := Sequential(
		"bad-policy",
		workreport.FailurePolicy(99),
		func(context.Context) workreport.Report {
			ran = true
			return workreport.Completed("work")
		},
	).Run(context.Background())

	if ran {
		t.Fatal("work should not run when failure policy is invalid")
	}
	if report.Status != workreport.StatusFailed {
		t.Fatalf("status = %q, want failed", report.Status)
	}
	if !errors.Is(report.Err, workreport.ErrUnknownFailurePolicy) {
		t.Fatalf("err = %v, want ErrUnknownFailurePolicy", report.Err)
	}
}

func TestSequentialCancelledContextBeforeRun(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	report := Sequential(
		"cancelled",
		workreport.StopOnFailure,
		func(context.Context) workreport.Report { return workreport.Completed("work") },
	).Run(ctx)

	if report.Status != workreport.StatusCancelled {
		t.Fatalf("status = %q, want cancelled", report.Status)
	}
	if !errors.Is(report.Err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", report.Err)
	}
}
