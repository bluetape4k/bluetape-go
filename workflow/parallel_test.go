package workflow

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/bluetape4k/bluetape-go/workreport"
)

func TestParallelContinueOnFailurePreservesInputOrder(t *testing.T) {
	expectedErr := errors.New("write failed")

	report := Parallel(
		"fanout",
		workreport.ContinueOnFailure,
		func(context.Context) workreport.Report { return workreport.Completed("first") },
		func(context.Context) workreport.Report { return workreport.Failed("second", expectedErr) },
		func(context.Context) workreport.Report { return workreport.Completed("third") },
	).Run(context.Background())

	if report.Status != workreport.StatusPartial {
		t.Fatalf("status = %q, want partial", report.Status)
	}
	if len(report.Children) != 3 {
		t.Fatalf("children = %d, want 3", len(report.Children))
	}
	if report.Children[0].Name != "first" || report.Children[1].Name != "second" || report.Children[2].Name != "third" {
		t.Fatalf("child order not preserved: %+v", report.Children)
	}
	if !errors.Is(report.Children[1].Err, expectedErr) {
		t.Fatalf("child err = %v, want %v", report.Children[1].Err, expectedErr)
	}
}

func TestParallelStopOnFailureCancelsSiblingsAndKeepsCause(t *testing.T) {
	expectedErr := errors.New("fast failure")
	slowStarted := make(chan struct{})

	report := Parallel(
		"fanout",
		workreport.StopOnFailure,
		func(ctx context.Context) workreport.Report {
			close(slowStarted)
			<-ctx.Done()
			return workreport.Cancelled("slow", ctx.Err())
		},
		func(context.Context) workreport.Report {
			<-slowStarted
			return workreport.Failed("fast", expectedErr)
		},
	).Run(context.Background())

	if report.Status != workreport.StatusFailed {
		t.Fatalf("status = %q, want failed", report.Status)
	}
	if !errors.Is(report.Err, expectedErr) {
		t.Fatalf("err = %v, want %v", report.Err, expectedErr)
	}
	if len(report.Children) != 2 {
		t.Fatalf("children = %d, want 2", len(report.Children))
	}
	if !report.Children[0].IsCancelled() {
		t.Fatalf("slow child should observe cancellation: %+v", report.Children[0])
	}
}

func TestParallelCancelledChildCancelsSiblingsRegardlessOfPolicy(t *testing.T) {
	slowStarted := make(chan struct{})

	report := Parallel(
		"fanout",
		workreport.ContinueOnFailure,
		func(ctx context.Context) workreport.Report {
			close(slowStarted)
			<-ctx.Done()
			return workreport.Cancelled("slow", ctx.Err())
		},
		func(context.Context) workreport.Report {
			<-slowStarted
			return workreport.Cancelled("cancel", context.Canceled)
		},
	).Run(context.Background())

	if report.Status != workreport.StatusCancelled {
		t.Fatalf("status = %q, want cancelled", report.Status)
	}
	if !errors.Is(report.Err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", report.Err)
	}
}

func TestParallelUnknownFailurePolicyAndNilWork(t *testing.T) {
	ran := false
	invalid := Parallel(
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
	if invalid.Status != workreport.StatusFailed || !errors.Is(invalid.Err, workreport.ErrUnknownFailurePolicy) {
		t.Fatalf("invalid policy report = %+v", invalid)
	}

	nilWork := Parallel("nil", workreport.StopOnFailure, nil).Run(context.Background())
	if nilWork.Status != workreport.StatusFailed || !errors.Is(nilWork.Err, ErrNilWork) {
		t.Fatalf("nil work report = %+v", nilWork)
	}
}

func TestParallelCallerCancellationBeforeRun(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	report := Parallel(
		"cancelled",
		workreport.ContinueOnFailure,
		func(context.Context) workreport.Report { return workreport.Completed("work") },
	).Run(ctx)

	if report.Status != workreport.StatusCancelled {
		t.Fatalf("status = %q, want cancelled", report.Status)
	}
	if !errors.Is(report.Err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", report.Err)
	}
}

func TestParallelWaitsForStartedGoroutines(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	done := make(chan struct{})

	go func() {
		report := Parallel(
			"fanout",
			workreport.ContinueOnFailure,
			func(context.Context) workreport.Report {
				close(started)
				<-release
				return workreport.Completed("slow")
			},
		).Run(context.Background())
		if report.Status != workreport.StatusCompleted {
			t.Errorf("status = %q, want completed", report.Status)
		}
		close(done)
	}()

	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("parallel work did not start")
	}

	select {
	case <-done:
		t.Fatal("parallel runner returned before started work finished")
	case <-time.After(10 * time.Millisecond):
	}

	close(release)
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("parallel runner did not return after work finished")
	}
}
