package graph

import (
	"errors"
	"fmt"
)

var (
	// ErrInvalidElementID는 graph IO Neo4j backend에서 반환값과 오류 의미를 설명한다.
	ErrInvalidElementID = errors.New("invalid graph element id")
	// ErrInvalidLabel는 graph IO Neo4j backend에서 반환값과 오류 의미를 설명한다.
	ErrInvalidLabel = errors.New("invalid graph label")
	// ErrInvalidVertex는 graph IO Neo4j backend에서 반환값과 오류 의미를 설명한다.
	ErrInvalidVertex = errors.New("invalid graph vertex")
	// ErrInvalidEdge는 graph IO Neo4j backend에서 반환값과 오류 의미를 설명한다.
	ErrInvalidEdge = errors.New("invalid graph edge")
	// ErrInvalidPath는 graph IO Neo4j backend에서 반환값과 오류 의미를 설명한다.
	ErrInvalidPath = errors.New("invalid graph path")
	// ErrUnsupportedCapability는 graph IO Neo4j backend에서 caller-visible 상태와 의미를 설명한다.
	ErrUnsupportedCapability = errors.New("unsupported graph capability")
)

// ValidationError는 graph IO Neo4j backend에서 동작과 caller-visible 계약을 설명한다.
type ValidationError struct {
	Kind    error
	Field   string
	Summary string
	Cause   error
}

// Error는 graph IO Neo4j backend에서 반환값과 오류 의미를 설명한다.
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

// Unwrap는 graph IO Neo4j backend에서 제공하는 기능과 사용 경계를 설명한다.
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
