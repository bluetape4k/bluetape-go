package concurrencytest

import (
	"context"
	"testing"
)

// GoroutineStressTester repeatedly runs tasks across a bounded goroutine pool.
//
// Use it for shared-state tests where the subject should survive ordinary
// goroutine contention and all failures should be collected instead of
// cancelling the run after the first failed task.
type GoroutineStressTester struct {
	options Options
}

// NewGoroutineStressTester creates a bounded goroutine stress tester.
func NewGoroutineStressTester(options Options) GoroutineStressTester {
	return GoroutineStressTester{options: options}
}

// Run executes every task for Options.RoundsPerTask rounds.
func (t GoroutineStressTester) Run(ctx context.Context, tasks ...Task) (Report, error) {
	return runAll(ctx, t.options, tasks)
}

// RunT executes every task and fails tb when any task fails.
func (t GoroutineStressTester) RunT(tb testing.TB, tasks ...Task) Report {
	tb.Helper()
	report, err := t.Run(context.Background(), tasks...)
	return fail(tb, report, err)
}
