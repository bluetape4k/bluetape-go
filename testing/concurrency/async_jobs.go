package concurrencytest

import (
	"context"
	"testing"
)

// AsyncJobTester runs context-aware asynchronous jobs with deterministic
// result collection.
//
// Use it for Go code whose correctness depends on cancellation, deadlines, or
// async error handling. It preserves the testing intent of Kotlin coroutine
// job stress tests without exposing coroutine-specific naming.
type AsyncJobTester struct {
	options Options
}

// NewAsyncJobTester creates an async job tester.
func NewAsyncJobTester(options Options) AsyncJobTester {
	return AsyncJobTester{options: options}
}

// Run executes every job for Options.RoundsPerTask rounds.
func (t AsyncJobTester) Run(ctx context.Context, jobs ...Task) (Report, error) {
	return runAll(ctx, t.options, jobs)
}

// RunT executes every job and fails tb when any job fails.
func (t AsyncJobTester) RunT(tb testing.TB, jobs ...Task) Report {
	tb.Helper()
	report, err := t.Run(context.Background(), jobs...)
	return fail(tb, report, err)
}
