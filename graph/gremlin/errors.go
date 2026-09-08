package gremlin

import (
	"errors"
	"fmt"
)

var (
	// ErrInvalidOptions gremlin remote adapter의 생성 옵션이 유효하지 않음을 뜻한다.
	ErrInvalidOptions = errors.New("graph/gremlin: invalid options")
	// ErrInvalidQuery gremlin remote adapter의 traversal이 비어 있거나 상한을 넘었음을 뜻한다.
	ErrInvalidQuery = errors.New("graph/gremlin: invalid query")
	// ErrInvalidResult gremlin remote adapter의 결과 shape가 graph contract와 맞지 않음을 뜻한다.
	ErrInvalidResult = errors.New("graph/gremlin: invalid result")
	// ErrProvider gremlin remote provider 또는 result stream이 실패했음을 뜻한다.
	ErrProvider = errors.New("graph/gremlin: provider error")
	// ErrUnsupportedCapability gremlin remote adapter가 해당 기능을 의도적으로 제공하지 않음을 뜻한다.
	ErrUnsupportedCapability = errors.New("graph/gremlin: unsupported capability")
	// ErrClosed gremlin remote adapter가 이미 닫혔음을 뜻한다.
	ErrClosed = errors.New("graph/gremlin: client closed")
)

// Error gremlin remote adapter의 redacted 오류와 안정적인 분류를 보관한다.
type Error struct {
	Kind      error
	Operation string
	Cause     error
}

// Error gremlin remote adapter 오류의 안전한 문자열을 반환한다.
func (e *Error) Error() string {
	if e == nil {
		return ErrInvalidOptions.Error()
	}
	kind := e.Kind
	if kind == nil {
		kind = ErrInvalidOptions
	}
	if e.Operation == "" {
		return kind.Error()
	}
	return fmt.Sprintf("%v: %s", kind, e.Operation)
}

// Unwrap gremlin remote adapter의 원인 chain을 유지한다.
func (e *Error) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Cause
}

// Is gremlin remote adapter sentinel과 wrapped 원인을 분류한다.
func (e *Error) Is(target error) bool {
	if e == nil {
		return false
	}
	return target == e.Kind || errors.Is(e.Kind, target) || errors.Is(e.Cause, target)
}

func classified(kind error, operation string, cause error) error {
	return &Error{Kind: kind, Operation: operation, Cause: cause}
}

func invalid(operation string) error {
	return classified(ErrInvalidOptions, operation, nil)
}
