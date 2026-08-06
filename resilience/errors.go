package resilience

import (
	"errors"
	"fmt"
	"time"
)

// Sentinel errors used with errors.Is.
var (
	ErrRetryExhausted   = errors.New("retry attempts exhausted")
	ErrTimeout          = errors.New("operation timed out")
	ErrCircuitOpen      = errors.New("circuit breaker is open")
	ErrBulkheadRejected = errors.New("bulkhead rejected call")
)

// RetryError struct 공개 타입이며 취소, deadline, retry, timeout, circuit breaker 상태를 보존한다.
type RetryError struct {
	PolicyName string
	Attempts   int
	Cause      error
}

func (e RetryError) Error() string {
	if e.PolicyName != "" {
		return fmt.Sprintf("retry policy %q exhausted after %d attempts: %v", e.PolicyName, e.Attempts, e.Cause)
	}
	return fmt.Sprintf("retry exhausted after %d attempts: %v", e.Attempts, e.Cause)
}

// Unwrap Unwrap 공개 API의 동작을 수행하며 취소, deadline, retry, timeout, circuit breaker 상태를 보존한다.
//
// 반환 오류는 입력 검증 실패, context 취소/deadline, 상태 전이 실패, 패키지 sentinel error와 typed error를 그대로 드러낸다.
func (e RetryError) Unwrap() error {
	return e.Cause
}

// Is Is 공개 API의 동작을 수행하며 취소, deadline, retry, timeout, circuit breaker 상태를 보존한다.
//
// 매개변수:
//   - target: 검사하거나 감쌀 오류 값이다.
func (e RetryError) Is(target error) bool {
	return target == ErrRetryExhausted
}

// TimeoutError struct 공개 타입이며 취소, deadline, retry, timeout, circuit breaker 상태를 보존한다.
type TimeoutError struct {
	PolicyName string
	Timeout    time.Duration
	Cause      error
}

func (e TimeoutError) Error() string {
	if e.PolicyName != "" {
		return fmt.Sprintf("timeout policy %q expired after %s: %v", e.PolicyName, e.Timeout, e.Cause)
	}
	return fmt.Sprintf("timeout expired after %s: %v", e.Timeout, e.Cause)
}

// Unwrap Unwrap 공개 API의 동작을 수행하며 취소, deadline, retry, timeout, circuit breaker 상태를 보존한다.
//
// 반환 오류는 입력 검증 실패, context 취소/deadline, 상태 전이 실패, 패키지 sentinel error와 typed error를 그대로 드러낸다.
func (e TimeoutError) Unwrap() error {
	return e.Cause
}

// Is Is 공개 API의 동작을 수행하며 취소, deadline, retry, timeout, circuit breaker 상태를 보존한다.
//
// 매개변수:
//   - target: 검사하거나 감쌀 오류 값이다.
func (e TimeoutError) Is(target error) bool {
	return target == ErrTimeout
}

// CircuitOpenError struct 공개 타입이며 취소, deadline, retry, timeout, circuit breaker 상태를 보존한다.
type CircuitOpenError struct {
	PolicyName string
	State      CircuitState
}

func (e CircuitOpenError) Error() string {
	if e.PolicyName != "" {
		return fmt.Sprintf("circuit breaker %q rejected call in %s state", e.PolicyName, e.State)
	}
	return fmt.Sprintf("circuit breaker rejected call in %s state", e.State)
}

// Is Is 공개 API의 동작을 수행하며 취소, deadline, retry, timeout, circuit breaker 상태를 보존한다.
//
// 매개변수:
//   - target: 검사하거나 감쌀 오류 값이다.
func (e CircuitOpenError) Is(target error) bool {
	return target == ErrCircuitOpen
}

// BulkheadRejectedError struct 공개 타입이며 취소, deadline, retry, timeout, circuit breaker 상태를 보존한다.
type BulkheadRejectedError struct {
	PolicyName string
}

func (e BulkheadRejectedError) Error() string {
	if e.PolicyName != "" {
		return fmt.Sprintf("bulkhead %q rejected call", e.PolicyName)
	}
	return "bulkhead rejected call"
}

// Is Is 공개 API의 동작을 수행하며 취소, deadline, retry, timeout, circuit breaker 상태를 보존한다.
//
// 매개변수:
//   - target: 검사하거나 감쌀 오류 값이다.
func (e BulkheadRejectedError) Is(target error) bool {
	return target == ErrBulkheadRejected
}
