package redisvalue

import (
	"errors"
	"fmt"
	"log/slog"
)

// Reason tiered Redis value cache의 local/remote ownership, TTL, clear coordination에서 사용하는 문자열 타입이다.
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

// ClearProgress tiered Redis value cache의 local/remote ownership, TTL, clear coordination에서 사용하는 구조체다.
type ClearProgress struct {
	ScannedKeys     int64
	UnlinkedBatches int64
}

// CacheError tiered Redis value cache의 local/remote ownership, TTL, clear coordination에서 사용하는 구조체다.
type CacheError struct {
	operation string
	reason    Reason
	keyID     string
	progress  ClearProgress
	hasClear  bool
	cause     error
}

// Error 오류 메시지를 반환한다.
func (e *CacheError) Error() string {
	if e == nil {
		return "<nil>"
	}
	if e.keyID == "" {
		return fmt.Sprintf("redisvalue %s failed: %s", e.operation, e.reason)
	}
	return fmt.Sprintf("redisvalue %s failed for %s: %s", e.operation, e.keyID, e.reason)
}

// GoString tiered Redis value cache의 local/remote ownership, TTL, clear coordination 동작을 수행한다.
func (e *CacheError) GoString() string {
	return e.Error()
}

// LogValue tiered Redis value cache의 local/remote ownership, TTL, clear coordination 동작을 수행한다.
func (e *CacheError) LogValue() slog.Value {
	return slog.StringValue(e.Error())
}

// Unwrap 감싼 원인 오류를 반환한다.
//
// 반환 오류는 cache miss, 입력 검증 실패, context 취소, Redis/backend 실패, package sentinel error와 typed error를 그대로 드러낸다.
func (e *CacheError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.cause
}

// Operation tiered Redis value cache의 local/remote ownership, TTL, clear coordination의 식별 정보를 반환한다.
func (e *CacheError) Operation() string {
	if e == nil {
		return ""
	}
	return e.operation
}

// Reason tiered Redis value cache의 local/remote ownership, TTL, clear coordination의 식별 정보를 반환한다.
func (e *CacheError) Reason() Reason {
	if e == nil {
		return ""
	}
	return e.reason
}

// ClearProgress tiered Redis value cache의 local/remote ownership, TTL, clear coordination 동작을 수행한다.
func (e *CacheError) ClearProgress() (ClearProgress, bool) {
	if e == nil {
		return ClearProgress{}, false
	}
	if e.hasClear {
		return e.progress, true
	}
	var nested *CacheError
	if errors.As(e.cause, &nested) && nested != e {
		return nested.ClearProgress()
	}
	return ClearProgress{}, false
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
