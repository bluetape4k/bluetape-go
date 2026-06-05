package workflow

import (
	"context"
	"errors"
	"testing"

	"github.com/bluetape4k/bluetape-go/workreport"
)

func TestConditionalRunsOnlyTrueBranch(t *testing.T) {
	trueRuns := 0
	falseRuns := 0

	report := Conditional(
		"branch",
		func(context.Context) (bool, error) { return true, nil },
		func(context.Context) workreport.Report {
			trueRuns++
			return workreport.Completed("true")
		},
		func(context.Context) workreport.Report {
			falseRuns++
			return workreport.Completed("false")
		},
	).Run(context.Background())

	if trueRuns != 1 || falseRuns != 0 {
		t.Fatalf("trueRuns=%d falseRuns=%d, want 1/0", trueRuns, falseRuns)
	}
	if report.Status != workreport.StatusCompleted || len(report.Children) != 1 {
		t.Fatalf("report = %+v", report)
	}
}

func TestConditionalRunsOnlyFalseBranch(t *testing.T) {
	trueRuns := 0
	falseRuns := 0

	report := Conditional(
		"branch",
		func(context.Context) (bool, error) { return false, nil },
		func(context.Context) workreport.Report {
			trueRuns++
			return workreport.Completed("true")
		},
		func(context.Context) workreport.Report {
			falseRuns++
			return workreport.Completed("false")
		},
	).Run(context.Background())

	if trueRuns != 0 || falseRuns != 1 {
		t.Fatalf("trueRuns=%d falseRuns=%d, want 0/1", trueRuns, falseRuns)
	}
	if report.Status != workreport.StatusCompleted || len(report.Children) != 1 {
		t.Fatalf("report = %+v", report)
	}
}

func TestConditionalFalseWithoutBranchCompletes(t *testing.T) {
	report := Conditional(
		"branch",
		func(context.Context) (bool, error) { return false, nil },
		func(context.Context) workreport.Report { return workreport.Completed("true") },
	).Run(context.Background())

	if report.Status != workreport.StatusCompleted {
		t.Fatalf("status = %q, want completed", report.Status)
	}
	if len(report.Children) != 0 {
		t.Fatalf("children = %d, want 0", len(report.Children))
	}
}

func TestConditionalPredicateErrorAndCancellation(t *testing.T) {
	expectedErr := errors.New("predicate failed")
	failed := Conditional(
		"branch",
		func(context.Context) (bool, error) { return false, expectedErr },
		func(context.Context) workreport.Report { return workreport.Completed("true") },
	).Run(context.Background())

	if failed.Status != workreport.StatusFailed || !errors.Is(failed.Err, expectedErr) {
		t.Fatalf("failed predicate report = %+v", failed)
	}

	cancelled := Conditional(
		"branch",
		func(context.Context) (bool, error) { return false, context.Canceled },
		func(context.Context) workreport.Report { return workreport.Completed("true") },
	).Run(context.Background())

	if cancelled.Status != workreport.StatusCancelled || !errors.Is(cancelled.Err, context.Canceled) {
		t.Fatalf("cancelled predicate report = %+v", cancelled)
	}
}

func TestConditionalValidationErrors(t *testing.T) {
	nilPredicate := Conditional(
		"branch",
		nil,
		func(context.Context) workreport.Report { return workreport.Completed("true") },
	).Run(context.Background())
	if nilPredicate.Status != workreport.StatusFailed || !errors.Is(nilPredicate.Err, ErrNilPredicate) {
		t.Fatalf("nil predicate report = %+v", nilPredicate)
	}

	tooManyFalseBranches := Conditional(
		"branch",
		func(context.Context) (bool, error) { return false, nil },
		func(context.Context) workreport.Report { return workreport.Completed("true") },
		func(context.Context) workreport.Report { return workreport.Completed("false-a") },
		func(context.Context) workreport.Report { return workreport.Completed("false-b") },
	).Run(context.Background())
	if tooManyFalseBranches.Status != workreport.StatusFailed ||
		!errors.Is(tooManyFalseBranches.Err, ErrTooManyFalseBranches) {
		t.Fatalf("too many false branches report = %+v", tooManyFalseBranches)
	}
}

func TestConditionalNilSelectedWorkFails(t *testing.T) {
	report := Conditional(
		"branch",
		func(context.Context) (bool, error) { return true, nil },
		nil,
	).Run(context.Background())

	if report.Status != workreport.StatusFailed || !errors.Is(report.Err, ErrNilWork) {
		t.Fatalf("report = %+v", report)
	}
	if len(report.Children) != 1 {
		t.Fatalf("children = %d, want 1", len(report.Children))
	}
}
