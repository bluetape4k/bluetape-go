package rediscoordfory

import (
	"errors"
	"fmt"
)

var (
	errRegistrationFailed = errors.New("fory codec registration failed")
	errProviderFailed     = errors.New("fory codec provider failed")
)

// Profile Redis 조정, stampede 방지, codec envelope에서 사용하는 문자열 타입이다.
type Profile string

const (
	// ProfileNativeFast 고정 schema Go-native profile이다.
	ProfileNativeFast Profile = "native-fast"
	// ProfileNativeCompatible schema-compatible Go-native profile이다.
	ProfileNativeCompatible Profile = "native-compatible"
)

// Reason Redis 조정, stampede 방지, codec envelope에서 사용하는 문자열 타입이다.
type Reason string

const (
	// ReasonConfiguration identifies invalid codec configuration.
	ReasonConfiguration Reason = "configuration"
	// ReasonUninitialized identifies use of a zero-value codec.
	ReasonUninitialized Reason = "uninitialized"
	// ReasonRegistration identifies deterministic type registration failure.
	ReasonRegistration Reason = "registration"
	// ReasonPayloadTooLarge identifies a configured payload bound violation.
	ReasonPayloadTooLarge Reason = "payload-too-large"
	// ReasonInvalidMagic identifies a non-BTFY payload.
	ReasonInvalidMagic Reason = "invalid-magic"
	// ReasonUnsupportedVersion identifies an unknown BTFY wrapper version.
	ReasonUnsupportedVersion Reason = "unsupported-version"
	// ReasonProfileMismatch identifies a wrapper from another Fory profile.
	ReasonProfileMismatch Reason = "profile-mismatch"
	// ReasonLengthMismatch identifies truncated or trailing wrapper bytes.
	ReasonLengthMismatch Reason = "length-mismatch"
	// ReasonUnsupportedValue identifies an unsupported generic root shape.
	ReasonUnsupportedValue Reason = "unsupported-value"
	// ReasonForyFailure identifies a provider marshal or unmarshal failure.
	ReasonForyFailure Reason = "fory-failure"
)

// CodecError Redis 조정, stampede 방지, codec envelope에서 사용하는 구조체다.
type CodecError struct {
	operation string
	profile   Profile
	reason    Reason
	cause     error
}

func (e *CodecError) Error() string {
	if e == nil {
		return "<nil>"
	}
	return fmt.Sprintf("fory codec %s failed (%s): %s", e.operation, e.profile, e.reason)
}

// Unwrap 감싼 원인 오류를 반환한다.
//
// 반환 오류는 cache miss, 입력 검증 실패, context 취소, Redis/backend 실패, package sentinel error와 typed error를 그대로 드러낸다.
func (e *CodecError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.cause
}

// Operation Redis 조정, stampede 방지, codec envelope의 식별 정보를 반환한다.
func (e *CodecError) Operation() string { return e.operation }

// Profile Redis 조정, stampede 방지, codec envelope의 식별 정보를 반환한다.
func (e *CodecError) Profile() Profile { return e.profile }

// Reason Redis 조정, stampede 방지, codec envelope의 식별 정보를 반환한다.
func (e *CodecError) Reason() Reason { return e.reason }
