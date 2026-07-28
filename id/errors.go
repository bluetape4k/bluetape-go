package id

import (
	"errors"
	"fmt"
)

var (
	// ErrInvalidOptions reports invalid generator options.
	ErrInvalidOptions = errors.New("id: invalid options")
	// ErrInvalidID reports malformed or out-of-range IDs.
	ErrInvalidID = errors.New("id: invalid id")
	// ErrUnsupportedVersion reports an unsupported ID algorithm or version.
	ErrUnsupportedVersion = errors.New("id: unsupported version")
	// ErrEntropy reports a random entropy source failure.
	ErrEntropy = errors.New("id: entropy failure")
	// ErrClockRollback reports a Snowflake clock rollback.
	ErrClockRollback = errors.New("id: clock rollback")
	// ErrSequenceExhausted reports Snowflake sequence exhaustion in one millisecond.
	ErrSequenceExhausted = errors.New("id: sequence exhausted")
)

// OptionError struct 공개 타입이다.
// 값의 zero value, nil 허용 여부, 동시성 소유권은 생성자와 메서드의 한국어 주석 및 테스트 계약을 따른다.
type OptionError struct {
	Option string
	Err    error
}

func (e OptionError) Error() string {
	if e.Option == "" {
		return fmt.Sprintf("%v: %v", ErrInvalidOptions, e.Err)
	}
	return fmt.Sprintf("%v: %s: %v", ErrInvalidOptions, e.Option, e.Err)
}

func (e OptionError) Unwrap() error { return e.Err }

// Is Is 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - target: 검사하거나 감쌀 오류 값이다.
func (e OptionError) Is(target error) bool {
	return target == ErrInvalidOptions || errors.Is(e.Err, target)
}

// ParseError struct 공개 타입이다.
// 값의 zero value, nil 허용 여부, 동시성 소유권은 생성자와 메서드의 한국어 주석 및 테스트 계약을 따른다.
type ParseError struct {
	Kind  string
	Value string
	Err   error
}

func (e ParseError) Error() string {
	if e.Kind == "" {
		return fmt.Sprintf("%v: %q", ErrInvalidID, e.Value)
	}
	return fmt.Sprintf("%v: %s %q", ErrInvalidID, e.Kind, e.Value)
}

func (e ParseError) Unwrap() error { return e.Err }

// Is Is 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - target: 검사하거나 감쌀 오류 값이다.
func (e ParseError) Is(target error) bool {
	return target == ErrInvalidID || errors.Is(e.Err, target)
}

// EntropyError struct 공개 타입이다.
// 값의 zero value, nil 허용 여부, 동시성 소유권은 생성자와 메서드의 한국어 주석 및 테스트 계약을 따른다.
type EntropyError struct {
	Kind string
	Err  error
}

func (e EntropyError) Error() string {
	if e.Kind == "" {
		return fmt.Sprintf("%v: %v", ErrEntropy, e.Err)
	}
	return fmt.Sprintf("%v: %s: %v", ErrEntropy, e.Kind, e.Err)
}

func (e EntropyError) Unwrap() error { return e.Err }

// Is Is 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - target: 검사하거나 감쌀 오류 값이다.
func (e EntropyError) Is(target error) bool {
	return target == ErrEntropy || errors.Is(e.Err, target)
}

// ClockRollbackError struct 공개 타입이다.
// 값의 zero value, nil 허용 여부, 동시성 소유권은 생성자와 메서드의 한국어 주석 및 테스트 계약을 따른다.
type ClockRollbackError struct {
	Last int64
	Now  int64
}

func (e ClockRollbackError) Error() string {
	return fmt.Sprintf("%v: last=%d now=%d", ErrClockRollback, e.Last, e.Now)
}

// Is Is 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - target: 검사하거나 감쌀 오류 값이다.
func (e ClockRollbackError) Is(target error) bool {
	return target == ErrClockRollback
}

// SequenceExhaustedError struct 공개 타입이다.
// 값의 zero value, nil 허용 여부, 동시성 소유권은 생성자와 메서드의 한국어 주석 및 테스트 계약을 따른다.
type SequenceExhaustedError struct {
	Millis int64
}

func (e SequenceExhaustedError) Error() string {
	return fmt.Sprintf("%v: millis=%d", ErrSequenceExhausted, e.Millis)
}

// Is Is 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - target: 검사하거나 감쌀 오류 값이다.
func (e SequenceExhaustedError) Is(target error) bool {
	return target == ErrSequenceExhausted
}
