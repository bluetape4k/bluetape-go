package resilience

import (
	"context"
	"errors"
	"time"
)

// Policy type constants provide stable labels for Event.PolicyType.
const (
	PolicyTypeRetry          = "retry"
	PolicyTypeTimeout        = "timeout"
	PolicyTypeCircuitBreaker = "circuit_breaker"
	PolicyTypeBulkhead       = "bulkhead"
)

// EventKind string 공개 타입이며 취소, deadline, retry, timeout, circuit breaker 상태를 보존한다.
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
	// EventFailure reports that a policy-protected operation failed without a
	// policy-specific event kind.
	EventFailure EventKind = "failure"
)

// EventCategory string 공개 타입이며 취소, deadline, retry, timeout, circuit breaker 상태를 보존한다.
type EventCategory string

const (
	// EventCategorySuccess marks successful protected operations.
	EventCategorySuccess EventCategory = "success"
	// EventCategoryRetry marks retry scheduling decisions.
	EventCategoryRetry EventCategory = "retry"
	// EventCategoryAdmission marks bulkhead admission decisions.
	EventCategoryAdmission EventCategory = "admission"
	// EventCategoryTimeout marks timeout decisions.
	EventCategoryTimeout EventCategory = "timeout"
	// EventCategoryRejection marks circuit breaker or bulkhead rejections.
	EventCategoryRejection EventCategory = "rejection"
	// EventCategoryTransition marks circuit breaker state transitions.
	EventCategoryTransition EventCategory = "transition"
	// EventCategoryFailure marks failures without a more specific category.
	EventCategoryFailure EventCategory = "failure"
)

// ErrorCategory string 공개 타입이며 취소, deadline, retry, timeout, circuit breaker 상태를 보존한다.
type ErrorCategory string

const (
	// ErrorCategoryNone means the event has no associated error.
	ErrorCategoryNone ErrorCategory = ""
	// ErrorCategoryTimeout identifies policy-owned timeout errors.
	ErrorCategoryTimeout ErrorCategory = "timeout"
	// ErrorCategoryCircuitOpen identifies circuit breaker rejection errors.
	ErrorCategoryCircuitOpen ErrorCategory = "circuit_open"
	// ErrorCategoryBulkheadRejected identifies bulkhead rejection errors.
	ErrorCategoryBulkheadRejected ErrorCategory = "bulkhead_rejected"
	// ErrorCategoryRetryExhausted identifies retry exhaustion errors.
	ErrorCategoryRetryExhausted ErrorCategory = "retry_exhausted"
	// ErrorCategoryContextCanceled identifies context cancellation errors.
	ErrorCategoryContextCanceled ErrorCategory = "context_canceled"
	// ErrorCategoryContextDeadline identifies context deadline errors not owned
	// by a timeout policy.
	ErrorCategoryContextDeadline ErrorCategory = "context_deadline"
	// ErrorCategoryFailure identifies other operation or policy failures.
	ErrorCategoryFailure ErrorCategory = "failure"
)

// Event struct 공개 타입이며 취소, deadline, retry, timeout, circuit breaker 상태를 보존한다.
type Event struct {
	PolicyName    string
	PolicyType    string
	Kind          EventKind
	Category      EventCategory
	Attempt       int
	Delay         time.Duration
	Timeout       time.Duration
	Err           error
	ErrorCategory ErrorCategory
	State         CircuitState
	PreviousState CircuitState
	InFlight      int
}

// EventHandler func 공개 타입이며 취소, deadline, retry, timeout, circuit breaker 상태를 보존한다.
type EventHandler func(context.Context, Event)

func emitEvent(ctx context.Context, handler EventHandler, event Event) {
	if handler != nil {
		handler(ctx, event)
	}
}

func categorizeError(err error) ErrorCategory {
	if err == nil {
		return ErrorCategoryNone
	}
	if errors.Is(err, ErrRetryExhausted) {
		return ErrorCategoryRetryExhausted
	}
	if errors.Is(err, ErrTimeout) {
		return ErrorCategoryTimeout
	}
	if errors.Is(err, ErrCircuitOpen) {
		return ErrorCategoryCircuitOpen
	}
	if errors.Is(err, ErrBulkheadRejected) {
		return ErrorCategoryBulkheadRejected
	}
	if errors.Is(err, context.Canceled) {
		return ErrorCategoryContextCanceled
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return ErrorCategoryContextDeadline
	}
	return ErrorCategoryFailure
}
