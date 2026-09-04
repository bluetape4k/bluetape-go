package stepfunctions

import (
	"errors"
	"fmt"
)

var (
	// ErrNilClient - client가 nil 또는 typed-nil일 때 반환된다.
	ErrNilClient = errors.New("stepfunctions: client must not be nil")
	// ErrInvalidOptions - bridge 옵션이 유효하지 않을 때 반환된다.
	ErrInvalidOptions = errors.New("stepfunctions: invalid options")
	// ErrInvalidRequest - 요청이 API 계약을 충족하지 않을 때 반환된다.
	ErrInvalidRequest = errors.New("stepfunctions: invalid request")
	// ErrInputTooLarge - execution input이 설정된 한도를 넘을 때 반환된다.
	ErrInputTooLarge = errors.New("stepfunctions: input exceeds limit")
	// ErrInvalidName - execution name이 허용된 문자·길이를 벗어날 때 반환된다.
	ErrInvalidName = errors.New("stepfunctions: invalid execution name")
	// ErrStartFailed - StartExecution transport 호출이 실패했을 때 반환된다.
	ErrStartFailed = errors.New("stepfunctions: start execution failed")
	// ErrDescribeFailed - DescribeExecution transport 호출이 실패했을 때 반환된다.
	ErrDescribeFailed = errors.New("stepfunctions: describe execution failed")
	// ErrStopFailed - StopExecution transport 호출이 실패했을 때 반환된다.
	ErrStopFailed = errors.New("stepfunctions: stop execution failed")
	// ErrStopUnsupported - 주입된 client가 StopExecution을 제공하지 않을 때 반환된다.
	ErrStopUnsupported = errors.New("stepfunctions: stop execution is unsupported")
	// ErrMalformedOutput - SDK response가 필수 필드를 누락했을 때 반환된다.
	ErrMalformedOutput = errors.New("stepfunctions: malformed response")
	// ErrUnknownStatus - SDK가 알 수 없는 execution status를 반환했을 때 반환된다.
	ErrUnknownStatus = errors.New("stepfunctions: unknown execution status")
	// ErrExecutionFailed - execution이 FAILED 상태로 종료됐을 때 반환된다.
	ErrExecutionFailed = errors.New("stepfunctions: execution failed")
	// ErrExecutionTimedOut - execution이 TIMED_OUT 상태로 종료됐을 때 반환된다.
	ErrExecutionTimedOut = errors.New("stepfunctions: execution timed out")
	// ErrExecutionAborted - execution이 ABORTED 상태로 종료됐을 때 반환된다.
	ErrExecutionAborted = errors.New("stepfunctions: execution aborted")
	// ErrWaitTimeout - bridge가 소유한 wait timeout이 만료됐을 때 반환된다.
	ErrWaitTimeout = errors.New("stepfunctions: wait timed out")
)

// Error - safe sentinel과 고정 operation/status만 표현하는 Step Functions 오류다.
// AWS response text, ARN, payload, credential, trace header는 Error 문자열에 포함하지 않는다.
type Error struct {
	kind      error
	operation string
	status    ExecutionStatus
	cause     error
}

// Error - provider가 반환한 민감한 값을 노출하지 않는 오류 문자열을 반환한다.
func (e *Error) Error() string {
	if e == nil {
		return ErrInvalidOptions.Error()
	}
	kind := safeKind(e.kind)
	operation := safeOperation(e.operation)
	if operation == "" {
		return kind.Error()
	}
	if e.status != "" && isKnownStatus(e.status) {
		return fmt.Sprintf("%v: %s (%s)", kind, operation, e.status)
	}
	return fmt.Sprintf("%v: %s", kind, operation)
}

// Unwrap - caller가 주입한 transport 또는 context 원인을 errors.Is로 확인하게 한다.
func (e *Error) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.cause
}

// Is - package sentinel과 wrapping된 원인의 errors.Is matching을 지원한다.
func (e *Error) Is(target error) bool {
	if e == nil {
		return false
	}
	kind := safeKind(e.kind)
	if target == kind || errors.Is(kind, target) || errors.Is(e.cause, target) {
		return true
	}
	return target == ErrInvalidRequest && isRequestValidationKind(kind)
}

// Status - 오류와 연관된 execution status를 반환한다.
func (e *Error) Status() ExecutionStatus {
	if e == nil {
		return ""
	}
	return e.status
}

// Operation - 오류의 allowlist operation label을 반환한다.
func (e *Error) Operation() string {
	if e == nil {
		return ""
	}
	return safeOperation(e.operation)
}

func newError(kind error, operation string, status ExecutionStatus, cause error) *Error {
	return &Error{
		kind:      safeKind(kind),
		operation: safeOperation(operation),
		status:    safeStatus(status),
		cause:     cause,
	}
}

func safeKind(kind error) error {
	for _, sentinel := range []error{
		ErrNilClient,
		ErrInvalidOptions,
		ErrInvalidRequest,
		ErrInputTooLarge,
		ErrInvalidName,
		ErrStartFailed,
		ErrDescribeFailed,
		ErrStopFailed,
		ErrStopUnsupported,
		ErrMalformedOutput,
		ErrUnknownStatus,
		ErrExecutionFailed,
		ErrExecutionTimedOut,
		ErrExecutionAborted,
		ErrWaitTimeout,
	} {
		if errors.Is(kind, sentinel) {
			return sentinel
		}
	}
	return ErrInvalidOptions
}

func isRequestValidationKind(kind error) bool {
	return errors.Is(kind, ErrInvalidRequest) || errors.Is(kind, ErrInputTooLarge) || errors.Is(kind, ErrInvalidName)
}

func safeOperation(operation string) string {
	switch operation {
	case "validate options", "validate request", "start", "describe", "stop", "wait":
		return operation
	default:
		return ""
	}
}

func safeStatus(status ExecutionStatus) ExecutionStatus {
	if isKnownStatus(status) {
		return status
	}
	return ""
}
