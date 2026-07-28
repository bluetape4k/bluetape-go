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

// ValidationError struct 공개 타입이며 audit entry, event, repository, recorder, history 계약을 보존한다.
// 필드와 zero value, nil 허용 여부, 동시성/transaction 소유권은 생성자와 메서드의 한국어 주석 및 테스트 계약을 따른다.
type ValidationError struct {
	Kind  error
	Field string
	Value any
	Cause error
}

// Error Error 공개 API의 동작을 수행하며 audit entry, event, repository, recorder, history 계약을 보존한다.
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

// Unwrap Unwrap 공개 API의 동작을 수행하며 audit entry, event, repository, recorder, history 계약을 보존한다.
//
// 반환 오류는 입력 검증 실패, 취소, transaction 실패, repository/outbox 실패, 또는 package sentinel/typed error 계약을 보존한다.
func (e ValidationError) Unwrap() error {
	return e.Cause
}

// Is Is 공개 API의 동작을 수행하며 audit entry, event, repository, recorder, history 계약을 보존한다.
//
// 매개변수:
//   - target: Is 동작에 필요한 target 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
func (e ValidationError) Is(target error) bool {
	return target == e.Kind
}

func validationError(kind error, field string, value any) error {
	return ValidationError{Kind: kind, Field: field, Value: value}
}

func validationCause(kind error, field string, value any, cause error) error {
	return ValidationError{Kind: kind, Field: field, Value: value, Cause: cause}
}
