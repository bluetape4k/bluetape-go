package bttesting

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"
)

// ContextOperation is a context-aware operation used by cancellation checks.
type ContextOperation func(context.Context) error

// WaiterProbe observes cancellation from a blocked waiter.
//
// The probe must call ready after it has reached the waiter state being tested.
type WaiterProbe func(context.Context, func()) error

// CleanupProbe observes cancellation-driven cleanup.
//
// The probe must call ready after it has started and cleaned after the
// underlying resource cleanup is visible.
type CleanupProbe func(context.Context, func(), func()) error

// CheckContextCanceled verifies that operation preserves context.Canceled.
func CheckContextCanceled(operation ContextOperation) error {
	if operation == nil {
		return fmt.Errorf("operation must not be nil")
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := operation(ctx)
	if !errors.Is(err, context.Canceled) {
		return expectedError("context.Canceled", err)
	}
	return nil
}

// RequireContextCanceled fails tb when operation does not preserve context.Canceled.
func RequireContextCanceled(tb testing.TB, operation ContextOperation) {
	tb.Helper()

	if err := CheckContextCanceled(operation); err != nil {
		tb.Fatalf("context cancellation assertion failed: %v", err)
	}
}

// CheckDeadlineExceeded verifies that operation preserves context.DeadlineExceeded.
func CheckDeadlineExceeded(timeout time.Duration, operation ContextOperation) error {
	if timeout <= 0 {
		return fmt.Errorf("timeout must be positive")
	}
	if operation == nil {
		return fmt.Errorf("operation must not be nil")
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	done := make(chan error, 1)
	go func() {
		done <- operation(ctx)
	}()

	select {
	case err := <-done:
		if !errors.Is(err, context.DeadlineExceeded) {
			return expectedError(context.DeadlineExceeded.Error(), err)
		}
		return nil
	case <-time.After(timeout * 2):
		return fmt.Errorf("operation did not return after %s deadline", timeout)
	}
}

// RequireDeadlineExceeded fails tb when operation does not preserve context.DeadlineExceeded.
func RequireDeadlineExceeded(tb testing.TB, timeout time.Duration, operation ContextOperation) {
	tb.Helper()

	if err := CheckDeadlineExceeded(timeout, operation); err != nil {
		tb.Fatalf("deadline assertion failed: %v", err)
	}
}

// CheckWaiterReleased verifies that a blocked waiter returns after cancellation.
func CheckWaiterReleased(timeout time.Duration, waiter WaiterProbe) error {
	if timeout <= 0 {
		return fmt.Errorf("timeout must be positive")
	}
	if waiter == nil {
		return fmt.Errorf("waiter must not be nil")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ready := make(chan struct{})
	done := make(chan error, 1)
	go func() {
		done <- waiter(ctx, closeOnce(ready))
	}()

	select {
	case <-ready:
	case err := <-done:
		return wrapProbeError("waiter returned before signaling ready", err)
	case <-time.After(timeout):
		return fmt.Errorf("waiter did not signal ready within %s", timeout)
	}

	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			return expectedError("context.Canceled after waiter cancellation", err)
		}
		return nil
	case <-time.After(timeout):
		return fmt.Errorf("waiter did not return within %s after cancellation", timeout)
	}
}

// RequireWaiterReleased fails tb when a blocked waiter does not return after cancellation.
func RequireWaiterReleased(tb testing.TB, timeout time.Duration, waiter WaiterProbe) {
	tb.Helper()

	if err := CheckWaiterReleased(timeout, waiter); err != nil {
		tb.Fatalf("waiter release assertion failed: %v", err)
	}
}

// CheckCleanupOnCancel verifies that cancellation triggers cleanup and returns.
func CheckCleanupOnCancel(timeout time.Duration, probe CleanupProbe) error {
	if timeout <= 0 {
		return fmt.Errorf("timeout must be positive")
	}
	if probe == nil {
		return fmt.Errorf("probe must not be nil")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ready := make(chan struct{})
	cleaned := make(chan struct{})
	done := make(chan error, 1)
	go func() {
		done <- probe(ctx, closeOnce(ready), closeOnce(cleaned))
	}()

	select {
	case <-ready:
	case err := <-done:
		return wrapProbeError("probe returned before signaling ready", err)
	case <-time.After(timeout):
		return fmt.Errorf("probe did not signal ready within %s", timeout)
	}

	cancel()

	var observedCleanup bool
	var observedReturn bool
	var runErr error
	timer := time.NewTimer(timeout)
	defer timer.Stop()

	for !observedCleanup || !observedReturn {
		select {
		case <-cleaned:
			observedCleanup = true
			cleaned = nil
		case runErr = <-done:
			observedReturn = true
			done = nil
		case <-timer.C:
			if !observedCleanup {
				return fmt.Errorf("cleanup was not observed within %s after cancellation", timeout)
			}
			return fmt.Errorf("probe did not return within %s after cancellation", timeout)
		}
	}

	if !errors.Is(runErr, context.Canceled) {
		return expectedError("context.Canceled after cleanup cancellation", runErr)
	}
	return nil
}

// RequireCleanupOnCancel fails tb when cancellation cleanup is not observed.
func RequireCleanupOnCancel(tb testing.TB, timeout time.Duration, probe CleanupProbe) {
	tb.Helper()

	if err := CheckCleanupOnCancel(timeout, probe); err != nil {
		tb.Fatalf("cleanup assertion failed: %v", err)
	}
}

func closeOnce(ch chan struct{}) func() {
	var once sync.Once
	return func() {
		once.Do(func() { close(ch) })
	}
}

func expectedError(want string, got error) error {
	if got == nil {
		return fmt.Errorf("expected %s, got <nil>", want)
	}
	return fmt.Errorf("expected %s, got %w", want, got)
}

func wrapProbeError(prefix string, err error) error {
	if err == nil {
		return fmt.Errorf("%s: <nil>", prefix)
	}
	return fmt.Errorf("%s: %w", prefix, err)
}
