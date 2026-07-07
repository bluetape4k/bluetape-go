package bttesting_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	bttesting "github.com/bluetape4k/bluetape-go/testing"
	concurrencytest "github.com/bluetape4k/bluetape-go/testing/concurrency"
)

func TestCheckContextCanceled(t *testing.T) {
	err := bttesting.CheckContextCanceled(func(ctx context.Context) error {
		return ctx.Err()
	})
	if err != nil {
		t.Fatalf("CheckContextCanceled error = %v", err)
	}
}

func TestCheckContextCanceledDiagnostics(t *testing.T) {
	tests := []struct {
		name      string
		operation bttesting.ContextOperation
		want      string
	}{
		{name: "nil operation", operation: nil, want: "operation must not be nil"},
		{
			name: "nil error",
			operation: func(context.Context) error {
				return nil
			},
			want: "expected context.Canceled",
		},
		{
			name: "wrong error",
			operation: func(context.Context) error {
				return errors.New("boom")
			},
			want: "expected context.Canceled",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := bttesting.CheckContextCanceled(tt.operation)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %v, want containing %q", err, tt.want)
			}
		})
	}
}

func TestCheckDeadlineExceeded(t *testing.T) {
	err := bttesting.CheckDeadlineExceeded(10*time.Millisecond, func(ctx context.Context) error {
		<-ctx.Done()
		return ctx.Err()
	})
	if err != nil {
		t.Fatalf("CheckDeadlineExceeded error = %v", err)
	}
}

func TestCancellationHelpersUseAsyncJobTester(t *testing.T) {
	tester := concurrencytest.NewAsyncJobTester(concurrencytest.Options{
		Workers:       2,
		RoundsPerTask: 16,
		Timeout:       2 * time.Second,
	})

	tester.RunT(t, func(context.Context) error {
		if err := bttesting.CheckDeadlineExceeded(5*time.Millisecond, func(ctx context.Context) error {
			<-ctx.Done()
			return ctx.Err()
		}); err != nil {
			return err
		}
		if err := bttesting.CheckWaiterReleased(50*time.Millisecond, func(ctx context.Context, ready func()) error {
			ready()
			<-ctx.Done()
			return ctx.Err()
		}); err != nil {
			return err
		}
		return bttesting.CheckCleanupOnCancel(50*time.Millisecond, func(ctx context.Context, ready func(), cleaned func()) error {
			ready()
			<-ctx.Done()
			cleaned()
			return ctx.Err()
		})
	})
}

func TestCheckDeadlineExceededDiagnostics(t *testing.T) {
	tests := []struct {
		name      string
		timeout   time.Duration
		operation bttesting.ContextOperation
		want      string
	}{
		{name: "invalid timeout", timeout: 0, operation: func(context.Context) error { return nil }, want: "timeout must be positive"},
		{name: "nil operation", timeout: time.Millisecond, operation: nil, want: "operation must not be nil"},
		{
			name:    "wrong error",
			timeout: time.Millisecond,
			operation: func(context.Context) error {
				return context.Canceled
			},
			want: "expected context deadline exceeded",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := bttesting.CheckDeadlineExceeded(tt.timeout, tt.operation)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %v, want containing %q", err, tt.want)
			}
		})
	}
}

func TestCheckWaiterReleased(t *testing.T) {
	err := bttesting.CheckWaiterReleased(50*time.Millisecond, func(ctx context.Context, ready func()) error {
		ready()
		<-ctx.Done()
		return ctx.Err()
	})
	if err != nil {
		t.Fatalf("CheckWaiterReleased error = %v", err)
	}
}

func TestCheckWaiterReleasedDiagnostics(t *testing.T) {
	tests := []struct {
		name    string
		timeout time.Duration
		waiter  bttesting.WaiterProbe
		want    string
	}{
		{name: "invalid timeout", timeout: 0, waiter: func(context.Context, func()) error { return nil }, want: "timeout must be positive"},
		{name: "nil waiter", timeout: time.Millisecond, waiter: nil, want: "waiter must not be nil"},
		{
			name:    "not ready",
			timeout: 10 * time.Millisecond,
			waiter: func(ctx context.Context, _ func()) error {
				<-ctx.Done()
				return ctx.Err()
			},
			want: "waiter did not signal ready",
		},
		{
			name:    "wrong error",
			timeout: 10 * time.Millisecond,
			waiter: func(_ context.Context, ready func()) error {
				ready()
				return errors.New("boom")
			},
			want: "expected context.Canceled",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := bttesting.CheckWaiterReleased(tt.timeout, tt.waiter)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %v, want containing %q", err, tt.want)
			}
		})
	}
}

func TestCheckWaiterReleasedReportsUnreleasedWaiter(t *testing.T) {
	release := make(chan struct{})
	err := bttesting.CheckWaiterReleased(10*time.Millisecond, func(_ context.Context, ready func()) error {
		ready()
		<-release
		return context.Canceled
	})
	close(release)

	if err == nil || !strings.Contains(err.Error(), "waiter did not return") {
		t.Fatalf("error = %v, want waiter did not return", err)
	}
}

func TestCheckCleanupOnCancel(t *testing.T) {
	err := bttesting.CheckCleanupOnCancel(50*time.Millisecond, func(ctx context.Context, ready func(), cleaned func()) error {
		ready()
		<-ctx.Done()
		cleaned()
		return ctx.Err()
	})
	if err != nil {
		t.Fatalf("CheckCleanupOnCancel error = %v", err)
	}
}

func TestCheckCleanupOnCancelDiagnostics(t *testing.T) {
	tests := []struct {
		name    string
		timeout time.Duration
		probe   bttesting.CleanupProbe
		want    string
	}{
		{name: "invalid timeout", timeout: 0, probe: func(context.Context, func(), func()) error { return nil }, want: "timeout must be positive"},
		{name: "nil probe", timeout: time.Millisecond, probe: nil, want: "probe must not be nil"},
		{
			name:    "not cleaned",
			timeout: 10 * time.Millisecond,
			probe: func(ctx context.Context, ready func(), _ func()) error {
				ready()
				<-ctx.Done()
				return ctx.Err()
			},
			want: "cleanup was not observed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := bttesting.CheckCleanupOnCancel(tt.timeout, tt.probe)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %v, want containing %q", err, tt.want)
			}
		})
	}
}

func TestCheckCleanupOnCancelReportsUnreturnedProbe(t *testing.T) {
	release := make(chan struct{})
	err := bttesting.CheckCleanupOnCancel(10*time.Millisecond, func(_ context.Context, ready func(), cleaned func()) error {
		ready()
		cleaned()
		<-release
		return context.Canceled
	})
	close(release)

	if err == nil || !strings.Contains(err.Error(), "probe did not return") {
		t.Fatalf("error = %v, want probe did not return", err)
	}
}
