package sqlkit

import (
	"database/sql/driver"
	"encoding/json"
)

// DefaultJSONColumnMaxBytes is the default maximum JSON source or output size.
const DefaultJSONColumnMaxBytes = 1 << 20

// JSONColumn stores a typed JSON value and its SQL NULL state.
//
// The zero value represents SQL NULL. MaxBytes uses
// DefaultJSONColumnMaxBytes when it is zero. JSON literal null remains a valid
// JSON value and is distinct from SQL NULL.
type JSONColumn[T any] struct {
	Data     T
	Valid    bool
	MaxBytes int
}

// Scan decodes a nil, string, or []byte database value into a fresh T.
//
// Scan copies driver-owned bytes, clears the previous value before decoding,
// and publishes Data only after decoding succeeds.
func (c *JSONColumn[T]) Scan(src any) (err error) {
	if c == nil {
		return newColumnError(ErrInvalidColumnValue, "scan JSON", nil)
	}
	var zero T
	c.Data, c.Valid = zero, false
	defer recoverColumnPanic("scan JSON", &err)

	raw, present, err := copiedColumnSource(src, "scan JSON")
	if err != nil || !present {
		return err
	}
	limit, err := effectiveColumnLimit(c.MaxBytes, DefaultJSONColumnMaxBytes, "scan JSON limit")
	if err != nil {
		return err
	}
	if len(raw) > limit {
		return newColumnError(ErrColumnValueTooLarge, "scan JSON", nil)
	}

	var decoded T
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return newColumnError(ErrInvalidColumnValue, "scan JSON", err)
	}
	c.Data, c.Valid = decoded, true
	return nil
}

// Value encodes Data as JSON or returns nil when Valid is false.
//
// Value returns []byte for non-NULL values and never exposes callback causes
// through its error string.
func (c JSONColumn[T]) Value() (value driver.Value, err error) {
	if !c.Valid {
		return nil, nil
	}
	defer recoverColumnPanic("encode JSON", &err)

	limit, err := effectiveColumnLimit(c.MaxBytes, DefaultJSONColumnMaxBytes, "encode JSON limit")
	if err != nil {
		return nil, err
	}
	raw, err := json.Marshal(c.Data)
	if err != nil {
		return nil, newColumnError(ErrInvalidColumnValue, "encode JSON", err)
	}
	if len(raw) > limit {
		return nil, newColumnError(ErrColumnValueTooLarge, "encode JSON", nil)
	}
	return raw, nil
}
