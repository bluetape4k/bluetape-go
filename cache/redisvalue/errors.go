package redisvalue

import "fmt"

// Reason is a stable low-cardinality Redis value-cache failure category.
type Reason string

const (
	// ReasonConfiguration identifies invalid constructor or configuration input.
	ReasonConfiguration Reason = "configuration"
	// ReasonUninitialized identifies use of a zero-value constructor-only cache.
	ReasonUninitialized Reason = "uninitialized"
	// ReasonSerialization identifies a serializer Marshal failure.
	ReasonSerialization Reason = "serialization"
	// ReasonPayloadTooLarge identifies a value that exceeds the configured bound.
	ReasonPayloadTooLarge Reason = "payload-too-large"
	// ReasonInvalidPayload identifies a serializer Unmarshal failure.
	ReasonInvalidPayload Reason = "invalid-payload"
	// ReasonLocalFailure identifies a caller-owned L1 operation failure.
	ReasonLocalFailure Reason = "local-failure"
	// ReasonLocalBlocked identifies a tiered cache blocked pending explicit repair.
	ReasonLocalBlocked Reason = "local-blocked"
	// ReasonProviderFailure identifies a Redis provider operation failure.
	ReasonProviderFailure Reason = "provider-failure"
	// ReasonPartialClear identifies a namespace clear that stopped after progress.
	ReasonPartialClear Reason = "partial-clear"
)

// ClearProgress reports monotonic partial namespace-clear work. ScannedKeys is
// only the number of matching keys returned by SCAN so far; it is not a total,
// completion percentage, or cursor.
type ClearProgress struct {
	ScannedKeys     int64
	UnlinkedBatches int64
}

// CacheError is an inspectable and redacted Redis value-cache failure.
type CacheError struct {
	operation string
	reason    Reason
	keyID     string
	progress  ClearProgress
	hasClear  bool
	cause     error
}

// Error returns a redacted low-cardinality failure message.
func (e *CacheError) Error() string {
	if e == nil {
		return "<nil>"
	}
	if e.keyID == "" {
		return fmt.Sprintf("redisvalue %s failed: %s", e.operation, e.reason)
	}
	return fmt.Sprintf("redisvalue %s failed for %s: %s", e.operation, e.keyID, e.reason)
}

// Unwrap returns the causal error.
func (e *CacheError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.cause
}

// Operation returns the stable operation label.
func (e *CacheError) Operation() string {
	if e == nil {
		return ""
	}
	return e.operation
}

// Reason returns the stable failure category.
func (e *CacheError) Reason() Reason {
	if e == nil {
		return ""
	}
	return e.reason
}

// ClearProgress returns partial namespace-clear progress when applicable.
func (e *CacheError) ClearProgress() (ClearProgress, bool) {
	if e == nil || !e.hasClear {
		return ClearProgress{}, false
	}
	return e.progress, true
}

func newCacheError(operation string, reason Reason, keyID string, cause error) *CacheError {
	return &CacheError{
		operation: operation,
		reason:    reason,
		keyID:     keyID,
		cause:     cause,
	}
}

func newPartialClearError(operation string, progress ClearProgress, cause error) *CacheError {
	return &CacheError{
		operation: operation,
		reason:    ReasonPartialClear,
		progress:  progress,
		hasClear:  true,
		cause:     cause,
	}
}
