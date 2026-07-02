package neo4j

import (
	"errors"
	"fmt"
)

var (
	// ErrInvalidOptions reports a nil driver, invalid option, or invalid column name.
	ErrInvalidOptions = errors.New("graph/neo4j: invalid options")
	// ErrInvalidRecord reports a Neo4j record value that cannot be adapted to graph values.
	ErrInvalidRecord = errors.New("graph/neo4j: invalid record")
	// ErrDriver reports a Neo4j driver, session, transaction, or query failure.
	ErrDriver = errors.New("graph/neo4j: driver error")
)

// Error preserves graph/neo4j sentinel identity without retaining raw Cypher,
// parameters, or property values in the rendered message.
type Error struct {
	Kind      error
	Operation string
	Column    string
	Cause     error
}

func (e *Error) Error() string {
	if e == nil {
		return ErrInvalidOptions.Error()
	}
	kind := e.Kind
	if kind == nil {
		kind = ErrInvalidOptions
	}
	if e.Column != "" && e.Operation != "" {
		return fmt.Sprintf("%v: %s: column %s", kind, e.Operation, e.Column)
	}
	if e.Operation != "" {
		return fmt.Sprintf("%v: %s", kind, e.Operation)
	}
	return kind.Error()
}

// Unwrap exposes the optional cause to errors.Is/As.
func (e *Error) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Cause
}

// Is reports matches against graph/neo4j sentinel errors and the wrapped cause.
func (e *Error) Is(target error) bool {
	if e == nil {
		return false
	}
	return target == e.Kind || errors.Is(e.Kind, target) || errors.Is(e.Cause, target)
}

func errorWith(kind error, operation string, cause error) *Error {
	return &Error{Kind: kind, Operation: operation, Cause: cause}
}

func columnError(kind error, operation string, column string, cause error) *Error {
	return &Error{Kind: kind, Operation: operation, Column: column, Cause: cause}
}
