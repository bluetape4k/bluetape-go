package state

import (
	"context"
	"errors"
	"testing"
)

type orderState string
type orderEvent string

const (
	stateCreated   orderState = "created"
	statePaid      orderState = "paid"
	stateShipped   orderState = "shipped"
	stateCancelled orderState = "cancelled"

	eventPay    orderEvent = "pay"
	eventShip   orderEvent = "ship"
	eventCancel orderEvent = "cancel"
)

func newOrderMachine(t *testing.T, transitions []Transition[orderState, orderEvent]) *Machine[orderState, orderEvent] {
	t.Helper()
	machine, err := NewMachine(
		stateCreated,
		transitions,
		WithFinalStates[orderState, orderEvent](stateShipped, stateCancelled),
	)
	if err != nil {
		t.Fatalf("new machine: %v", err)
	}
	return machine
}

func TestTransitionAppliesValidEvent(t *testing.T) {
	machine := newOrderMachine(t, []Transition[orderState, orderEvent]{
		{From: stateCreated, Event: eventPay, To: statePaid},
	})

	result, err := machine.Transition(context.Background(), eventPay)
	if err != nil {
		t.Fatalf("transition: %v", err)
	}
	if result.Previous != stateCreated || result.Event != eventPay || result.Current != statePaid {
		t.Fatalf("unexpected result: %+v", result)
	}
	if got := machine.State(); got != statePaid {
		t.Fatalf("state = %q, want %q", got, statePaid)
	}
}

func TestTransitionRejectsInvalidEvent(t *testing.T) {
	machine := newOrderMachine(t, []Transition[orderState, orderEvent]{
		{From: stateCreated, Event: eventPay, To: statePaid},
	})

	if _, err := machine.Transition(context.Background(), eventShip); !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("expected invalid transition, got %v", err)
	}
	if got := machine.State(); got != stateCreated {
		t.Fatalf("state changed to %q", got)
	}
}

func TestTransitionRunsGuards(t *testing.T) {
	guardErr := errors.New("payment rejected")
	machine := newOrderMachine(t, []Transition[orderState, orderEvent]{
		{
			From:  stateCreated,
			Event: eventPay,
			To:    statePaid,
			Guard: func(context.Context, orderState, orderEvent) error {
				return guardErr
			},
		},
	})

	if _, err := machine.Transition(context.Background(), eventPay); !errors.Is(err, ErrGuardRejected) || !errors.Is(err, guardErr) {
		t.Fatalf("expected guard rejection with cause, got %v", err)
	}
	if got := machine.State(); got != stateCreated {
		t.Fatalf("state changed to %q", got)
	}
}

func TestTransitionRejectsFinalState(t *testing.T) {
	machine := newOrderMachine(t, []Transition[orderState, orderEvent]{
		{From: stateCreated, Event: eventCancel, To: stateCancelled},
	})
	if _, err := machine.Transition(context.Background(), eventCancel); err != nil {
		t.Fatalf("cancel: %v", err)
	}

	if _, err := machine.Transition(context.Background(), eventPay); !errors.Is(err, ErrFinalState) {
		t.Fatalf("expected final state error, got %v", err)
	}
}

func TestNewMachineRejectsDuplicateTransitions(t *testing.T) {
	_, err := NewMachine(stateCreated, []Transition[orderState, orderEvent]{
		{From: stateCreated, Event: eventPay, To: statePaid},
		{From: stateCreated, Event: eventPay, To: stateCancelled},
	})
	if !errors.Is(err, ErrDuplicateTransition) {
		t.Fatalf("expected duplicate transition, got %v", err)
	}
}

func TestNewMachineRejectsUnknownInitialState(t *testing.T) {
	_, err := NewMachine(stateCreated, []Transition[orderState, orderEvent]{
		{From: statePaid, Event: eventShip, To: stateShipped},
	})
	if !errors.Is(err, ErrUnknownInitialState) {
		t.Fatalf("expected unknown initial state, got %v", err)
	}
}

func TestCanTransitionEvaluatesGuardWithoutMutatingState(t *testing.T) {
	guardErr := errors.New("not ready")
	calls := 0
	machine := newOrderMachine(t, []Transition[orderState, orderEvent]{
		{
			From:  stateCreated,
			Event: eventPay,
			To:    statePaid,
			Guard: func(context.Context, orderState, orderEvent) error {
				calls++
				return guardErr
			},
		},
	})

	ok, err := machine.CanTransition(context.Background(), eventPay)
	if ok {
		t.Fatalf("can transition should be false")
	}
	if !errors.Is(err, ErrGuardRejected) || !errors.Is(err, guardErr) {
		t.Fatalf("expected guard rejection with cause, got %v", err)
	}
	if calls != 1 {
		t.Fatalf("guard calls = %d, want 1", calls)
	}
	if got := machine.State(); got != stateCreated {
		t.Fatalf("state changed to %q", got)
	}
}

func TestAllowedEventsReturnsRegisteredEventsWithoutEvaluatingGuards(t *testing.T) {
	calls := 0
	machine := newOrderMachine(t, []Transition[orderState, orderEvent]{
		{
			From:  stateCreated,
			Event: eventPay,
			To:    statePaid,
			Guard: func(context.Context, orderState, orderEvent) error {
				calls++
				return errors.New("not ready")
			},
		},
		{From: stateCreated, Event: eventCancel, To: stateCancelled},
	})

	events := machine.AllowedEvents()
	if len(events) != 2 || events[0] != eventPay || events[1] != eventCancel {
		t.Fatalf("events = %v", events)
	}
	if calls != 0 {
		t.Fatalf("allowed events evaluated guard %d times", calls)
	}
}

func TestAllowedEventsReturnsEmptyForFinalState(t *testing.T) {
	machine := newOrderMachine(t, []Transition[orderState, orderEvent]{
		{From: stateCreated, Event: eventCancel, To: stateCancelled},
		{From: stateCancelled, Event: eventPay, To: statePaid},
	})
	if _, err := machine.Transition(context.Background(), eventCancel); err != nil {
		t.Fatalf("cancel: %v", err)
	}

	events := machine.AllowedEvents()
	if len(events) != 0 {
		t.Fatalf("events from final state = %v", events)
	}
}

func TestCanTransitionReturnsFalseForInvalidAndFinalStates(t *testing.T) {
	machine := newOrderMachine(t, []Transition[orderState, orderEvent]{
		{From: stateCreated, Event: eventCancel, To: stateCancelled},
	})

	ok, err := machine.CanTransition(context.Background(), eventPay)
	if err != nil {
		t.Fatalf("can transition invalid: %v", err)
	}
	if ok {
		t.Fatalf("invalid transition should return false")
	}

	if _, err := machine.Transition(context.Background(), eventCancel); err != nil {
		t.Fatalf("cancel: %v", err)
	}
	ok, err = machine.CanTransition(context.Background(), eventPay)
	if err != nil {
		t.Fatalf("can transition final: %v", err)
	}
	if ok {
		t.Fatalf("final state should return false")
	}
}

func TestNilContextIsNormalized(t *testing.T) {
	machine := newOrderMachine(t, []Transition[orderState, orderEvent]{
		{From: stateCreated, Event: eventPay, To: statePaid},
	})

	if _, err := machine.Transition(nilContext(), eventPay); err != nil {
		t.Fatalf("nil context transition: %v", err)
	}
	if got := machine.State(); got != statePaid {
		t.Fatalf("state = %q, want %q", got, statePaid)
	}
}

func nilContext() context.Context {
	return nil
}

func TestTransitionErrorMatchesSentinelAndCause(t *testing.T) {
	guardErr := errors.New("guard failed")
	err := TransitionError[orderState, orderEvent]{
		Kind:  ErrGuardRejected,
		From:  stateCreated,
		Event: eventPay,
		To:    statePaid,
		Cause: guardErr,
	}
	if !errors.Is(err, ErrGuardRejected) {
		t.Fatalf("expected sentinel match")
	}
	if !errors.Is(err, guardErr) {
		t.Fatalf("expected guard cause match")
	}
}

func TestTransitionChecksCancellationBeforeCommit(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	machine := newOrderMachine(t, []Transition[orderState, orderEvent]{
		{
			From:  stateCreated,
			Event: eventPay,
			To:    statePaid,
			Guard: func(context.Context, orderState, orderEvent) error {
				cancel()
				return nil
			},
		},
	})

	if _, err := machine.Transition(ctx, eventPay); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context cancellation, got %v", err)
	}
	if got := machine.State(); got != stateCreated {
		t.Fatalf("state changed to %q", got)
	}
}

func TestGuardCanInspectStateWithoutDeadlock(t *testing.T) {
	var machine *Machine[orderState, orderEvent]
	machine, err := NewMachine(stateCreated, []Transition[orderState, orderEvent]{
		{
			From:  stateCreated,
			Event: eventPay,
			To:    statePaid,
			Guard: func(context.Context, orderState, orderEvent) error {
				if got := machine.State(); got != stateCreated {
					t.Fatalf("state in guard = %q, want %q", got, stateCreated)
				}
				return nil
			},
		},
	})
	if err != nil {
		t.Fatalf("new machine: %v", err)
	}

	if _, err := machine.Transition(context.Background(), eventPay); err != nil {
		t.Fatalf("transition: %v", err)
	}
}
