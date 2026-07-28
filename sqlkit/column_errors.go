package sqlkit

import (
	"errors"
	"fmt"
)

var (
	// ErrInvalidColumnValue 지원하지 않거나 malformed이거나 잘못 설정된 column value일 때 반환된다.
	ErrInvalidColumnValue = errors.New("sqlkit: invalid column value")

	// ErrColumnValueTooLarge column source 또는 encoded value가 byte 한도를 넘을 때 반환된다.
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

func boundedCopiedColumnSource(src any, limit int, operation string) ([]byte, bool, error) {
	switch value := src.(type) {
	case nil:
		return nil, false, nil
	case []byte:
		if len(value) > limit {
			return nil, true, newColumnError(ErrColumnValueTooLarge, operation, nil)
		}
		return append([]byte(nil), value...), true, nil
	case string:
		if len(value) > limit {
			return nil, true, newColumnError(ErrColumnValueTooLarge, operation, nil)
		}
		return []byte(value), true, nil
	default:
		return nil, false, newColumnError(ErrInvalidColumnValue, operation, nil)
	}
}
