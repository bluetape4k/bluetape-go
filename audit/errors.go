package audit

import (
	"errors"
	"fmt"
)

// Sentinel errors used with errors.Is.
var (
	ErrInvalidAggregateID = errors.New("invalid aggregate id")
	ErrInvalidRevision    = errors.New("invalid revision")
	ErrInvalidEvent       = errors.New("invalid audit event")
	ErrInvalidEntry       = errors.New("invalid audit entry")
	ErrInvalidQuery       = errors.New("invalid audit query")
	ErrMixedAggregate     = errors.New("mixed aggregate")
	ErrRevisionConflict   = errors.New("revision conflict")
)

// ValidationError audit entry, event, repository, recorder, history에서 사용하는 구조체다.
type ValidationError struct {
	Kind  error
	Field string
	Value any
	Cause error
}

// Error 오류 메시지를 반환한다.
func (e ValidationError) Error() string {
	if e.Field == "" {
		if e.Cause != nil {
			return fmt.Sprintf("%v: %v", e.Kind, e.Cause)
		}
		return e.Kind.Error()
	}
	if e.Cause != nil {
		return fmt.Sprintf("%v: field=%s value=redacted: %v", e.Kind, e.Field, e.Cause)
	}
	return fmt.Sprintf("%v: field=%s value=redacted", e.Kind, e.Field)
}

// Unwrap 감싼 원인 오류를 반환한다.
//
// 반환 오류는 입력 검증 실패, context 취소, transaction 실패, repository/outbox 실패, package sentinel error와 typed error를 그대로 드러낸다.
func (e ValidationError) Unwrap() error {
	return e.Cause
}

// Is errors.Is 비교를 지원한다.
//
// 매개변수:
//   - target: Is에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
func (e ValidationError) Is(target error) bool {
	return target == e.Kind
}

func validationError(kind error, field string, value any) error {
	return ValidationError{Kind: kind, Field: field, Value: value}
}

func validationCause(kind error, field string, value any, cause error) error {
	return ValidationError{Kind: kind, Field: field, Value: value, Cause: cause}
}
