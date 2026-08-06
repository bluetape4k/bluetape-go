package redisfory

import (
	"errors"
	"fmt"
)

var (
	errProviderFailed     = errors.New("redisfory provider failed")
	errRegistrationFailed = errors.New("redisfory registration failed")
)

// Reason Redis 값 캐시의 serialization, TTL, backend ownership에서 사용하는 문자열 타입이다.
type Reason string

const (
	// ReasonConfiguration identifies invalid cache configuration.
	ReasonConfiguration Reason = "configuration"
	// ReasonUninitialized identifies use of a zero-value cache.
	ReasonUninitialized Reason = "uninitialized"
	// ReasonRegistration identifies deterministic Fory registration failure.
	ReasonRegistration Reason = "registration"
	// ReasonPayloadTooLarge identifies a configured payload limit violation.
	ReasonPayloadTooLarge Reason = "payload-too-large"
	// ReasonInvalidMagic identifies data without the BTFV envelope marker.
	ReasonInvalidMagic Reason = "invalid-magic"
	// ReasonUnsupportedVersion identifies an unsupported BTFV version.
	ReasonUnsupportedVersion Reason = "unsupported-version"
	// ReasonFormatMismatch identifies data written by another Fory profile.
	ReasonFormatMismatch Reason = "format-mismatch"
	// ReasonSchemaMismatch identifies data written for another schema generation.
	ReasonSchemaMismatch Reason = "schema-mismatch"
	// ReasonLengthMismatch identifies truncated or trailing envelope data.
	ReasonLengthMismatch Reason = "length-mismatch"
	// ReasonUnsupportedValue identifies a generic root type unsupported by this cache.
	ReasonUnsupportedValue Reason = "unsupported-value"
	// ReasonForyFailure identifies a sanitized Fory provider failure.
	ReasonForyFailure Reason = "fory-failure"
)

// CacheError Redis 값 캐시의 serialization, TTL, backend ownership에서 사용하는 구조체다.
type CacheError struct {
	operation string
	profile   Profile
	reason    Reason
	cause     error
}

// Error 오류 메시지를 반환한다.
func (e *CacheError) Error() string {
	if e == nil {
		return "<nil>"
	}
	return fmt.Sprintf("redisfory %s failed: %s", e.operation, e.reason)
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

// Operation Redis 값 캐시의 serialization, TTL, backend ownership의 식별 정보를 반환한다.
func (e *CacheError) Operation() string {
	if e == nil {
		return ""
	}
	return e.operation
}

// Profile Redis 값 캐시의 serialization, TTL, backend ownership의 식별 정보를 반환한다.
func (e *CacheError) Profile() Profile {
	if e == nil {
		return ""
	}
	return e.profile
}

// Reason Redis 값 캐시의 serialization, TTL, backend ownership의 식별 정보를 반환한다.
func (e *CacheError) Reason() Reason {
	if e == nil {
		return ""
	}
	return e.reason
}
