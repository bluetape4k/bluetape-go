package falkordb

import (
	"errors"
	"fmt"
)

var (
	// ErrInvalidOptions는 client 또는 operation option이 유효하지 않음을 나타낸다.
	ErrInvalidOptions = errors.New("graph/falkordb: invalid options")
	// ErrInvalidQuery는 query, graph name 또는 parameter가 유효하지 않음을 나타낸다.
	ErrInvalidQuery = errors.New("graph/falkordb: invalid query")
	// ErrInvalidResult는 FalkorDB RESP shape가 기대와 다름을 나타낸다.
	ErrInvalidResult = errors.New("graph/falkordb: invalid result")
	// ErrProvider는 Redis/FalkorDB provider 오류를 나타낸다.
	ErrProvider = errors.New("graph/falkordb: provider error")
	// ErrUnsupportedCapability는 이 좁은 adapter가 지원하지 않는 기능을 나타낸다.
	ErrUnsupportedCapability = errors.New("graph/falkordb: unsupported capability")
)

// Error는 provider payload와 graph name을 노출하지 않는 typed 오류다.
type Error struct {
	Kind  error
	Cause error
}

// Error는 안정된 kind 문자열만 반환한다.
func (e *Error) Error() string {
	if e == nil || e.Kind == nil {
		return ErrProvider.Error()
	}
	return e.Kind.Error()
}

// Unwrap는 errors.Is/errors.As용 내부 원인을 보존한다.
func (e *Error) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Cause
}

// Is는 sentinel 분류와 wrapped provider 오류를 지원한다.
func (e *Error) Is(target error) bool {
	if e == nil {
		return false
	}
	return target == e.Kind || errors.Is(e.Kind, target) || errors.Is(e.Cause, target)
}

func classified(kind error, cause error) error {
	if cause == nil {
		return &Error{Kind: kind}
	}
	return &Error{Kind: kind, Cause: cause}
}

func invalid(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrInvalidOptions, fmt.Sprintf(format, args...))
}
