package kms

import (
	"errors"
	"fmt"

	"github.com/bluetape4k/bluetape-go/encrypt"
)

var (
	// ErrNilClient - KMS client가 nil 또는 typed-nil일 때 반환된다.
	ErrNilClient = errors.New("kms: client must not be nil")
	// ErrInvalidKeyID - key ID가 비어 있거나 유효하지 않을 때 반환된다.
	ErrInvalidKeyID = errors.New("kms: key ID is invalid")
	// ErrInvalidProvider - zero-value 또는 초기화되지 않은 Provider일 때 반환된다.
	ErrInvalidProvider = errors.New("kms: invalid provider")
	// ErrInvalidOptions - provider option이 유효하지 않을 때 반환된다.
	ErrInvalidOptions = errors.New("kms: invalid options")
	// ErrInputTooLarge - provider 입력이 정의된 한도를 넘을 때 반환된다.
	ErrInputTooLarge = errors.New("kms: input exceeds limit")
	// ErrMalformedEnvelope - BTKMS envelope 형식이 유효하지 않을 때 반환된다.
	ErrMalformedEnvelope = errors.New("kms: malformed envelope")
	// ErrUnsupportedVersion - 지원하지 않는 envelope version일 때 반환된다.
	ErrUnsupportedVersion = errors.New("kms: unsupported envelope version")
	// ErrUnsupportedAlgorithm - 지원하지 않는 envelope algorithm일 때 반환된다.
	ErrUnsupportedAlgorithm = errors.New("kms: unsupported envelope algorithm")
	// ErrMetadataMismatch - envelope metadata와 provider 설정이 다를 때 반환된다.
	ErrMetadataMismatch = errors.New("kms: envelope metadata mismatch")
	// ErrInvalidDataKey - KMS data key output이 유효하지 않을 때 반환된다.
	ErrInvalidDataKey = errors.New("kms: invalid data key")
	// ErrKMSOperation - KMS operation이 실패했을 때 반환된다.
	ErrKMSOperation = errors.New("kms: KMS operation failed")
	// ErrAuthenticationFailed - local encrypt authentication이 실패했을 때 반환된다.
	ErrAuthenticationFailed = encrypt.ErrAuthenticationFailed
)

// Error - safe sentinel과 고정 operation label을 보존하는 provider 오류다.
// 원인은 Unwrap과 errors.Is로만 접근하며 Error 문자열에는 입력이나 SDK 상세를 넣지 않는다.
type Error struct {
	Kind      error
	Operation string
	Cause     error
}

// Error - secret material을 노출하지 않는 고정 형식의 오류 문자열을 반환한다.
func (e *Error) Error() string {
	if e == nil {
		return ErrInvalidProvider.Error()
	}
	kind := e.Kind
	if kind == nil {
		kind = ErrInvalidProvider
	}
	if e.Operation == "" {
		return kind.Error()
	}
	return fmt.Sprintf("%v: %s", kind, e.Operation)
}

// Unwrap - 원인 오류를 반환해 context 및 SDK sentinel matching을 지원한다.
func (e *Error) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Cause
}

// Is - provider sentinel과 wrapping된 원인의 errors.Is matching을 지원한다.
func (e *Error) Is(target error) bool {
	if e == nil {
		return false
	}
	return target == e.Kind || errors.Is(e.Kind, target) || errors.Is(e.Cause, target)
}

func errorWith(kind error, operation string, cause error) *Error {
	return &Error{Kind: kind, Operation: operation, Cause: cause}
}
