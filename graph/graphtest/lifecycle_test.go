package graphtest

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestRunCancellationJoinsBeforeCleanupAndClose(t *testing.T) {
	var mu sync.Mutex
	var events []string
	record := func(value string) {
		mu.Lock()
		defer mu.Unlock()
		events = append(events, value)
	}
	adapter := completeAdapter()
	adapter.BlockUntilCanceled = func(ctx context.Context, _ Fixture, started Started) error {
		record("started")
		started()
		<-ctx.Done()
		record("joined")
		return ctx.Err()
	}
	adapter.CleanupFixture = func(context.Context, Fixture) error {
		record("cleanup")
		return nil
	}
	adapter.Close = func(context.Context) error {
		record("close")
		return nil
	}
	fixture, _ := newFixture()
	config := DefaultConfig()
	config.CaseTimeout = 50 * time.Millisecond
	if err := exerciseCancellation(context.Background(), adapter, fixture, config); !errors.Is(err, context.Canceled) {
		t.Fatalf("exerciseCancellation() error = %v, want context.Canceled", err)
	}
	if err := cleanupAndClose(context.Background(), adapter, fixture, config); err != nil {
		t.Fatalf("cleanupAndClose() error = %v", err)
	}
	if want := []string{"started", "joined", "cleanup", "close"}; !reflect.DeepEqual(events, want) {
		t.Fatalf("events = %v, want %v", events, want)
	}
}

func TestCleanupIgnoresParentCancellationButKeepsDeadline(t *testing.T) {
	parent, cancel := context.WithCancel(context.Background())
	cancel()
	adapter := completeAdapter()
	adapter.CleanupFixture = func(ctx context.Context, _ Fixture) error {
		if ctx.Err() != nil {
			return errors.New("cleanup inherited cancellation")
		}
		if _, ok := ctx.Deadline(); !ok {
			return errors.New("cleanup missing deadline")
		}
		return nil
	}
	fixture, _ := newFixture()
	if err := cleanupAndClose(parent, adapter, fixture, DefaultConfig()); err != nil {
		t.Fatal(err)
	}
}

func TestExerciseCancellationRejectsInvalidHandshake(t *testing.T) {
	fixture, _ := newFixture()
	config := DefaultConfig()
	config.CaseTimeout = 50 * time.Millisecond
	config.CloseTimeout = 20 * time.Millisecond

	t.Run("returned-before-start", func(t *testing.T) {
		adapter := completeAdapter()
		adapter.BlockUntilCanceled = func(context.Context, Fixture, Started) error { return nil }
		if err := exerciseCancellation(context.Background(), adapter, fixture, config); !errors.Is(err, errCancellationReturnedBeforeStart) {
			t.Fatalf("exerciseCancellation() error = %v", err)
		}
	})

	t.Run("missing-start", func(t *testing.T) {
		adapter := completeAdapter()
		adapter.BlockUntilCanceled = func(ctx context.Context, _ Fixture, _ Started) error {
			<-ctx.Done()
			return ctx.Err()
		}
		if err := exerciseCancellation(context.Background(), adapter, fixture, config); !errors.Is(err, errCancellationStartTimeout) {
			t.Fatalf("exerciseCancellation() error = %v", err)
		}
	})

	t.Run("duplicate-start", func(t *testing.T) {
		adapter := completeAdapter()
		adapter.BlockUntilCanceled = func(ctx context.Context, _ Fixture, started Started) error {
			started()
			started()
			<-ctx.Done()
			return ctx.Err()
		}
		if err := exerciseCancellation(context.Background(), adapter, fixture, config); !errors.Is(err, errCancellationDuplicateStart) {
			t.Fatalf("exerciseCancellation() error = %v", err)
		}
	})
}

func TestExerciseCancellationHonorsPreCanceledParentAndDeadline(t *testing.T) {
	fixture, _ := newFixture()
	config := DefaultConfig()
	config.CaseTimeout = 50 * time.Millisecond
	adapter := completeAdapter()
	var calls atomic.Int32
	adapter.BlockUntilCanceled = func(ctx context.Context, _ Fixture, started Started) error {
		calls.Add(1)
		if _, ok := ctx.Deadline(); !ok {
			return errors.New("missing deadline")
		}
		started()
		<-ctx.Done()
		return ctx.Err()
	}
	parent, cancel := context.WithCancel(context.Background())
	cancel()
	if err := exerciseCancellation(parent, adapter, fixture, config); !errors.Is(err, context.Canceled) {
		t.Fatalf("exerciseCancellation() error = %v", err)
	}
	if got := calls.Load(); got != 0 {
		t.Fatalf("callback calls = %d, want 0", got)
	}
	if err := exerciseCancellation(context.Background(), adapter, fixture, config); !errors.Is(err, context.Canceled) {
		t.Fatalf("exerciseCancellation() error = %v", err)
	}
}

func TestCallUsesContextErrorPrecedenceAndRedactsPanic(t *testing.T) {
	config := DefaultConfig()
	result := call(context.Background(), 10*time.Millisecond, func(ctx context.Context) (string, error) {
		<-ctx.Done()
		return "late-success", nil
	})
	if !errors.Is(callbackError("read", result), context.DeadlineExceeded) {
		t.Fatalf("callbackError() = %v", callbackError("read", result))
	}
	status, categoryName, timedOut := callbackStatus(result)
	if status != "error" || categoryName != "timeout" || !timedOut {
		t.Fatalf("callbackStatus() = %q, %q, %v", status, categoryName, timedOut)
	}

	panicResult := call(context.Background(), config.CaseTimeout, func(context.Context) (struct{}, error) {
		panic("credential-marker")
	})
	panicErr := callbackError("read", panicResult)
	if panicErr == nil || strings.Contains(panicErr.Error(), "credential-marker") {
		t.Fatalf("callbackError() = %v", panicErr)
	}
}

func TestCleanupAndCloseRecoversPanicsAndContinues(t *testing.T) {
	adapter := completeAdapter()
	fixture, _ := newFixture()
	var closed atomic.Bool
	adapter.CleanupFixture = func(context.Context, Fixture) error { panic("cleanup-secret") }
	adapter.Close = func(context.Context) error {
		closed.Store(true)
		panic("close-secret")
	}
	err := cleanupAndClose(context.Background(), adapter, fixture, DefaultConfig())
	if err == nil {
		t.Fatal("cleanupAndClose() error = nil")
	}
	if !closed.Load() {
		t.Fatal("close was not called after cleanup panic")
	}
	if strings.Contains(err.Error(), "cleanup-secret") || strings.Contains(err.Error(), "close-secret") {
		t.Fatalf("cleanupAndClose() exposed panic value: %v", err)
	}
}

func TestNonCooperativeCallbackFailsProcess(t *testing.T) {
	if os.Getenv("BTGC_NON_COOPERATIVE_HELPER") == "1" {
		_ = call(context.Background(), 10*time.Millisecond, func(context.Context) (struct{}, error) {
			select {}
		})
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestNonCooperativeCallbackFailsProcess$", "-test.timeout=200ms")
	command.Env = append(os.Environ(), "BTGC_NON_COOPERATIVE_HELPER=1")
	output, err := command.CombinedOutput()
	if err == nil {
		t.Fatal("non-cooperative callback exited successfully")
	}
	if !strings.Contains(string(output), "test timed out") {
		t.Fatalf("subprocess output = %s", output)
	}
}
