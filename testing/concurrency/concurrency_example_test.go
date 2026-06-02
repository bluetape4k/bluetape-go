package concurrencytest_test

import (
	"context"
	"fmt"

	concurrencytest "github.com/bluetape4k/bluetape-go/testing/concurrency"
)

func ExampleGoroutineStressTester() {
	tester := concurrencytest.NewGoroutineStressTester(concurrencytest.Options{
		Workers:       1,
		RoundsPerTask: 3,
	})

	report, err := tester.Run(context.Background(), func(context.Context) error {
		return nil
	})
	if err != nil {
		return
	}

	fmt.Println(report.Started, report.Completed, report.Failures)

	// Output:
	// 3 3 0
}

func ExampleAsyncJobTester() {
	tester := concurrencytest.NewAsyncJobTester(concurrencytest.Options{
		Workers:       1,
		RoundsPerTask: 2,
	})

	report, err := tester.Run(context.Background(), func(context.Context) error {
		return nil
	})
	if err != nil {
		return
	}

	fmt.Println(report.Completed)

	// Output:
	// 2
}
