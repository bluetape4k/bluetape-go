package resilience_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/bluetape4k/bluetape-go/resilience"
)

type eventRecorder struct {
	mu     sync.Mutex
	events []resilience.Event
}

func (r *eventRecorder) onEvent(_ context.Context, event resilience.Event) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events = append(r.events, event)
}

func (r *eventRecorder) snapshot() []resilience.Event {
	r.mu.Lock()
	defer r.mu.Unlock()

	events := make([]resilience.Event, len(r.events))
	copy(events, r.events)
	return events
}

func TestRetryEventsExposeOrderingAndPayload(t *testing.T) {
	operationErr := errors.New("temporary")
	recorder := &eventRecorder{}
	retry, err := resilience.NewRetry[string](resilience.RetryOptions{
		Name:        "catalog",
		MaxAttempts: 3,
		Backoff:     resilience.ConstantBackoff(25 * time.Millisecond),
		Sleeper:     &fakeSleeper{},
		OnEvent:     recorder.onEvent,
	})
	if err != nil {
		t.Fatalf("NewRetry failed: %v", err)
	}

	attempts := 0
	got, err := resilience.Run(context.Background(), func(context.Context) (string, error) {
		attempts++
		if attempts == 1 {
			return "", operationErr
		}
		return "ok", nil
	}, retry)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	if got != "ok" {
		t.Fatalf("got %q, want ok", got)
	}

	events := recorder.snapshot()
	if len(events) != 2 {
		t.Fatalf("events = %#v, want retry and success", events)
	}
	assertEvent(t, events[0], eventWant{
		policyName:    "catalog",
		policyType:    resilience.PolicyTypeRetry,
		kind:          resilience.EventRetry,
		category:      resilience.EventCategoryRetry,
		attempt:       1,
		delay:         25 * time.Millisecond,
		err:           operationErr,
		errorCategory: resilience.ErrorCategoryFailure,
	})
	assertEvent(t, events[1], eventWant{
		policyName: "catalog",
		policyType: resilience.PolicyTypeRetry,
		kind:       resilience.EventSuccess,
		category:   resilience.EventCategorySuccess,
		attempt:    2,
	})
}

func TestRetryExhaustionEmitsFailureEvent(t *testing.T) {
	recorder := &eventRecorder{}
	retry, err := resilience.NewRetry[int](resilience.RetryOptions{
		Name:        "pricing",
		MaxAttempts: 2,
		Backoff:     resilience.NoBackoff(),
		Sleeper:     &fakeSleeper{},
		OnEvent:     recorder.onEvent,
	})
	if err != nil {
		t.Fatalf("NewRetry failed: %v", err)
	}

	_, err = resilience.Run(context.Background(), func(context.Context) (int, error) {
		return 0, errors.New("still down")
	}, retry)
	if !errors.Is(err, resilience.ErrRetryExhausted) {
		t.Fatalf("expected ErrRetryExhausted, got %v", err)
	}

	events := recorder.snapshot()
	if len(events) != 2 {
		t.Fatalf("events = %#v, want retry then failure", events)
	}
	assertEvent(t, events[0], eventWant{
		policyName:    "pricing",
		policyType:    resilience.PolicyTypeRetry,
		kind:          resilience.EventRetry,
		category:      resilience.EventCategoryRetry,
		attempt:       1,
		errorCategory: resilience.ErrorCategoryFailure,
	})
	assertEvent(t, events[1], eventWant{
		policyName:    "pricing",
		policyType:    resilience.PolicyTypeRetry,
		kind:          resilience.EventFailure,
		category:      resilience.EventCategoryFailure,
		attempt:       2,
		errorCategory: resilience.ErrorCategoryRetryExhausted,
	})
	if !errors.Is(events[1].Err, resilience.ErrRetryExhausted) {
		t.Fatalf("failure event error = %v, want ErrRetryExhausted", events[1].Err)
	}
}

func TestRetryPredicateRejectedFailureEmitsFailureEvent(t *testing.T) {
	operationErr := errors.New("permanent")
	recorder := &eventRecorder{}
	retry, err := resilience.NewRetry[int](resilience.RetryOptions{
		Name:        "orders",
		MaxAttempts: 3,
		Backoff:     resilience.NoBackoff(),
		Sleeper:     &fakeSleeper{},
		RetryIf: func(error) bool {
			return false
		},
		OnEvent: recorder.onEvent,
	})
	if err != nil {
		t.Fatalf("NewRetry failed: %v", err)
	}

	_, err = resilience.Run(context.Background(), func(context.Context) (int, error) {
		return 0, operationErr
	}, retry)
	if !errors.Is(err, operationErr) {
		t.Fatalf("expected operation error, got %v", err)
	}

	events := recorder.snapshot()
	if len(events) != 1 {
		t.Fatalf("events = %#v, want one failure event", events)
	}
	assertEvent(t, events[0], eventWant{
		policyName:    "orders",
		policyType:    resilience.PolicyTypeRetry,
		kind:          resilience.EventFailure,
		category:      resilience.EventCategoryFailure,
		attempt:       1,
		err:           operationErr,
		errorCategory: resilience.ErrorCategoryFailure,
	})
}

func TestTimeoutEventsExposeDurationAndParentCancellationDoesNotEmitTimeout(t *testing.T) {
	recorder := &eventRecorder{}
	timeout, err := resilience.NewTimeout[string](resilience.TimeoutOptions{
		Name:    "inventory",
		Timeout: 5 * time.Millisecond,
		OnEvent: recorder.onEvent,
	})
	if err != nil {
		t.Fatalf("NewTimeout failed: %v", err)
	}

	_, err = resilience.Run(context.Background(), func(ctx context.Context) (string, error) {
		<-ctx.Done()
		return "", ctx.Err()
	}, timeout)
	if !errors.Is(err, resilience.ErrTimeout) {
		t.Fatalf("expected ErrTimeout, got %v", err)
	}

	events := recorder.snapshot()
	if len(events) != 1 {
		t.Fatalf("events = %#v, want one timeout event", events)
	}
	assertEvent(t, events[0], eventWant{
		policyName:    "inventory",
		policyType:    resilience.PolicyTypeTimeout,
		kind:          resilience.EventTimeout,
		category:      resilience.EventCategoryTimeout,
		timeout:       5 * time.Millisecond,
		errorCategory: resilience.ErrorCategoryTimeout,
	})

	parentRecorder := &eventRecorder{}
	parentTimeout, err := resilience.NewTimeout[int](resilience.TimeoutOptions{
		Name:    "parent",
		Timeout: time.Second,
		OnEvent: parentRecorder.onEvent,
	})
	if err != nil {
		t.Fatalf("NewTimeout failed: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err = resilience.Run(ctx, func(context.Context) (int, error) {
		t.Fatal("operation should not run")
		return 0, nil
	}, parentTimeout)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
	if events := parentRecorder.snapshot(); len(events) != 0 {
		t.Fatalf("parent cancellation events = %#v, want none", events)
	}
}

func TestCircuitBreakerEventsExposeTransitionAndRejectionPayloads(t *testing.T) {
	now := time.Date(2026, 6, 3, 0, 0, 0, 0, time.UTC)
	recorder := &eventRecorder{}
	breaker, err := resilience.NewCircuitBreaker[int](resilience.CircuitBreakerOptions{
		Name:             "payments",
		FailureThreshold: 1,
		OpenTimeout:      time.Second,
		Now: func() time.Time {
			return now
		},
		OnEvent: recorder.onEvent,
	})
	if err != nil {
		t.Fatalf("NewCircuitBreaker failed: %v", err)
	}

	_, _ = resilience.Run(context.Background(), func(context.Context) (int, error) {
		return 0, errors.New("downstream failed")
	}, breaker)
	_, err = resilience.Run(context.Background(), func(context.Context) (int, error) {
		t.Fatal("open circuit must not execute operation")
		return 0, nil
	}, breaker)
	if !errors.Is(err, resilience.ErrCircuitOpen) {
		t.Fatalf("expected ErrCircuitOpen, got %v", err)
	}

	now = now.Add(time.Second)
	got, err := resilience.Run(context.Background(), func(context.Context) (int, error) {
		return 1, nil
	}, breaker)
	if err != nil {
		t.Fatalf("half-open probe failed: %v", err)
	}
	if got != 1 {
		t.Fatalf("got %d, want 1", got)
	}

	events := recorder.snapshot()
	if len(events) != 4 {
		t.Fatalf("events = %#v, want open transition, rejection, half-open transition, close transition", events)
	}
	assertEvent(t, events[0], eventWant{
		policyName:    "payments",
		policyType:    resilience.PolicyTypeCircuitBreaker,
		kind:          resilience.EventCircuitStateTransition,
		category:      resilience.EventCategoryTransition,
		state:         resilience.CircuitStateOpen,
		previousState: resilience.CircuitStateClosed,
	})
	assertEvent(t, events[1], eventWant{
		policyName:    "payments",
		policyType:    resilience.PolicyTypeCircuitBreaker,
		kind:          resilience.EventCircuitRejected,
		category:      resilience.EventCategoryRejection,
		state:         resilience.CircuitStateOpen,
		errorCategory: resilience.ErrorCategoryCircuitOpen,
	})
	assertEvent(t, events[2], eventWant{
		policyName:    "payments",
		policyType:    resilience.PolicyTypeCircuitBreaker,
		kind:          resilience.EventCircuitStateTransition,
		category:      resilience.EventCategoryTransition,
		state:         resilience.CircuitStateHalfOpen,
		previousState: resilience.CircuitStateOpen,
	})
	assertEvent(t, events[3], eventWant{
		policyName:    "payments",
		policyType:    resilience.PolicyTypeCircuitBreaker,
		kind:          resilience.EventCircuitStateTransition,
		category:      resilience.EventCategoryTransition,
		state:         resilience.CircuitStateClosed,
		previousState: resilience.CircuitStateHalfOpen,
	})
}

func TestBulkheadEventsExposeAdmissionRejectionAndSuccessPayloads(t *testing.T) {
	recorder := &eventRecorder{}
	bulkhead, err := resilience.NewBulkhead[int](resilience.BulkheadOptions{
		Name:          "workers",
		MaxConcurrent: 1,
		OnEvent:       recorder.onEvent,
	})
	if err != nil {
		t.Fatalf("NewBulkhead failed: %v", err)
	}

	entered := make(chan struct{})
	release := make(chan struct{})
	done := make(chan error, 1)
	go func() {
		_, err := resilience.Run(context.Background(), func(context.Context) (int, error) {
			close(entered)
			<-release
			return 1, nil
		}, bulkhead)
		done <- err
	}()

	<-entered
	_, err = resilience.Run(context.Background(), func(context.Context) (int, error) {
		t.Fatal("rejected operation must not execute")
		return 0, nil
	}, bulkhead)
	if !errors.Is(err, resilience.ErrBulkheadRejected) {
		t.Fatalf("expected ErrBulkheadRejected, got %v", err)
	}

	close(release)
	if err := <-done; err != nil {
		t.Fatalf("first operation failed: %v", err)
	}

	events := recorder.snapshot()
	if len(events) != 3 {
		t.Fatalf("events = %#v, want accepted, rejected, success", events)
	}
	assertEvent(t, events[0], eventWant{
		policyName: "workers",
		policyType: resilience.PolicyTypeBulkhead,
		kind:       resilience.EventBulkheadAccepted,
		category:   resilience.EventCategoryAdmission,
		inFlight:   1,
	})
	assertEvent(t, events[1], eventWant{
		policyName:    "workers",
		policyType:    resilience.PolicyTypeBulkhead,
		kind:          resilience.EventBulkheadRejected,
		category:      resilience.EventCategoryRejection,
		inFlight:      1,
		errorCategory: resilience.ErrorCategoryBulkheadRejected,
	})
	assertEvent(t, events[2], eventWant{
		policyName: "workers",
		policyType: resilience.PolicyTypeBulkhead,
		kind:       resilience.EventSuccess,
		category:   resilience.EventCategorySuccess,
		inFlight:   1,
	})
}

type eventWant struct {
	policyName    string
	policyType    string
	kind          resilience.EventKind
	category      resilience.EventCategory
	attempt       int
	delay         time.Duration
	timeout       time.Duration
	err           error
	errorCategory resilience.ErrorCategory
	state         resilience.CircuitState
	previousState resilience.CircuitState
	inFlight      int
}

func assertEvent(t *testing.T, got resilience.Event, want eventWant) {
	t.Helper()

	if got.PolicyName != want.policyName {
		t.Fatalf("policy name = %q, want %q in event %+v", got.PolicyName, want.policyName, got)
	}
	if got.PolicyType != want.policyType {
		t.Fatalf("policy type = %q, want %q in event %+v", got.PolicyType, want.policyType, got)
	}
	if got.Kind != want.kind {
		t.Fatalf("kind = %q, want %q in event %+v", got.Kind, want.kind, got)
	}
	if got.Category != want.category {
		t.Fatalf("category = %q, want %q in event %+v", got.Category, want.category, got)
	}
	if got.Attempt != want.attempt {
		t.Fatalf("attempt = %d, want %d in event %+v", got.Attempt, want.attempt, got)
	}
	if got.Delay != want.delay {
		t.Fatalf("delay = %s, want %s in event %+v", got.Delay, want.delay, got)
	}
	if got.Timeout != want.timeout {
		t.Fatalf("timeout = %s, want %s in event %+v", got.Timeout, want.timeout, got)
	}
	if want.err != nil && !errors.Is(got.Err, want.err) {
		t.Fatalf("err = %v, want %v in event %+v", got.Err, want.err, got)
	}
	if got.ErrorCategory != want.errorCategory {
		t.Fatalf("error category = %q, want %q in event %+v", got.ErrorCategory, want.errorCategory, got)
	}
	if got.State != want.state {
		t.Fatalf("state = %q, want %q in event %+v", got.State, want.state, got)
	}
	if got.PreviousState != want.previousState {
		t.Fatalf("previous state = %q, want %q in event %+v", got.PreviousState, want.previousState, got)
	}
	if got.InFlight != want.inFlight {
		t.Fatalf("in flight = %d, want %d in event %+v", got.InFlight, want.inFlight, got)
	}
}
