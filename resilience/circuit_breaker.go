package resilience

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

// CircuitState string 공개 타입이며 취소, deadline, retry, timeout, circuit breaker 상태를 보존한다.
type CircuitState string

const (
	// CircuitStateClosed admits calls and records outcomes.
	CircuitStateClosed CircuitState = "closed"
	// CircuitStateOpen rejects calls until the open interval elapses.
	CircuitStateOpen CircuitState = "open"
	// CircuitStateHalfOpen admits a bounded number of probe calls.
	CircuitStateHalfOpen CircuitState = "half-open"
)

// FailurePredicate func 공개 타입이며 취소, deadline, retry, timeout, circuit breaker 상태를 보존한다.
type FailurePredicate func(error) bool

// CircuitBreakerOptions struct 공개 타입이며 취소, deadline, retry, timeout, circuit breaker 상태를 보존한다.
type CircuitBreakerOptions struct {
	Name                  string
	FailureThreshold      int
	SuccessThreshold      int
	OpenTimeout           time.Duration
	HalfOpenMaxConcurrent int
	FailureIf             FailurePredicate
	Now                   func() time.Time
	OnEvent               EventHandler
}

// CircuitBreakerPolicy struct 공개 타입이며 취소, deadline, retry, timeout, circuit breaker 상태를 보존한다.
type CircuitBreakerPolicy[T any] struct {
	options CircuitBreakerOptions

	mu               sync.Mutex
	state            CircuitState
	failureCount     int
	successCount     int
	openedAt         time.Time
	halfOpenInFlight int
}

// NewCircuitBreaker NewCircuitBreaker 공개 API의 동작을 수행하며 취소, deadline, retry, timeout, circuit breaker 상태를 보존한다.
//
// 매개변수:
//   - options: 적용할 옵션 목록이다. nil이면 기본값만 사용한다.
//
// 반환 오류는 입력 검증 실패, context 취소/deadline, 상태 전이 실패, 패키지 sentinel error와 typed error를 그대로 드러낸다.
func NewCircuitBreaker[T any](options CircuitBreakerOptions) (*CircuitBreakerPolicy[T], error) {
	if options.FailureThreshold <= 0 {
		return nil, fmt.Errorf("failure threshold must be positive")
	}
	if options.SuccessThreshold <= 0 {
		options.SuccessThreshold = 1
	}
	if options.OpenTimeout <= 0 {
		return nil, fmt.Errorf("open timeout must be positive")
	}
	if options.HalfOpenMaxConcurrent <= 0 {
		options.HalfOpenMaxConcurrent = 1
	}
	if options.FailureIf == nil {
		options.FailureIf = defaultFailurePredicate
	}
	if options.Now == nil {
		options.Now = time.Now
	}

	return &CircuitBreakerPolicy[T]{
		options: options,
		state:   CircuitStateClosed,
	}, nil
}

// State State 공개 API의 동작을 수행하며 취소, deadline, retry, timeout, circuit breaker 상태를 보존한다.
func (p *CircuitBreakerPolicy[T]) State() CircuitState {
	p.mu.Lock()
	defer p.mu.Unlock()

	return p.state
}

// Apply Apply 공개 API의 동작을 수행하며 취소, deadline, retry, timeout, circuit breaker 상태를 보존한다.
//
// 매개변수:
//   - operation: 보호 정책 안에서 실행할 작업이다.
func (p *CircuitBreakerPolicy[T]) Apply(operation Operation[T]) Operation[T] {
	return func(ctx context.Context) (T, error) {
		var zero T
		if ctx == nil {
			ctx = context.Background()
		}

		state, err := p.beforeCall(ctx)
		if err != nil {
			return zero, err
		}

		panicked := true
		defer func() {
			if panicked {
				p.afterCall(ctx, state, true)
			}
		}()

		value, err := runOperation(ctx, operation)
		panicked = false
		p.afterCall(ctx, state, p.options.FailureIf(err))
		return value, err
	}
}

func (p *CircuitBreakerPolicy[T]) beforeCall(ctx context.Context) (CircuitState, error) {
	now := p.options.Now()

	p.mu.Lock()
	var transition *Event
	state := p.state

	if p.state == CircuitStateOpen && now.Sub(p.openedAt) >= p.options.OpenTimeout {
		transition = p.transitionLocked(CircuitStateHalfOpen, now)
		state = p.state
	}

	if p.state == CircuitStateOpen {
		rejection := CircuitOpenError{PolicyName: p.options.Name, State: p.state}
		event := Event{
			PolicyName:    p.options.Name,
			PolicyType:    PolicyTypeCircuitBreaker,
			Kind:          EventCircuitRejected,
			Category:      EventCategoryRejection,
			State:         p.state,
			Err:           rejection,
			ErrorCategory: categorizeError(rejection),
		}
		p.mu.Unlock()
		emitEvent(ctx, p.options.OnEvent, event)
		return "", rejection
	}

	if p.state == CircuitStateHalfOpen {
		if p.halfOpenInFlight >= p.options.HalfOpenMaxConcurrent {
			rejection := CircuitOpenError{PolicyName: p.options.Name, State: p.state}
			event := Event{
				PolicyName:    p.options.Name,
				PolicyType:    PolicyTypeCircuitBreaker,
				Kind:          EventCircuitRejected,
				Category:      EventCategoryRejection,
				State:         p.state,
				InFlight:      p.halfOpenInFlight,
				Err:           rejection,
				ErrorCategory: categorizeError(rejection),
			}
			p.mu.Unlock()
			emitEvent(ctx, p.options.OnEvent, event)
			return "", rejection
		}
		p.halfOpenInFlight++
		state = p.state
	}
	p.mu.Unlock()

	if transition != nil {
		emitEvent(ctx, p.options.OnEvent, *transition)
	}

	return state, nil
}

func (p *CircuitBreakerPolicy[T]) afterCall(ctx context.Context, callState CircuitState, failed bool) {
	now := p.options.Now()

	p.mu.Lock()
	var transition *Event

	switch callState {
	case CircuitStateClosed:
		if failed {
			p.failureCount++
			if p.failureCount >= p.options.FailureThreshold {
				transition = p.transitionLocked(CircuitStateOpen, now)
			}
		} else {
			p.failureCount = 0
		}
	case CircuitStateHalfOpen:
		if p.halfOpenInFlight > 0 {
			p.halfOpenInFlight--
		}
		if failed {
			transition = p.transitionLocked(CircuitStateOpen, now)
		} else {
			p.successCount++
			if p.successCount >= p.options.SuccessThreshold {
				transition = p.transitionLocked(CircuitStateClosed, now)
			}
		}
	}

	p.mu.Unlock()

	if transition != nil {
		emitEvent(ctx, p.options.OnEvent, *transition)
	}
}

func (p *CircuitBreakerPolicy[T]) transitionLocked(next CircuitState, now time.Time) *Event {
	previous := p.state
	if previous == next {
		return nil
	}

	p.state = next
	p.failureCount = 0
	p.successCount = 0
	p.halfOpenInFlight = 0
	if next == CircuitStateOpen {
		p.openedAt = now
	}

	return &Event{
		PolicyName:    p.options.Name,
		PolicyType:    PolicyTypeCircuitBreaker,
		Kind:          EventCircuitStateTransition,
		Category:      EventCategoryTransition,
		State:         next,
		PreviousState: previous,
	}
}

func defaultFailurePredicate(err error) bool {
	if err == nil || errors.Is(err, context.Canceled) {
		return false
	}
	return true
}
