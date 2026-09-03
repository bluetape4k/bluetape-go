package s3vectors

import (
	"errors"
	"fmt"
)

var (
	// ErrNilClient 는 New가 nil 또는 typed-nil client를 받았을 때 반환된다.
	ErrNilClient = errors.New("s3vectors: client must not be nil")
	// ErrInvalidProvider 는 zero-value 또는 사용할 수 없는 Provider일 때 반환된다.
	ErrInvalidProvider = errors.New("s3vectors: invalid provider")
	// ErrInvalidRequest 는 input이 안전한 preflight validation을 통과하지 못했을
	// 때 반환된다.
	ErrInvalidRequest = errors.New("s3vectors: invalid request")
	// ErrOperationFailed 는 AWS SDK operation이 오류를 반환했을 때 사용된다.
	ErrOperationFailed = errors.New("s3vectors: operation failed")
	// ErrMalformedOutput 은 SDK output이 nil이거나 required field를 잃었을 때
	// 사용된다.
	ErrMalformedOutput = errors.New("s3vectors: malformed output")
)

// Error 는 provider message나 request 값을 Error/formatted output에 노출하지
// 않고 operation failure를 감싼다. 원인은 Unwrap을 통해 errors.Is와
// errors.As로 확인할 수 있다.
type Error struct {
	kind      error
	operation string
	cause     error
}

// Error는 안정적인 redacted package error string을 반환한다.
func (e *Error) Error() string {
	if e == nil {
		return ErrInvalidProvider.Error()
	}
	kind := safeKind(e.kind)
	operation := safeOperation(e.operation)
	if operation == "" {
		return kind.Error()
	}
	return fmt.Sprintf("%v: %s", kind, operation)
}

// GoString은 %#v 형식에서도 provider cause와 request 세부정보를 숨긴다.
func (e *Error) GoString() string {
	return e.Error()
}

// Unwrap은 caller가 errors.Is/errors.As로 확인할 수 있는 원인을 반환한다.
func (e *Error) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.cause
}

// Is 는 package sentinel과 wrapping한 provider cause의 matching을 지원한다.
func (e *Error) Is(target error) bool {
	if e == nil {
		return false
	}
	return target == safeKind(e.kind) || errors.Is(e.cause, target)
}

func newError(kind error, operation string, cause error) *Error {
	return &Error{kind: safeKind(kind), operation: safeOperation(operation), cause: cause}
}

func safeKind(kind error) error {
	for _, sentinel := range []error{ErrNilClient, ErrInvalidProvider, ErrInvalidRequest, ErrOperationFailed, ErrMalformedOutput} {
		if errors.Is(kind, sentinel) {
			return sentinel
		}
	}
	return ErrInvalidProvider
}

func safeOperation(operation string) string {
	switch operation {
	case "validate provider", "validate request", "list vector buckets", "get vector bucket", "list indexes", "get index", "put vectors", "get vectors", "list vectors", "query vectors":
		return operation
	default:
		return ""
	}
}
