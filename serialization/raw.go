package serialization

import (
	"fmt"
	"unicode/utf8"

	"github.com/bluetape4k/bluetape-go/core"
)

// BytesSerializer copies byte slices without transforming them.
type BytesSerializer struct{}

// Format returns the stable serializer format name.
func (BytesSerializer) Format() string {
	return "bytes"
}

// Marshal returns a copy of value.
func (BytesSerializer) Marshal(value []byte) ([]byte, error) {
	copied := make([]byte, len(value))
	copy(copied, value)
	return copied, nil
}

// Unmarshal returns a copy of data.
func (BytesSerializer) Unmarshal(data []byte) ([]byte, error) {
	if data == nil {
		return nil, fmt.Errorf("unmarshal bytes: input must not be nil")
	}
	copied := make([]byte, len(data))
	copy(copied, data)
	return copied, nil
}

// StringSerializer serializes strings as UTF-8 bytes.
type StringSerializer struct{}

// Format returns the stable serializer format name.
func (StringSerializer) Format() string {
	return "string"
}

// Marshal serializes value as UTF-8 bytes.
//
// It returns an error wrapping core.ErrInvalidUTF8 when value is not valid UTF-8.
func (StringSerializer) Marshal(value string) ([]byte, error) {
	if !utf8.ValidString(value) {
		return nil, fmt.Errorf("marshal string: %w", core.ErrInvalidUTF8)
	}
	return []byte(value), nil
}

// Unmarshal deserializes UTF-8 bytes into a string.
//
// It returns an error wrapping core.ErrInvalidUTF8 when data is not valid UTF-8.
func (StringSerializer) Unmarshal(data []byte) (string, error) {
	if data == nil {
		return "", fmt.Errorf("unmarshal string: input must not be nil")
	}
	if !utf8.Valid(data) {
		return "", fmt.Errorf("unmarshal string: %w", core.ErrInvalidUTF8)
	}
	return string(data), nil
}
