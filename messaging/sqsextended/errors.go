package sqsextended

import (
	"errors"
	"fmt"
)

var (
	// ErrNilClient - nil 또는 typed-nil SQS/S3 client를 나타낸다.
	ErrNilClient = errors.New("sqsextended: client must not be nil")
	// ErrInvalidOptions - 잘못된 Provider 생성 option을 나타낸다.
	ErrInvalidOptions = errors.New("sqsextended: invalid options")
	// ErrInvalidRequest - dispatch할 수 없는 호출자 request 값을 나타낸다.
	ErrInvalidRequest = errors.New("sqsextended: invalid request")
	// ErrInvalidEnvelope - 잘못되었거나 canonical하지 않은 envelope data를 나타낸다.
	ErrInvalidEnvelope = errors.New("sqsextended: invalid envelope")
	// ErrUnsupportedVersion - 이 package가 읽을 수 없는 envelope version을 나타낸다.
	ErrUnsupportedVersion = errors.New("sqsextended: unsupported envelope version")
	// ErrEnvelopeTooLarge - 제한된 wire limit을 초과한 envelope body를 나타낸다.
	ErrEnvelopeTooLarge = errors.New("sqsextended: envelope is too large")
	// ErrPayloadTooLarge - Provider가 구성한 상한을 초과한 payload를 나타낸다.
	ErrPayloadTooLarge = errors.New("sqsextended: payload is too large")
	// ErrPayloadSizeMismatch - envelope metadata와 크기가 다른 payload를 나타낸다.
	ErrPayloadSizeMismatch = errors.New("sqsextended: payload size mismatch")
	// ErrChecksumMismatch - envelope와 SHA-256이 다른 payload를 나타낸다.
	ErrChecksumMismatch = errors.New("sqsextended: payload checksum mismatch")
	// ErrObjectPutFailed - S3 PutObject 실패를 나타낸다.
	ErrObjectPutFailed = errors.New("sqsextended: object put failed")
	// ErrMessageSendFailed - SQS SendMessage 실패를 나타낸다.
	ErrMessageSendFailed = errors.New("sqsextended: message send failed")
	// ErrReceiveFailed - SQS ReceiveMessage 실패를 나타낸다.
	ErrReceiveFailed = errors.New("sqsextended: receive failed")
	// ErrObjectReadFailed - S3 GetObject 또는 response body 실패를 나타낸다.
	ErrObjectReadFailed = errors.New("sqsextended: object read failed")
	// ErrMessageDeleteFailed - SQS DeleteMessage 실패를 나타낸다.
	ErrMessageDeleteFailed = errors.New("sqsextended: message delete failed")
	// ErrObjectDeleteFailed - S3 DeleteObject 실패를 나타낸다.
	ErrObjectDeleteFailed = errors.New("sqsextended: object delete failed")
	// ErrMalformedOutput - nil 또는 불완전한 AWS SDK output을 나타낸다.
	ErrMalformedOutput = errors.New("sqsextended: malformed provider output")
	// ErrCanceled - 외부 side effect 뒤 caller context가 취소되어 결과 상태가
	// 부분적으로만 확인된 경우를 나타낸다. errors.Is로 context 원인을 확인할
	// 수 있으며, OrphanedObject 또는 QueueDeleted 상태를 함께 확인해야 한다.
	ErrCanceled = errors.New("sqsextended: operation canceled")
)

// Error - 민감한 값을 제거한 provider operation error이다.
//
// Provider와 호출자 값, payload byte, checksum과 AWS diagnostic message는
// Error의 formatted representation에서 의도적으로 제외한다. 주입된 cause는
// errors.Is와 errors.As 호출자가 계속 사용할 수 있다.
type Error struct {
	kind         error
	operation    string
	cause        error
	orphaned     bool
	queueDeleted bool
}

// Error - provider data를 포함하지 않고 stable sentinel과 허용된 operation을 반환한다.
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

// GoString은 %#v 형식에서도 provider cause와 request 세부정보를 숨긴다.
func (e *Error) GoString() string {
	return e.Error()
}

// Unwrap은 causal error를 형식화하지 않은 채 errors.Is/errors.As에 노출한다.
func (e *Error) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.cause
}

// Is - package sentinel 또는 주입된 causal error와 일치하는지 확인한다.
func (e *Error) Is(target error) bool {
	if e == nil {
		return false
	}
	return target == safeKind(e.kind) || errors.Is(e.cause, target)
}

// OrphanedObject - SQS send가 실패하거나 사용할 수 없는 response를 반환하기
// 전에 S3 PutObject가 완료되었음을 나타낸다. provider는 해당 object를 암묵적으로
// 삭제하지 않는다.
func (e *Error) OrphanedObject() bool {
	return e != nil && e.orphaned
}

// QueueDeleted - object cleanup이 실패하기 전에 SQS DeleteMessage가 완료되었음을 나타낸다.
func (e *Error) QueueDeleted() bool {
	return e != nil && e.queueDeleted
}

func newError(kind error, operation string, cause error, orphaned, queueDeleted bool) *Error {
	return &Error{
		kind:         safeKind(kind),
		operation:    safeOperation(operation),
		cause:        cause,
		orphaned:     orphaned,
		queueDeleted: queueDeleted,
	}
}

func safeKind(kind error) error {
	for _, sentinel := range []error{
		ErrNilClient,
		ErrInvalidOptions,
		ErrInvalidRequest,
		ErrInvalidEnvelope,
		ErrUnsupportedVersion,
		ErrEnvelopeTooLarge,
		ErrPayloadTooLarge,
		ErrPayloadSizeMismatch,
		ErrChecksumMismatch,
		ErrObjectPutFailed,
		ErrMessageSendFailed,
		ErrReceiveFailed,
		ErrObjectReadFailed,
		ErrMessageDeleteFailed,
		ErrObjectDeleteFailed,
		ErrMalformedOutput,
		ErrCanceled,
	} {
		if errors.Is(kind, sentinel) {
			return sentinel
		}
	}
	return ErrInvalidOptions
}

func safeOperation(operation string) string {
	switch operation {
	case "validate options", "validate request", "encode envelope", "decode envelope", "send", "put object", "send message", "receive", "get object", "read object", "delete", "delete message", "delete object":
		return operation
	default:
		return ""
	}
}
