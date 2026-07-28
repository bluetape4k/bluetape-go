package neo4j

import (
	"errors"
	"fmt"
)

var (
	// ErrInvalidOptions graph IO Neo4j backend에서 반환값과 오류 의미를 설명한다.
	ErrInvalidOptions = errors.New("graph/neo4j: invalid options")
	// ErrInvalidRecord graph IO Neo4j backend에서 반환값과 오류 의미를 설명한다.
	ErrInvalidRecord = errors.New("graph/neo4j: invalid record")
	// ErrDriver graph IO Neo4j backend에서 반환값과 오류 의미를 설명한다.
	ErrDriver = errors.New("graph/neo4j: driver error")
)

// Error graph IO Neo4j backend에서 동작과 caller-visible 계약을 설명한다.
// 이 주석은 graph IO Neo4j backend의 backend 요구사항, cancellation, timeout, 오류 처리 세부사항을 설명한다.
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

// Unwrap graph IO Neo4j backend에서 제공하는 기능과 사용 경계를 설명한다.
func (e *Error) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Cause
}

// Is graph IO Neo4j backend에서 반환값과 오류 의미를 설명한다.
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
