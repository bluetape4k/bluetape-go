package workreport

import (
	"context"
	"errors"
	"testing"
	"time"

	concurrencytest "github.com/bluetape4k/bluetape-go/testing/concurrency"
)

func TestAggregateStressPreservesImmutableChildInputs(t *testing.T) {
	expectedErr := errors.New("branch failed")
	children := []Report{
		Completed("a"),
		Failed("b", expectedErr),
		Completed("c"),
	}

	tester := concurrencytest.NewGoroutineStressTester(concurrencytest.Options{
		Workers:       4,
		RoundsPerTask: 20,
		Timeout:       2 * time.Second,
	})
	report, err := tester.Run(context.Background(), func(context.Context) error {
		got, err := Aggregate("workflow", ContinueOnFailure, children...)
		if err != nil {
			return err
		}
		if got.Status != StatusPartial {
			return errors.New("aggregate should be partial")
		}
		if len(got.Children) != len(children) {
			return errors.New("aggregate child count changed")
		}
		if !errors.Is(got.Children[1].Err, expectedErr) {
			return errors.New("aggregate did not preserve child error")
		}
		return nil
	})
	if err != nil {
		t.Fatalf("stress aggregate failed: report=%+v err=%v", report, err)
	}
}

func TestCancelledReportUsesAsyncJobTester(t *testing.T) {
	tester := concurrencytest.NewAsyncJobTester(concurrencytest.Options{
		Workers:       1,
		RoundsPerTask: 3,
	})
	report, err := tester.Run(context.Background(), func(context.Context) error {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		cancelled := Cancelled("job", ctx.Err())
		if !cancelled.IsCancelled() {
			return errors.New("report should be cancelled")
		}
		if !errors.Is(cancelled.Err, context.Canceled) {
			return errors.New("report should preserve caller cancellation")
		}
		return nil
	})
	if err != nil {
		t.Fatalf("async cancellation report failed: report=%+v err=%v", report, err)
	}
	if report.Completed != 3 {
		t.Fatalf("completed = %d, want 3", report.Completed)
	}
}
