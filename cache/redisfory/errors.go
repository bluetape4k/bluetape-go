package redisfory

import (
	"errors"
	"fmt"
)

var (
	errProviderFailed     = errors.New("redisfory provider failed")
	errRegistrationFailed = errors.New("redisfory registration failed")
)

// Reason string 공개 타입이며 Redis 값 캐시의 serialization, TTL, backend ownership 계약을 보존한다.
// 필드와 zero value, nil 허용 여부, 동시성 소유권은 생성자와 메서드의 한국어 주석 및 테스트 계약을 따른다.
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

// CacheError struct 공개 타입이며 Redis 값 캐시의 serialization, TTL, backend ownership 계약을 보존한다.
// 필드와 zero value, nil 허용 여부, 동시성 소유권은 생성자와 메서드의 한국어 주석 및 테스트 계약을 따른다.
type CacheError struct {
	operation string
	profile   Profile
	reason    Reason
	cause     error
}

// Error Error 공개 API의 동작을 수행하며 Redis 값 캐시의 serialization, TTL, backend ownership 계약을 보존한다.
func (e *CacheError) Error() string {
	if e == nil {
		return "<nil>"
	}
	return fmt.Sprintf("redisfory %s failed: %s", e.operation, e.reason)
}

// Unwrap Unwrap 공개 API의 동작을 수행하며 Redis 값 캐시의 serialization, TTL, backend ownership 계약을 보존한다.
//
// 반환 오류는 cache miss, 입력 검증 실패, context 취소, Redis/backend 실패, package sentinel error와 typed error를 그대로 드러낸다.
func (e *CacheError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.cause
}

// Operation Operation 공개 API의 동작을 수행하며 Redis 값 캐시의 serialization, TTL, backend ownership 계약을 보존한다.
func (e *CacheError) Operation() string {
	if e == nil {
		return ""
	}
	return e.operation
}

// Profile Profile 공개 API의 동작을 수행하며 Redis 값 캐시의 serialization, TTL, backend ownership 계약을 보존한다.
func (e *CacheError) Profile() Profile {
	if e == nil {
		return ""
	}
	return e.profile
}

// Reason Reason 공개 API의 동작을 수행하며 Redis 값 캐시의 serialization, TTL, backend ownership 계약을 보존한다.
func (e *CacheError) Reason() Reason {
	if e == nil {
		return ""
	}
	return e.reason
}
