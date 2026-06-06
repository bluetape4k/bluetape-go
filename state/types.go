package state

import "context"

// Guard decides whether a transition may proceed.
//
// Guards receive the caller context, current state, and event. Returning an
// error rejects the transition. Because CanTransition can evaluate guards
// without mutating the machine, guards should be safe for inquiry calls.
type Guard[S comparable, E comparable] func(context.Context, S, E) error

// Transition defines one event path from a source state to a target state.
type Transition[S comparable, E comparable] struct {
	From  S
	Event E
	To    S
	Guard Guard[S, E]
}

// Result reports the state change performed by Transition.
type Result[S comparable, E comparable] struct {
	Previous S
	Event    E
	Current  S
}

// Option configures a Machine during construction.
type Option[S comparable, E comparable] func(*config[S, E])

type config[S comparable, E comparable] struct {
	finalStates map[S]struct{}
}

// WithFinalStates marks states that reject further transitions.
func WithFinalStates[S comparable, E comparable](states ...S) Option[S, E] {
	return func(cfg *config[S, E]) {
		if cfg.finalStates == nil {
			cfg.finalStates = make(map[S]struct{}, len(states))
		}
		for _, item := range states {
			cfg.finalStates[item] = struct{}{}
		}
	}
}
