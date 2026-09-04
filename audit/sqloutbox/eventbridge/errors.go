package eventbridge

import (
	"errors"
	"fmt"
	"regexp"
)

var (
	// ErrNilClient client가 nil 또는 typed-nil일 때 반환된다.
	ErrNilClient = errors.New("eventbridge: client must not be nil")
	// ErrInvalidOptions publisher 생성 옵션이 유효하지 않을 때 반환된다.
	ErrInvalidOptions = errors.New("eventbridge: invalid options")
	// ErrInvalidRecord outbox record가 유효하지 않을 때 반환된다.
	ErrInvalidRecord = errors.New("eventbridge: invalid outbox record")
	// ErrDetailTooLarge detail 또는 EventBridge entry가 크기 한도를 넘을 때 반환된다.
	ErrDetailTooLarge = errors.New("eventbridge: detail exceeds limit")
	// ErrPublishFailed SDK transport 호출이 실패했을 때 반환된다.
	ErrPublishFailed = errors.New("eventbridge: publish failed")
	// ErrPartialFailure EventBridge entry 결과에 실패가 포함될 때 반환된다.
	ErrPartialFailure = errors.New("eventbridge: entry failure")
	// ErrMalformedOutput SDK output 구조가 계약과 다를 때 반환된다.
	ErrMalformedOutput = errors.New("eventbridge: malformed response")
)

var safeErrorCodePattern = regexp.MustCompile(`^[A-Za-z0-9._-]{1,64}$`)

// Error - EventBridge 오류를 안전한 sentinel과 operation으로 표현한다.
//
// AWS response message, detail, bus 이름과 같은 caller/provider 값은 Error
// 문자열에 포함하지 않는다. Cause는 errors.Is로만 관찰할 수 있다.
type Error struct {
	kind         error
	operation    string
	cause        error
	failureCount int32
	errorCode    string
}

// Error는 detail 또는 provider 오류의 민감한 내용을 노출하지 않는 문자열을 반환한다.
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

// Unwrap는 caller가 주입한 transport cause를 errors.Is로 확인할 수 있게 한다.
func (e *Error) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.cause
}

// Is - package sentinel과 sanitised transport cause의 errors.Is matching을 지원한다.
func (e *Error) Is(target error) bool {
	if e == nil {
		return false
	}
	return target == safeKind(e.kind) || errors.Is(e.cause, target)
}

// FailureCount - EventBridge가 보고한 실패 entry 수를 반환한다.
func (e *Error) FailureCount() int32 {
	if e == nil {
		return 0
	}
	return e.failureCount
}

// ErrorCode - allowlist를 통과한 EventBridge 오류 코드를 반환한다.
func (e *Error) ErrorCode() string {
	if e == nil {
		return ""
	}
	return e.errorCode
}

func newError(kind error, operation string, cause error, failureCount int32, errorCode string) *Error {
	if failureCount < 0 {
		failureCount = 0
	}
	return &Error{
		kind:         safeKind(kind),
		operation:    safeOperation(operation),
		cause:        cause,
		failureCount: failureCount,
		errorCode:    safeErrorCode(errorCode),
	}
}

func safeKind(kind error) error {
	for _, sentinel := range []error{
		ErrNilClient,
		ErrInvalidOptions,
		ErrInvalidRecord,
		ErrDetailTooLarge,
		ErrPublishFailed,
		ErrPartialFailure,
		ErrMalformedOutput,
	} {
		if errors.Is(kind, sentinel) {
			return sentinel
		}
	}
	return ErrInvalidOptions
}

func safeOperation(operation string) string {
	switch operation {
	case "publish", "marshal detail", "validate record", "validate options":
		return operation
	default:
		return ""
	}
}

func safeErrorCode(code string) string {
	if !safeErrorCodePattern.MatchString(code) {
		return ""
	}
	return code
}
