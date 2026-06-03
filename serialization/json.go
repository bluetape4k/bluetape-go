package serialization

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
)

// JSONOption configures JSONSerializer.
type JSONOption func(*JSONSerializerOptions)

// JSONSerializerOptions controls JSON decoding behavior.
type JSONSerializerOptions struct {
	DisallowUnknownFields bool
}

// JSONSerializer serializes values with Go's standard encoding/json package.
type JSONSerializer[T any] struct {
	options JSONSerializerOptions
}

// NewJSONSerializer creates a JSON serializer.
func NewJSONSerializer[T any](options ...JSONOption) JSONSerializer[T] {
	serializer := JSONSerializer[T]{}
	for _, option := range options {
		if option != nil {
			option(&serializer.options)
		}
	}
	return serializer
}

// WithDisallowUnknownFields makes JSON decoding reject unknown object fields.
func WithDisallowUnknownFields() JSONOption {
	return func(options *JSONSerializerOptions) {
		options.DisallowUnknownFields = true
	}
}

// Format returns the stable serializer format name.
func (s JSONSerializer[T]) Format() string {
	return "json"
}

// Marshal serializes value as JSON.
func (s JSONSerializer[T]) Marshal(value T) ([]byte, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("marshal json: %w", err)
	}
	return data, nil
}

// Unmarshal deserializes JSON bytes into T.
func (s JSONSerializer[T]) Unmarshal(data []byte) (T, error) {
	var value T
	if len(data) == 0 {
		return value, fmt.Errorf("unmarshal json: input must not be empty")
	}

	decoder := json.NewDecoder(bytes.NewReader(data))
	if s.options.DisallowUnknownFields {
		decoder.DisallowUnknownFields()
	}
	if err := decoder.Decode(&value); err != nil {
		return value, fmt.Errorf("unmarshal json: %w", err)
	}
	var trailing json.RawMessage
	if err := decoder.Decode(&trailing); err != io.EOF {
		return value, fmt.Errorf("unmarshal json: trailing data")
	}
	return value, nil
}
