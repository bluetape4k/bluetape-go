package concurrencytest

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"
)

const (
	defaultWorkers       = 4
	defaultRoundsPerTask = 1
)

// Task is a context-aware test body.
type Task func(context.Context) error

// Options configures a tester run.
type Options struct {
	// Workers bounds the number of tasks that may run concurrently.
	Workers int
	// RoundsPerTask repeats every registered task this many times.
	RoundsPerTask int
	// Timeout cancels the run after the duration. A zero value disables the
	// tester-owned timeout and relies on the caller's context.
	Timeout time.Duration
}

func (o Options) normalize() (Options, error) {
	if o.Workers == 0 {
		o.Workers = defaultWorkers
	}
	if o.RoundsPerTask == 0 {
		o.RoundsPerTask = defaultRoundsPerTask
	}
	if o.Workers < 0 {
		return o, fmt.Errorf("workers must be positive")
	}
	if o.RoundsPerTask < 0 {
		return o, fmt.Errorf("rounds per task must be positive")
	}
	if o.Timeout < 0 {
		return o, fmt.Errorf("timeout must not be negative")
	}
	return o, nil
}

func withTimeout(ctx context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	if ctx == nil {
		ctx = context.Background()
	}
	if timeout <= 0 {
		return ctx, func() {}
	}
	return context.WithTimeout(ctx, timeout)
}

// Report summarizes a tester run.
type Report struct {
	Started       int
	Completed     int
	Failures      int
	Panics        int
	MaxConcurrent int
	Duration      time.Duration
}

// RunError contains one or more task failures captured during a tester run.
type RunError struct {
	Errors []error
}

func (e RunError) Error() string {
	if len(e.Errors) == 0 {
		return "concurrency test failed"
	}

	parts := make([]string, 0, len(e.Errors))
	for _, err := range e.Errors {
		if err != nil {
			parts = append(parts, err.Error())
		}
	}
	return "concurrency test failed: " + strings.Join(parts, "; ")
}

// Is reports whether any captured error matches target.
func (e RunError) Is(target error) bool {
	for _, err := range e.Errors {
		if errors.Is(err, target) {
			return true
		}
	}
	return false
}

// As reports whether any captured error can be assigned to target.
func (e RunError) As(target any) bool {
	for _, err := range e.Errors {
		if errors.As(err, target) {
			return true
		}
	}
	return false
}

func fail(t testing.TB, report Report, err error) Report {
	t.Helper()
	if err != nil {
		t.Fatalf("concurrency test failed after %d/%d completions: %v", report.Completed, report.Started, err)
	}
	return report
}
