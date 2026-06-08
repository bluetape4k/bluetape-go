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

// OptionError wraps an invalid option with its sentinel category.
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

// Is reports whether the error matches ErrInvalidOptions or its wrapped cause.
func (e OptionError) Is(target error) bool {
	return target == ErrInvalidOptions || errors.Is(e.Err, target)
}

// ParseError wraps dependency parse failures without exposing dependency types.
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

// Is reports whether the error matches ErrInvalidID or its wrapped cause.
func (e ParseError) Is(target error) bool {
	return target == ErrInvalidID || errors.Is(e.Err, target)
}

// EntropyError wraps reader failures from UUID, ULID, and KSUID entropy sources.
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

// Is reports whether the error matches ErrEntropy or its wrapped cause.
func (e EntropyError) Is(target error) bool {
	return target == ErrEntropy || errors.Is(e.Err, target)
}

// ClockRollbackError reports a Snowflake timestamp moving backwards.
type ClockRollbackError struct {
	Last int64
	Now  int64
}

func (e ClockRollbackError) Error() string {
	return fmt.Sprintf("%v: last=%d now=%d", ErrClockRollback, e.Last, e.Now)
}

// Is reports whether the error matches ErrClockRollback.
func (e ClockRollbackError) Is(target error) bool {
	return target == ErrClockRollback
}

// SequenceExhaustedError reports sequence exhaustion for a single millisecond.
type SequenceExhaustedError struct {
	Millis int64
}

func (e SequenceExhaustedError) Error() string {
	return fmt.Sprintf("%v: millis=%d", ErrSequenceExhausted, e.Millis)
}

// Is reports whether the error matches ErrSequenceExhausted.
func (e SequenceExhaustedError) Is(target error) bool {
	return target == ErrSequenceExhausted
}
