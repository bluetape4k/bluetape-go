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

// OptionError 패키지에서 공개하는 구조체다.
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

// Is errors.Is 비교를 지원한다.
//
// 매개변수:
//   - target: 검사하거나 감쌀 오류 값이다.
func (e OptionError) Is(target error) bool {
	return target == ErrInvalidOptions || errors.Is(e.Err, target)
}

// ParseError 패키지에서 공개하는 구조체다.
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

// Is errors.Is 비교를 지원한다.
//
// 매개변수:
//   - target: 검사하거나 감쌀 오류 값이다.
func (e ParseError) Is(target error) bool {
	return target == ErrInvalidID || errors.Is(e.Err, target)
}

// EntropyError 패키지에서 공개하는 구조체다.
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

// Is errors.Is 비교를 지원한다.
//
// 매개변수:
//   - target: 검사하거나 감쌀 오류 값이다.
func (e EntropyError) Is(target error) bool {
	return target == ErrEntropy || errors.Is(e.Err, target)
}

// ClockRollbackError 패키지에서 공개하는 구조체다.
type ClockRollbackError struct {
	Last int64
	Now  int64
}

func (e ClockRollbackError) Error() string {
	return fmt.Sprintf("%v: last=%d now=%d", ErrClockRollback, e.Last, e.Now)
}

// Is errors.Is 비교를 지원한다.
//
// 매개변수:
//   - target: 검사하거나 감쌀 오류 값이다.
func (e ClockRollbackError) Is(target error) bool {
	return target == ErrClockRollback
}

// SequenceExhaustedError 패키지에서 공개하는 구조체다.
type SequenceExhaustedError struct {
	Millis int64
}

func (e SequenceExhaustedError) Error() string {
	return fmt.Sprintf("%v: millis=%d", ErrSequenceExhausted, e.Millis)
}

// Is errors.Is 비교를 지원한다.
//
// 매개변수:
//   - target: 검사하거나 감쌀 오류 값이다.
func (e SequenceExhaustedError) Is(target error) bool {
	return target == ErrSequenceExhausted
}
