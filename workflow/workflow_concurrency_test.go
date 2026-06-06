package workflow

import (
	"context"
	"fmt"
	"testing"
	"time"

	concurrencytest "github.com/bluetape4k/bluetape-go/testing/concurrency"
	"github.com/bluetape4k/bluetape-go/workreport"
)

func TestParallelRunnerStress(t *testing.T) {
	tester := concurrencytest.NewGoroutineStressTester(concurrencytest.Options{
		Workers:       4,
		RoundsPerTask: 50,
		Timeout:       2 * time.Second,
	})

	_, err := tester.Run(context.Background(), func(ctx context.Context) error {
		report := Parallel(
			"stress",
			workreport.ContinueOnFailure,
			func(context.Context) workreport.Report { return workreport.Completed("a") },
			func(context.Context) workreport.Report { return workreport.Completed("b") },
			func(context.Context) workreport.Report { return workreport.Completed("c") },
		).Run(ctx)
		if !report.IsSuccess() || len(report.Children) != 3 {
			return fmt.Errorf("unexpected report: %+v", report)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("stress run failed: %v", err)
	}
}

func TestRunnerCancellationWithAsyncJobTester(t *testing.T) {
	tester := concurrencytest.NewAsyncJobTester(concurrencytest.Options{
		Workers:       2,
		RoundsPerTask: 20,
		Timeout:       2 * time.Second,
	})

	_, err := tester.Run(context.Background(), func(ctx context.Context) error {
		runCtx, cancel := context.WithCancel(ctx)
		cancel()

		report := Parallel(
			"cancelled",
			workreport.ContinueOnFailure,
			func(context.Context) workreport.Report { return workreport.Completed("unreached") },
		).Run(runCtx)
		if !report.IsCancelled() {
			return fmt.Errorf("expected cancelled report, got %+v", report)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("async cancellation run failed: %v", err)
	}
}
