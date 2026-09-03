package auth

import (
	"errors"
	"fmt"
)

var (
	// ErrInvalidRequest - endpoint, region 또는 username이 유효하지 않을 때 반환된다.
	ErrInvalidRequest = errors.New("rds/auth: invalid request")
	// ErrNilCredentials - nil 또는 typed-nil credentials provider를 전달했을 때 반환된다.
	ErrNilCredentials = errors.New("rds/auth: credentials provider must not be nil")
	// ErrBuildFailed - AWS RDS auth token signing이 실패했을 때 반환된다.
	ErrBuildFailed = errors.New("rds/auth: auth token build failed")
	// ErrMalformedToken - SDK가 빈 token을 반환했을 때 반환된다.
	ErrMalformedToken = errors.New("rds/auth: malformed auth token")
)

// Error - request 값, credentials와 provider payload를 노출하지 않는 오류다.
type Error struct {
	kind      error
	operation string
	cause     error
}

// Error는 고정 sentinel과 안전한 operation만 문자열로 반환한다.
func (e *Error) Error() string {
	if e == nil {
		return ErrInvalidRequest.Error()
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
		ErrInvalidRequest,
		ErrNilCredentials,
		ErrBuildFailed,
		ErrMalformedToken,
	} {
		if errors.Is(kind, sentinel) {
			return sentinel
		}
	}
	return ErrInvalidRequest
}

func safeOperation(operation string) string {
	switch operation {
	case "validate request", "validate credentials", "build", "validate token":
		return operation
	default:
		return ""
	}
}
