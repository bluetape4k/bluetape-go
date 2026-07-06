package bttesting_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	bttesting "github.com/bluetape4k/bluetape-go/testing"
	concurrencytest "github.com/bluetape4k/bluetape-go/testing/concurrency"
)

func TestCheckAwaitImmediateSuccess(t *testing.T) {
	result, err := bttesting.CheckAwait(
		context.Background(),
		50*time.Millisecond,
		time.Millisecond,
		func(context.Context) (string, error) {
			return "ready", nil
		},
		func(value string, err error) bttesting.AwaitStatus {
			if err == nil && value == "ready" {
				return bttesting.AwaitSuccess
			}
			return bttesting.AwaitContinue
		},
	)
	if err != nil {
		t.Fatalf("CheckAwait error = %v", err)
	}
	if result.Value != "ready" || result.Attempts != 1 || result.Err != nil {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestCheckAwaitEventualSuccess(t *testing.T) {
	var attempts atomic.Int32

	result, err := bttesting.CheckAwait(
		context.Background(),
		100*time.Millisecond,
		time.Millisecond,
		func(context.Context) (int, error) {
			return int(attempts.Add(1)), nil
		},
		func(value int, err error) bttesting.AwaitStatus {
			if err == nil && value >= 3 {
				return bttesting.AwaitSuccess
			}
			return bttesting.AwaitContinue
		},
	)
	if err != nil {
		t.Fatalf("CheckAwait error = %v", err)
	}
	if result.Value != 3 || result.Attempts != 3 || attempts.Load() != 3 {
		t.Fatalf("unexpected result: %+v attempts=%d", result, attempts.Load())
	}
}

func TestCheckAwaitValue(t *testing.T) {
	var value atomic.Int32

	result, err := bttesting.CheckAwaitValue(
		context.Background(),
		100*time.Millisecond,
		time.Millisecond,
		func(context.Context) (int32, error) {
			return value.Add(1), nil
		},
		int32(4),
	)
	if err != nil {
		t.Fatalf("CheckAwaitValue error = %v", err)
	}
	if result.Value != 4 || result.Attempts != 4 {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestCheckAwaitError(t *testing.T) {
	expected := errors.New("not visible yet")
	var attempts atomic.Int32

	result, err := bttesting.CheckAwaitError(
		context.Background(),
		100*time.Millisecond,
		time.Millisecond,
		func(context.Context) error {
			if attempts.Add(1) < 3 {
				return nil
			}
			return expected
		},
		expected,
	)
	if err != nil {
		t.Fatalf("CheckAwaitError error = %v", err)
	}
	if result.Err == nil || !errors.Is(result.Err, expected) || result.Attempts != 3 {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestCheckAwaitImmediateFailureDiagnostics(t *testing.T) {
	result, err := bttesting.CheckAwait(
		context.Background(),
		50*time.Millisecond,
		time.Millisecond,
		func(context.Context) (int, error) {
			return 42, errors.New("bad state")
		},
		func(int, error) bttesting.AwaitStatus {
			return bttesting.AwaitFailure
		},
	)
	if err == nil {
		t.Fatal("expected failure")
	}
	if result.Value != 42 || result.Attempts != 1 {
		t.Fatalf("unexpected result: %+v", result)
	}
	for _, want := range []string{"await failed", "last value 42", "bad state"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("error = %v, want containing %q", err, want)
		}
	}
}

func TestCheckAwaitTimeoutReportsFinalObservation(t *testing.T) {
	var attempts atomic.Int32

	result, err := bttesting.CheckAwait(
		context.Background(),
		10*time.Millisecond,
		time.Millisecond,
		func(context.Context) (int32, error) {
			return attempts.Add(1), errors.New("not ready")
		},
		func(int32, error) bttesting.AwaitStatus {
			return bttesting.AwaitContinue
		},
	)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected deadline exceeded, got %v", err)
	}
	if result.Attempts == 0 || result.Value == 0 || result.Err == nil {
		t.Fatalf("expected final observation in result, got %+v", result)
	}
	for _, want := range []string{"await timed out", "last value", "last error"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("error = %v, want containing %q", err, want)
		}
	}
}

func TestCheckAwaitContextCancellationIsNotRetried(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	result, err := bttesting.CheckAwait(
		ctx,
		50*time.Millisecond,
		time.Millisecond,
		func(context.Context) (int, error) {
			t.Fatal("probe should not run after caller cancellation")
			return 0, nil
		},
		func(int, error) bttesting.AwaitStatus {
			return bttesting.AwaitContinue
		},
	)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
	if result.Attempts != 0 {
		t.Fatalf("expected no attempts after caller cancellation, got %+v", result)
	}
}

func TestCheckAwaitProbeCancellationIsNotRetried(t *testing.T) {
	var attempts atomic.Int32

	result, err := bttesting.CheckAwait(
		context.Background(),
		50*time.Millisecond,
		time.Millisecond,
		func(context.Context) (int32, error) {
			return attempts.Add(1), context.Canceled
		},
		func(int32, error) bttesting.AwaitStatus {
			return bttesting.AwaitContinue
		},
	)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
	if result.Attempts != 1 || attempts.Load() != 1 {
		t.Fatalf("expected one attempt before cancellation, got result=%+v attempts=%d", result, attempts.Load())
	}
}

func TestCheckAwaitCancellationUsesAsyncJobTester(t *testing.T) {
	tester := concurrencytest.NewAsyncJobTester(concurrencytest.Options{
		Workers:       2,
		RoundsPerTask: 32,
		Timeout:       time.Second,
	})

	tester.RunT(t, func(context.Context) error {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		result, err := bttesting.CheckAwait(
			ctx,
			50*time.Millisecond,
			time.Millisecond,
			func(context.Context) (int, error) {
				return 0, errors.New("probe should not run after caller cancellation")
			},
			func(int, error) bttesting.AwaitStatus {
				return bttesting.AwaitContinue
			},
		)
		if !errors.Is(err, context.Canceled) {
			return fmt.Errorf("CheckAwait error = %w, want context.Canceled", err)
		}
		if result.Attempts != 0 {
			return errors.New("cancelled await should not retry")
		}
		return nil
	})
}

func TestCheckAwaitDiagnostics(t *testing.T) {
	tests := []struct {
		name     string
		timeout  time.Duration
		interval time.Duration
		probe    bttesting.AwaitProbe[int]
		check    bttesting.AwaitCheck[int]
		want     string
	}{
		{
			name:     "invalid timeout",
			timeout:  0,
			interval: time.Millisecond,
			probe:    func(context.Context) (int, error) { return 0, nil },
			check:    func(int, error) bttesting.AwaitStatus { return bttesting.AwaitSuccess },
			want:     "timeout must be positive",
		},
		{
			name:     "invalid interval",
			timeout:  time.Millisecond,
			interval: 0,
			probe:    func(context.Context) (int, error) { return 0, nil },
			check:    func(int, error) bttesting.AwaitStatus { return bttesting.AwaitSuccess },
			want:     "interval must be positive",
		},
		{
			name:     "nil probe",
			timeout:  time.Millisecond,
			interval: time.Millisecond,
			probe:    nil,
			check:    func(int, error) bttesting.AwaitStatus { return bttesting.AwaitSuccess },
			want:     "probe must not be nil",
		},
		{
			name:     "nil check",
			timeout:  time.Millisecond,
			interval: time.Millisecond,
			probe:    func(context.Context) (int, error) { return 0, nil },
			check:    nil,
			want:     "check must not be nil",
		},
		{
			name:     "invalid status",
			timeout:  time.Millisecond,
			interval: time.Millisecond,
			probe:    func(context.Context) (int, error) { return 0, nil },
			check:    func(int, error) bttesting.AwaitStatus { return bttesting.AwaitStatus(99) },
			want:     "unknown await status",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := bttesting.CheckAwait(context.Background(), tt.timeout, tt.interval, tt.probe, tt.check)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %v, want containing %q", err, tt.want)
			}
		})
	}
}
