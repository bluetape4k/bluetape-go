package graph

import (
	"errors"
	"fmt"
)

var (
	// ErrInvalidElementID reports an empty or unsupported graph element ID.
	ErrInvalidElementID = errors.New("invalid graph element id")
	// ErrInvalidLabel reports an empty graph label.
	ErrInvalidLabel = errors.New("invalid graph label")
	// ErrInvalidVertex reports an invalid vertex value.
	ErrInvalidVertex = errors.New("invalid graph vertex")
	// ErrInvalidEdge reports an invalid edge value.
	ErrInvalidEdge = errors.New("invalid graph edge")
	// ErrInvalidPath reports an invalid path or path step value.
	ErrInvalidPath = errors.New("invalid graph path")
	// ErrUnsupportedCapability is reserved for future graph I/O and backend boundaries.
	ErrUnsupportedCapability = errors.New("unsupported graph capability")
)

// ValidationError describes a graph validation failure without retaining raw values.
type ValidationError struct {
	Kind    error
	Field   string
	Summary string
	Cause   error
}

// Error returns a redacted validation message.
func (e *ValidationError) Error() string {
	if e == nil {
		return "<nil>"
	}
	if e.Field == "" {
		if e.Summary == "" {
			return e.kindText()
		}
		return fmt.Sprintf("%s: %s", e.kindText(), e.Summary)
	}
	if e.Summary == "" {
		return fmt.Sprintf("%s: field %s", e.kindText(), e.Field)
	}
	return fmt.Sprintf("%s: field %s: %s", e.kindText(), e.Field, e.Summary)
}

// Unwrap exposes the sentinel kind and optional cause to errors.Is/As.
func (e *ValidationError) Unwrap() []error {
	if e == nil {
		return nil
	}
	errs := make([]error, 0, 2)
	if e.Kind != nil {
		errs = append(errs, e.Kind)
	}
	if e.Cause != nil {
		errs = append(errs, e.Cause)
	}
	return errs
}

func (e *ValidationError) kindText() string {
	if e.Kind == nil {
		return "graph validation error"
	}
	return e.Kind.Error()
}

func validationError(kind error, field string, summary string, cause error) error {
	return &ValidationError{
		Kind:    kind,
		Field:   field,
		Summary: summary,
		Cause:   cause,
	}
}
