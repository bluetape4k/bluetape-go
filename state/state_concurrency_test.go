package state

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	concurrencytest "github.com/bluetape4k/bluetape-go/testing/concurrency"
)

func TestConcurrentGuardedTransitionsCommitOnce(t *testing.T) {
	const workers = 8
	var inGuard atomic.Int32
	var successes atomic.Int32
	var conflicts atomic.Int32

	machine := newOrderMachine(t, []Transition[orderState, orderEvent]{
		{
			From:  stateCreated,
			Event: eventPay,
			To:    statePaid,
			Guard: func(ctx context.Context, _ orderState, _ orderEvent) error {
				inGuard.Add(1)
				for inGuard.Load() < workers {
					select {
					case <-ctx.Done():
						return ctx.Err()
					default:
						time.Sleep(time.Millisecond)
					}
				}
				return nil
			},
		},
	})

	tasks := make([]concurrencytest.Task, workers)
	for i := range tasks {
		tasks[i] = func(ctx context.Context) error {
			_, err := machine.Transition(ctx, eventPay)
			switch {
			case err == nil:
				successes.Add(1)
				return nil
			case errors.Is(err, ErrConcurrentTransition):
				conflicts.Add(1)
				return nil
			default:
				return fmt.Errorf("unexpected transition error: %w", err)
			}
		}
	}

	tester := concurrencytest.NewGoroutineStressTester(concurrencytest.Options{
		Workers: workers,
		Timeout: 5 * time.Second,
	})
	report, err := tester.Run(context.Background(), tasks...)
	if err != nil {
		t.Fatalf("stress run failed: report=%+v err=%v", report, err)
	}
	if successes.Load() != 1 {
		t.Fatalf("successes = %d, want 1", successes.Load())
	}
	if conflicts.Load() != workers-1 {
		t.Fatalf("conflicts = %d, want %d", conflicts.Load(), workers-1)
	}
	if got := machine.State(); got != statePaid {
		t.Fatalf("state = %q, want %q", got, statePaid)
	}
}

func TestGuardCancellationUsesAsyncJobTester(t *testing.T) {
	machine := newOrderMachine(t, []Transition[orderState, orderEvent]{
		{
			From:  stateCreated,
			Event: eventPay,
			To:    statePaid,
			Guard: func(ctx context.Context, _ orderState, _ orderEvent) error {
				<-ctx.Done()
				return ctx.Err()
			},
		},
	})

	tester := concurrencytest.NewAsyncJobTester(concurrencytest.Options{
		Workers: 1,
		Timeout: 20 * time.Millisecond,
	})
	report, err := tester.Run(context.Background(), func(ctx context.Context) error {
		_, err := machine.Transition(ctx, eventPay)
		return err
	})
	if err == nil {
		t.Fatalf("expected cancellation error, report=%+v", report)
	}
	if !errors.Is(err, ErrGuardRejected) || !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected guard rejection with deadline cause, got %v", err)
	}
	if got := machine.State(); got != stateCreated {
		t.Fatalf("state changed to %q", got)
	}
}
