package resilience

import (
	"errors"
	"fmt"
	"time"
)

// Sentinel errors used with errors.Is.
var (
	ErrRetryExhausted = errors.New("retry attempts exhausted")
	ErrTimeout        = errors.New("operation timed out")
)

// RetryError reports that a retry policy exhausted its attempts.
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

// Unwrap returns the last operation error.
func (e RetryError) Unwrap() error {
	return e.Cause
}

// Is allows errors.Is(err, ErrRetryExhausted).
func (e RetryError) Is(target error) bool {
	return target == ErrRetryExhausted
}

// TimeoutError reports that a timeout policy's own deadline expired.
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

// Unwrap returns the context deadline error observed by the operation.
func (e TimeoutError) Unwrap() error {
	return e.Cause
}

// Is allows errors.Is(err, ErrTimeout).
func (e TimeoutError) Is(target error) bool {
	return target == ErrTimeout
}
