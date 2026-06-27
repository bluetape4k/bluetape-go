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
	ErrMixedAggregate     = errors.New("mixed aggregate")
	ErrRevisionConflict   = errors.New("revision conflict")
)

// ValidationError reports a field-specific validation failure.
type ValidationError struct {
	Kind  error
	Field string
	Value any
	Cause error
}

// Error returns a stable validation error message.
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

// Unwrap returns the underlying validation cause.
func (e ValidationError) Unwrap() error {
	return e.Cause
}

// Is allows errors.Is checks against the package sentinel error.
func (e ValidationError) Is(target error) bool {
	return target == e.Kind
}

func validationError(kind error, field string, value any) error {
	return ValidationError{Kind: kind, Field: field, Value: value}
}

func validationCause(kind error, field string, value any, cause error) error {
	return ValidationError{Kind: kind, Field: field, Value: value, Cause: cause}
}
