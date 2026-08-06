package redisvalue

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestLocalStateHealthyLeaseAndTicketAdmission(t *testing.T) {
	state := newLocalState()
	lease, err := state.acquireHealthy(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if lease.generation != 0 || state.activeValue() != 1 {
		t.Fatalf("lease generation/active = %d/%d", lease.generation, state.activeValue())
	}
	ticket, ok := lease.issueTicket()
	if !ok || ticket.generation != lease.generation {
		t.Fatalf("ticket = %+v/%v", ticket, ok)
	}
	if err := ticket.consume(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := ticket.consume(context.Background()); !errors.Is(err, errTicketConsumed) {
		t.Fatalf("second consume = %v", err)
	}
	if disposition := state.classify(lease.generation); disposition != localCurrent {
		t.Fatalf("classification = %v", disposition)
	}
	lease.release()
	if state.activeValue() != 0 {
		t.Fatalf("active leases = %d", state.activeValue())
	}
}

func TestLocalStateAcquireWaitsDuringHealthyOriginRepair(t *testing.T) {
	state := newLocalState()
	repair, err := state.beginRepair(context.Background(), repairExplicit)
	if err != nil {
		t.Fatal(err)
	}
	result := make(chan error, 1)
	go func() {
		lease, acquireErr := state.acquireHealthy(context.Background())
		if acquireErr == nil {
			lease.release()
		}
		result <- acquireErr
	}()
	select {
	case err := <-result:
		t.Fatalf("acquireHealthy returned during repair: %v", err)
	case <-time.After(20 * time.Millisecond):
	}
	if state.activeValue() != 0 {
		t.Fatalf("repair waiter retained a lease: %d", state.activeValue())
	}
	if !state.finishRepair(repair, nil) {
		t.Fatal("healthy-origin repair did not restore healthy state")
	}
	if err := <-result; err != nil {
		t.Fatal(err)
	}
}

func TestLocalStateBlockedAndBlockedOriginRepairFailClosed(t *testing.T) {
	state := newLocalState()
	state.block()
	if _, err := state.acquireHealthy(context.Background()); !hasReason(err, ReasonLocalBlocked) {
		t.Fatalf("blocked acquire = %v", err)
	}
	repair, err := state.beginRepair(context.Background(), repairExplicit)
	if err != nil {
		t.Fatal(err)
	}
	if repair.origin != repairFromBlocked || state.phaseValue() != phaseRepairing {
		t.Fatalf("repair origin/phase = %v/%v", repair.origin, state.phaseValue())
	}
	if _, err := state.acquireHealthy(context.Background()); !hasReason(err, ReasonLocalBlocked) {
		t.Fatalf("blocked-origin repair acquire = %v", err)
	}
	if !state.finishRepair(repair, nil) {
		t.Fatal("explicit blocked-origin repair did not heal")
	}
}

func TestLocalStateCanceledWaitLeavesRepairUnchanged(t *testing.T) {
	state := newLocalState()
	repair, err := state.beginRepair(context.Background(), repairExplicit)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := state.acquireHealthy(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("acquireHealthy = %v", err)
	}
	if state.phaseValue() != phaseRepairing || state.repairEpochValue() != repair.epoch {
		t.Fatalf("canceled waiter changed repair: phase=%v epoch=%d", state.phaseValue(), state.repairEpochValue())
	}
	state.finishRepair(repair, nil)
}

func TestLocalStateTicketAdmissionRejectsGenerationChange(t *testing.T) {
	state := newLocalState()
	lease, err := state.acquireHealthy(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	state.block()
	if _, ok := lease.issueTicket(); ok {
		t.Fatal("stale lease issued a side-effect ticket")
	}
	if disposition := state.classify(lease.generation); disposition != localBlocked {
		t.Fatalf("classification = %v", disposition)
	}
	lease.release()
}

func TestLocalStateTicketCancellationDoesNotConsume(t *testing.T) {
	state := newLocalState()
	lease, err := state.acquireHealthy(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	ticket, ok := lease.issueTicket()
	lease.release()
	if !ok {
		t.Fatal("ticket admission failed")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := ticket.consume(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("consume(canceled) = %v", err)
	}
	if err := ticket.consume(context.Background()); err != nil {
		t.Fatalf("canceled ticket was consumed: %v", err)
	}
}

func TestLocalStateAdmittedSideEffectRunsAfterTransitionButCannotPublish(t *testing.T) {
	for _, operation := range []string{"loader", "redis-set", "redis-del"} {
		t.Run(operation, func(t *testing.T) {
			state := newLocalState()
			lease, err := state.acquireHealthy(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			ticket, ok := lease.issueTicket()
			if !ok {
				t.Fatal("side effect was not admitted")
			}
			lease.release()

			state.block()

			invocations := 0
			if err := ticket.consume(context.Background()); err != nil {
				t.Fatal(err)
			}
			invocations++

			publishes := 0
			if state.classify(ticket.generation) == localCurrent {
				publishes++
			}
			if invocations != 1 {
				t.Fatalf("side-effect invocations = %d", invocations)
			}
			if disposition := state.classify(ticket.generation); disposition != localBlocked {
				t.Fatalf("post-invocation classification = %v", disposition)
			}
			if publishes != 0 {
				t.Fatalf("post-transition publishes = %d", publishes)
			}
			if err := ticket.consume(context.Background()); !errors.Is(err, errTicketConsumed) {
				t.Fatalf("second consume = %v", err)
			}
		})
	}
}

func TestLocalStateRepairUsesOneContextBudgetAndBlocksOnTimeout(t *testing.T) {
	state := newLocalState()
	lease, err := state.acquireHealthy(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	started := time.Now()
	if _, err := state.beginRepair(ctx, repairExplicit); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("beginRepair = %v", err)
	}
	if elapsed := time.Since(started); elapsed > 200*time.Millisecond {
		t.Fatalf("repair exceeded caller budget: %s", elapsed)
	}
	if state.phaseValue() != phaseBlocked {
		t.Fatalf("timed-out repair phase = %v", state.phaseValue())
	}
	lease.release()
}

func TestLocalStateRepairEpochPreventsStaleHealing(t *testing.T) {
	state := newLocalState()
	first, err := state.beginRepair(context.Background(), repairExplicit)
	if err != nil {
		t.Fatal(err)
	}
	state.block()
	if state.finishRepair(first, nil) {
		t.Fatal("stale repair healed newer block")
	}
	if state.phaseValue() != phaseBlocked {
		t.Fatalf("phase = %v", state.phaseValue())
	}
	second, err := state.beginRepair(context.Background(), repairExplicit)
	if err != nil {
		t.Fatal(err)
	}
	if !state.finishRepair(second, nil) || state.phaseValue() != phaseHealthy {
		t.Fatalf("explicit repair did not heal: %v", state.phaseValue())
	}
}

func TestLocalStateMandatoryRepairFromBlockedPreservesBlock(t *testing.T) {
	state := newLocalState()
	state.block()
	repair, err := state.beginRepair(context.Background(), repairMandatory)
	if err != nil {
		t.Fatal(err)
	}
	if state.finishRepair(repair, nil) {
		t.Fatal("mandatory blocked-origin repair healed state")
	}
	if state.phaseValue() != phaseBlocked {
		t.Fatalf("phase = %v", state.phaseValue())
	}
}

func TestLocalStateFailedRepairBlocksHealthyOrigin(t *testing.T) {
	state := newLocalState()
	repair, err := state.beginRepair(context.Background(), repairExplicit)
	if err != nil {
		t.Fatal(err)
	}
	if state.finishRepair(repair, errors.New("local clear failed")) {
		t.Fatal("failed repair reported healthy")
	}
	if state.phaseValue() != phaseBlocked {
		t.Fatalf("phase = %v", state.phaseValue())
	}
}

func TestLocalStateMaintenanceLeaseWorksWhileBlockedAndDrainsBeforeRepair(t *testing.T) {
	state := newLocalState()
	state.block()
	maintenance, err := state.acquireMaintenance(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if _, err := state.beginRepair(ctx, repairExplicit); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("beginRepair = %v", err)
	}
	maintenance.release()
	if state.phaseValue() != phaseBlocked {
		t.Fatalf("phase = %v", state.phaseValue())
	}
}

func TestLocalStateBlockDoesNotWaitForOldLease(t *testing.T) {
	state := newLocalState()
	lease, err := state.acquireHealthy(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	go func() {
		state.block()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(100 * time.Millisecond):
		t.Fatal("block waited for old lease")
	}
	if state.phaseValue() != phaseBlocked {
		t.Fatalf("phase = %v", state.phaseValue())
	}
	lease.release()
}

func TestLocalStateClassificationDistinguishesHealthyGenerationChange(t *testing.T) {
	state := newLocalState()
	lease, err := state.acquireHealthy(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	repairDone := make(chan repairLease, 1)
	go func() {
		repair, repairErr := state.beginRepair(context.Background(), repairExplicit)
		if repairErr != nil {
			return
		}
		repairDone <- repair
	}()
	deadline := time.Now().Add(time.Second)
	for state.phaseValue() != phaseRepairing {
		if time.Now().After(deadline) {
			t.Fatal("repair did not enter repairing phase")
		}
	}
	lease.release()
	repair := <-repairDone
	if !state.finishRepair(repair, nil) {
		t.Fatal("repair did not finish")
	}
	if disposition := state.classify(lease.generation); disposition != localNewerGeneration {
		t.Fatalf("classification = %v", disposition)
	}
}

func TestHealthyLeaseAdmissionAllocatesNothing(t *testing.T) {
	state := newLocalState()
	allocs := testing.AllocsPerRun(1000, func() {
		lease, err := state.acquireHealthy(context.Background())
		if err != nil {
			panic(err)
		}
		lease.release()
	})
	if allocs != 0 {
		t.Fatalf("allocations = %f", allocs)
	}
}
