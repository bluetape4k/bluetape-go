package ssm

import (
	"errors"
	"fmt"
)

var (
	// ErrNilClient - nil 또는 typed-nil AWS client를 전달했을 때 반환된다.
	ErrNilClient = errors.New("ssm: client must not be nil")
	// ErrInvalidOptions - provider option이 유효하지 않을 때 반환된다.
	ErrInvalidOptions = errors.New("ssm: invalid options")
	// ErrInvalidName - parameter name이 비어 있거나 AWS 길이 상한을 넘을 때 반환된다.
	ErrInvalidName = errors.New("ssm: name is invalid")
	// ErrLookupFailed - Parameter Store 호출이 실패했을 때 반환된다.
	ErrLookupFailed = errors.New("ssm: lookup failed")
	// ErrMalformedOutput - SDK 응답의 필수 구조가 유효하지 않을 때 반환된다.
	ErrMalformedOutput = errors.New("ssm: malformed response")
	// ErrMissingValue - SDK 응답에 parameter value가 없을 때 반환된다.
	ErrMissingValue = errors.New("ssm: value is missing")
	// ErrCacheFailed - caller-owned cache 호출이 실패했을 때 반환된다.
	ErrCacheFailed = errors.New("ssm: cache operation failed")
)

// Error - parameter name과 provider payload를 노출하지 않는 오류다.
type Error struct {
	kind      error
	operation string
	cause     error
}

// Error는 고정된 sentinel과 안전한 operation만 문자열로 반환한다.
func (e *Error) Error() string {
	if e == nil {
		return ErrInvalidOptions.Error()
	}
	kind := safeKind(e.kind)
	operation := safeOperation(e.operation)
	if operation == "" {
		return kind.Error()
	}
	return fmt.Sprintf("%v: %s", kind, operation)
}

// Unwrap는 caller가 원인에 대해 errors.Is/errors.As를 사용할 수 있게 한다.
func (e *Error) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.cause
}

// Is - provider sentinel과 wrapping된 원인의 matching을 지원한다.
func (e *Error) Is(target error) bool {
	if e == nil {
		return false
	}
	kind := safeKind(e.kind)
	return target == kind || errors.Is(kind, target) || errors.Is(e.cause, target)
}

// Operation - 안전하게 허용된 operation label을 반환한다.
func (e *Error) Operation() string {
	if e == nil {
		return ""
	}
	return safeOperation(e.operation)
}

func newError(kind error, operation string, cause error) *Error {
	return &Error{kind: safeKind(kind), operation: safeOperation(operation), cause: cause}
}

func safeKind(kind error) error {
	for _, sentinel := range []error{
		ErrNilClient,
		ErrInvalidOptions,
		ErrInvalidName,
		ErrLookupFailed,
		ErrMalformedOutput,
		ErrMissingValue,
		ErrCacheFailed,
	} {
		if errors.Is(kind, sentinel) {
			return sentinel
		}
	}
	return ErrInvalidOptions
}

func safeOperation(operation string) string {
	switch operation {
	case "validate options", "validate name", "lookup", "cache":
		return operation
	default:
		return ""
	}
}
