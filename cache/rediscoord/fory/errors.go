package rediscoordfory

import (
	"errors"
	"fmt"
)

var (
	errRegistrationFailed = errors.New("fory codec registration failed")
	errProviderFailed     = errors.New("fory codec provider failed")
)

// Profile string 공개 타입이며 Redis 조정, stampede 방지, codec envelope 계약을 보존한다.
// 필드와 zero value, nil 허용 여부, 동시성 소유권은 생성자와 메서드의 한국어 주석 및 테스트 계약을 따른다.
type Profile string

const (
	// ProfileNativeFast 고정 schema Go-native profile이다.
	ProfileNativeFast Profile = "native-fast"
	// ProfileNativeCompatible은 schema-compatible Go-native profile이다.
	ProfileNativeCompatible Profile = "native-compatible"
)

// Reason string 공개 타입이며 Redis 조정, stampede 방지, codec envelope 계약을 보존한다.
// 필드와 zero value, nil 허용 여부, 동시성 소유권은 생성자와 메서드의 한국어 주석 및 테스트 계약을 따른다.
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

// CodecError struct 공개 타입이며 Redis 조정, stampede 방지, codec envelope 계약을 보존한다.
// 필드와 zero value, nil 허용 여부, 동시성 소유권은 생성자와 메서드의 한국어 주석 및 테스트 계약을 따른다.
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

// Unwrap Unwrap 공개 API의 동작을 수행하며 Redis 조정, stampede 방지, codec envelope 계약을 보존한다.
//
// 반환 오류는 cache miss, 입력 검증 실패, 취소, Redis/backend 실패, 또는 package sentinel/typed error 계약을 보존한다.
func (e *CodecError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.cause
}

// Operation Operation 공개 API의 동작을 수행하며 Redis 조정, stampede 방지, codec envelope 계약을 보존한다.
func (e *CodecError) Operation() string { return e.operation }

// Profile Profile 공개 API의 동작을 수행하며 Redis 조정, stampede 방지, codec envelope 계약을 보존한다.
func (e *CodecError) Profile() Profile { return e.profile }

// Reason Reason 공개 API의 동작을 수행하며 Redis 조정, stampede 방지, codec envelope 계약을 보존한다.
func (e *CodecError) Reason() Reason { return e.reason }
