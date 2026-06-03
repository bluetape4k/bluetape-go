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
)

// Event describes a policy decision or outcome.
type Event struct {
	PolicyName string
	PolicyType string
	Kind       EventKind
	Attempt    int
	Delay      time.Duration
	Err        error
}

// EventHandler receives policy events synchronously.
type EventHandler func(context.Context, Event)

func emitEvent(ctx context.Context, handler EventHandler, event Event) {
	if handler != nil {
		handler(ctx, event)
	}
}
