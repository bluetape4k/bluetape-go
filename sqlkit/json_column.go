package sqlkit

import (
	"database/sql/driver"
	"encoding/json"
)

// DefaultJSONColumnMaxBytes JSON source 또는 output size의 기본 최대값이다.
const DefaultJSONColumnMaxBytes = 1 << 20

// JSONColumn은 typed JSON value와 SQL NULL 상태를 함께 저장한다.
//
// zero value는 SQL NULL을 나타낸다. MaxBytes가 0이면 DefaultJSONColumnMaxBytes를 사용한다.
// JSON literal null은 유효한 JSON value이며 SQL NULL과 구분된다.
type JSONColumn[T any] struct {
	Data     T
	Valid    bool
	MaxBytes int
}

// Scan은 nil, string, []byte database value를 새로운 T 값으로 decode한다.
//
// Scan은 driver가 소유한 byte를 복사하고 decode 전에 이전 값을 지우며, decode가 성공한 뒤에만 Data를 공개한다.
func (c *JSONColumn[T]) Scan(src any) (err error) {
	if c == nil {
		return newColumnError(ErrInvalidColumnValue, "scan JSON", nil)
	}
	var zero T
	c.Data, c.Valid = zero, false
	defer recoverColumnPanic("scan JSON", &err)

	if src == nil {
		return nil
	}
	limit, err := effectiveColumnLimit(c.MaxBytes, DefaultJSONColumnMaxBytes, "scan JSON limit")
	if err != nil {
		return err
	}
	raw, present, err := boundedCopiedColumnSource(src, limit, "scan JSON")
	if err != nil || !present {
		return err
	}

	var decoded T
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return newColumnError(ErrInvalidColumnValue, "scan JSON", err)
	}
	c.Data, c.Valid = decoded, true
	return nil
}

// Value Data를 JSON으로 encode하거나 Valid가 false이면 nil을 반환한다.
//
// Value non-NULL 값에 대해 []byte를 반환하며 error 문자열에 callback 원인을 노출하지 않는다.
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
