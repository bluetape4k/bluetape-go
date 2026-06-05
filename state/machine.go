package state

import (
	"context"
	"errors"
	"sync"
)

type transitionKey[S comparable, E comparable] struct {
	from  S
	event E
}

type transitionTarget[S comparable, E comparable] struct {
	to    S
	guard Guard[S, E]
}

// Machine is a concurrency-safe finite state machine.
type Machine[S comparable, E comparable] struct {
	mu          sync.RWMutex
	current     S
	transitions map[transitionKey[S, E]]transitionTarget[S, E]
	allowed     map[S][]E
	finalStates map[S]struct{}
}

// NewMachine creates a finite state machine from explicit transitions.
func NewMachine[S comparable, E comparable](
	initial S,
	transitions []Transition[S, E],
	options ...Option[S, E],
) (*Machine[S, E], error) {
	cfg := config[S, E]{finalStates: make(map[S]struct{})}
	for _, option := range options {
		if option != nil {
			option(&cfg)
		}
	}

	registered := make(map[transitionKey[S, E]]transitionTarget[S, E], len(transitions))
	allowed := make(map[S][]E)
	knownStates := make(map[S]struct{})

	for state := range cfg.finalStates {
		knownStates[state] = struct{}{}
	}
	for _, transition := range transitions {
		key := transitionKey[S, E]{from: transition.From, event: transition.Event}
		if _, exists := registered[key]; exists {
			return nil, TransitionError[S, E]{
				Kind:  ErrDuplicateTransition,
				From:  transition.From,
				Event: transition.Event,
				To:    transition.To,
			}
		}
		registered[key] = transitionTarget[S, E]{
			to:    transition.To,
			guard: transition.Guard,
		}
		allowed[transition.From] = append(allowed[transition.From], transition.Event)
		knownStates[transition.From] = struct{}{}
		knownStates[transition.To] = struct{}{}
	}

	if _, exists := knownStates[initial]; !exists {
		return nil, TransitionError[S, E]{
			Kind: ErrUnknownInitialState,
			From: initial,
		}
	}

	return &Machine[S, E]{
		current:     initial,
		transitions: registered,
		allowed:     allowed,
		finalStates: cfg.finalStates,
	}, nil
}

// State returns the current state.
func (m *Machine[S, E]) State() S {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.current
}

// Transition applies one event transition.
func (m *Machine[S, E]) Transition(ctx context.Context, event E) (Result[S, E], error) {
	var zero Result[S, E]
	ctx = normalizeContext(ctx)
	if err := ctx.Err(); err != nil {
		return zero, err
	}

	from, target, err := m.lookup(event)
	if err != nil {
		return zero, err
	}

	if target.guard != nil {
		if err := target.guard(ctx, from, event); err != nil {
			return zero, TransitionError[S, E]{
				Kind:  ErrGuardRejected,
				From:  from,
				Event: event,
				To:    target.to,
				Cause: err,
			}
		}
	}
	if err := ctx.Err(); err != nil {
		return zero, err
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	if m.current != from {
		return zero, TransitionError[S, E]{
			Kind:  ErrConcurrentTransition,
			From:  from,
			Event: event,
			To:    target.to,
		}
	}
	m.current = target.to
	return Result[S, E]{
		Previous: from,
		Event:    event,
		Current:  target.to,
	}, nil
}

// CanTransition reports whether the current state can transition with event.
//
// It may execute the transition guard, but it never mutates the machine.
func (m *Machine[S, E]) CanTransition(ctx context.Context, event E) (bool, error) {
	ctx = normalizeContext(ctx)
	if err := ctx.Err(); err != nil {
		return false, err
	}

	from, target, err := m.lookup(event)
	if err != nil {
		if errors.Is(err, ErrInvalidTransition) || errors.Is(err, ErrFinalState) {
			return false, nil
		}
		return false, err
	}

	if target.guard == nil {
		return true, nil
	}
	if err := target.guard(ctx, from, event); err != nil {
		return false, TransitionError[S, E]{
			Kind:  ErrGuardRejected,
			From:  from,
			Event: event,
			To:    target.to,
			Cause: err,
		}
	}
	if err := ctx.Err(); err != nil {
		return false, err
	}
	return true, nil
}

// AllowedEvents returns registered events for the current state.
//
// It does not evaluate guards; use CanTransition to evaluate a specific guard.
func (m *Machine[S, E]) AllowedEvents() []E {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if _, final := m.finalStates[m.current]; final {
		return []E{}
	}
	events := m.allowed[m.current]
	return append([]E(nil), events...)
}

func (m *Machine[S, E]) lookup(event E) (S, transitionTarget[S, E], error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	from := m.current
	if _, final := m.finalStates[from]; final {
		return from, transitionTarget[S, E]{}, TransitionError[S, E]{
			Kind:  ErrFinalState,
			From:  from,
			Event: event,
		}
	}

	target, exists := m.transitions[transitionKey[S, E]{from: from, event: event}]
	if !exists {
		return from, transitionTarget[S, E]{}, TransitionError[S, E]{
			Kind:  ErrInvalidTransition,
			From:  from,
			Event: event,
		}
	}
	return from, target, nil
}

func normalizeContext(ctx context.Context) context.Context {
	if ctx == nil {
		return context.Background()
	}
	return ctx
}
