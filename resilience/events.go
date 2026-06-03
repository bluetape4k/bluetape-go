package resilience

import (
	"context"
	"time"
)

// EventKind classifies policy events. #21 will extend the event set and
// payloads without changing the policy composition model.
type EventKind string

const (
	// EventSuccess reports that a policy-protected operation succeeded.
	EventSuccess EventKind = "success"
	// EventRetry reports that a retry policy scheduled another attempt.
	EventRetry EventKind = "retry"
	// EventTimeout reports that a timeout policy's own deadline expired.
	EventTimeout EventKind = "timeout"
	// EventCircuitStateTransition reports that a circuit breaker changed state.
	EventCircuitStateTransition EventKind = "circuit_state_transition"
	// EventCircuitRejected reports that a circuit breaker rejected a call.
	EventCircuitRejected EventKind = "circuit_rejected"
	// EventBulkheadAccepted reports that a bulkhead admitted a call.
	EventBulkheadAccepted EventKind = "bulkhead_accepted"
	// EventBulkheadRejected reports that a bulkhead rejected a call.
	EventBulkheadRejected EventKind = "bulkhead_rejected"
)

// Event describes a policy decision or outcome.
type Event struct {
	PolicyName    string
	PolicyType    string
	Kind          EventKind
	Attempt       int
	Delay         time.Duration
	Err           error
	State         CircuitState
	PreviousState CircuitState
	InFlight      int
}

// EventHandler receives policy events synchronously.
type EventHandler func(context.Context, Event)

func emitEvent(ctx context.Context, handler EventHandler, event Event) {
	if handler != nil {
		handler(ctx, event)
	}
}
