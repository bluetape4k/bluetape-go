package state

import (
	"errors"
	"fmt"
)

// Sentinel errors used with errors.Is.
var (
	ErrInvalidTransition    = errors.New("invalid transition")
	ErrGuardRejected        = errors.New("guard rejected transition")
	ErrFinalState           = errors.New("state is final")
	ErrConcurrentTransition = errors.New("concurrent transition conflict")
	ErrDuplicateTransition  = errors.New("duplicate transition")
	ErrUnknownInitialState  = errors.New("unknown initial state")
)

// TransitionError reports a transition-specific failure.
type TransitionError[S comparable, E comparable] struct {
	Kind  error
	From  S
	Event E
	To    S
	Cause error
}

func (e TransitionError[S, E]) Error() string {
	message := fmt.Sprintf("%v: from=%v event=%v to=%v", e.Kind, e.From, e.Event, e.To)
	if e.Cause != nil {
		return message + ": " + e.Cause.Error()
	}
	return message
}

// Unwrap returns the guard or context cause when one exists.
func (e TransitionError[S, E]) Unwrap() error {
	return e.Cause
}

// Is allows errors.Is checks against the package sentinel error.
func (e TransitionError[S, E]) Is(target error) bool {
	return target == e.Kind
}
