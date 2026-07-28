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

// Task 테스트 helper의 timeout, cancellation, cleanup에서 사용하는 함수 타입이다.
type Task func(context.Context) error

// Options 테스트 helper의 timeout, cancellation, cleanup에서 사용하는 구조체다.
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

// Report 테스트 helper의 timeout, cancellation, cleanup에서 사용하는 구조체다.
type Report struct {
	// Scheduled is the total number of task executions planned for the run.
	Scheduled int

	// Started is the number of task executions that began running.
	Started int

	// Completed is the number of task executions that returned nil.
	Completed int

	// Failures is the number of task, panic, or run-level errors captured.
	Failures int

	// Panics is the number of captured panics.
	Panics int

	// Skipped is the number of scheduled executions not started before cancellation.
	Skipped int

	// MaxConcurrent is the highest observed number of concurrently running tasks.
	MaxConcurrent int

	// Duration is the elapsed wall-clock time for the run.
	Duration time.Duration
}

// RunError 테스트 helper의 timeout, cancellation, cleanup에서 사용하는 구조체다.
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

// Is errors.Is 비교를 지원한다.
//
// 매개변수:
//   - target: 검사하거나 감쌀 오류 값이다.
func (e RunError) Is(target error) bool {
	for _, err := range e.Errors {
		if errors.Is(err, target) {
			return true
		}
	}
	return false
}

// As 테스트 helper의 timeout, cancellation, cleanup 동작을 수행한다.
//
// 매개변수:
//   - target: As에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
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
