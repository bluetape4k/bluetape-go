package sqlkit

import (
	"errors"
	"fmt"
)

var (
	// ErrInvalidColumnValue reports an unsupported, malformed, or invalidly configured column value.
	ErrInvalidColumnValue = errors.New("sqlkit: invalid column value")

	// ErrColumnValueTooLarge reports a column source or encoded value above its byte limit.
	ErrColumnValueTooLarge = errors.New("sqlkit: column value too large")
)

type columnError struct {
	kind      error
	operation string
	cause     error
}

func (e *columnError) Error() string {
	if e == nil {
		return ErrInvalidColumnValue.Error()
	}
	kind := e.kind
	if kind == nil {
		kind = ErrInvalidColumnValue
	}
	if e.operation == "" {
		return kind.Error()
	}
	return fmt.Sprintf("%v: %s", kind, e.operation)
}

func (e *columnError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.cause
}

func (e *columnError) Is(target error) bool {
	return e != nil && (errors.Is(e.kind, target) || errors.Is(e.cause, target))
}

func newColumnError(kind error, operation string, cause error) error {
	return &columnError{kind: kind, operation: operation, cause: cause}
}

func recoverColumnPanic(operation string, errp *error) {
	if recover() != nil {
		*errp = newColumnError(ErrInvalidColumnValue, operation, nil)
	}
}

func effectiveColumnLimit(configured, fallback int, operation string) (int, error) {
	if configured < 0 {
		return 0, newColumnError(ErrInvalidColumnValue, operation, nil)
	}
	if configured == 0 {
		return fallback, nil
	}
	return configured, nil
}

func copiedColumnSource(src any, operation string) ([]byte, bool, error) {
	switch value := src.(type) {
	case nil:
		return nil, false, nil
	case []byte:
		return append([]byte(nil), value...), true, nil
	case string:
		return []byte(value), true, nil
	default:
		return nil, false, newColumnError(ErrInvalidColumnValue, operation, nil)
	}
}
