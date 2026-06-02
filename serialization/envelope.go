package serialization

import (
	"encoding/binary"
	"fmt"
)

const (
	envelopeMagic = "BTGS"
	envelopeV1    = 1
)

// VersionedSerializer wraps a named serializer with a small versioned envelope.
type VersionedSerializer[T any] struct {
	serializer NamedSerializer[T]
	version    uint16
}

// NewVersionedSerializer creates a versioned serializer.
func NewVersionedSerializer[T any](serializer NamedSerializer[T], version uint16) (VersionedSerializer[T], error) {
	if serializer == nil {
		return VersionedSerializer[T]{}, fmt.Errorf("serializer must not be nil")
	}
	if serializer.Format() == "" {
		return VersionedSerializer[T]{}, fmt.Errorf("serializer format must not be empty")
	}
	if len(serializer.Format()) > 255 {
		return VersionedSerializer[T]{}, fmt.Errorf("serializer format must fit in 255 bytes")
	}
	if version == 0 {
		return VersionedSerializer[T]{}, fmt.Errorf("serializer version must be positive")
	}

	return VersionedSerializer[T]{
		serializer: serializer,
		version:    version,
	}, nil
}

// Format returns the wrapped serializer format.
func (s VersionedSerializer[T]) Format() string {
	return s.serializer.Format()
}

// Version returns the payload version written by Marshal.
func (s VersionedSerializer[T]) Version() uint16 {
	return s.version
}

// Marshal serializes value and prefixes a versioned envelope.
func (s VersionedSerializer[T]) Marshal(value T) ([]byte, error) {
	payload, err := s.serializer.Marshal(value)
	if err != nil {
		return nil, err
	}

	format := []byte(s.serializer.Format())
	headerLen := len(envelopeMagic) + 2 + 1 + len(format)
	result := make([]byte, headerLen+len(payload))
	copy(result, envelopeMagic)
	binary.BigEndian.PutUint16(result[4:6], s.version)
	result[6] = byte(len(format))
	copy(result[7:], format)
	copy(result[headerLen:], payload)
	return result, nil
}

// Unmarshal validates the envelope and deserializes its payload.
func (s VersionedSerializer[T]) Unmarshal(data []byte) (T, error) {
	var zero T
	payload, err := s.payload(data)
	if err != nil {
		return zero, err
	}
	return s.serializer.Unmarshal(payload)
}

func (s VersionedSerializer[T]) payload(data []byte) ([]byte, error) {
	if len(data) < 7 {
		return nil, fmt.Errorf("%w: header too short", ErrInvalidEnvelope)
	}
	if string(data[:4]) != envelopeMagic {
		return nil, fmt.Errorf("%w: bad magic", ErrInvalidEnvelope)
	}

	version := binary.BigEndian.Uint16(data[4:6])
	if version > s.version {
		return nil, fmt.Errorf("%w: got %d want <= %d", ErrUnsupportedVersion, version, s.version)
	}

	formatLen := int(data[6])
	headerLen := 7 + formatLen
	if len(data) < headerLen {
		return nil, fmt.Errorf("%w: format truncated", ErrInvalidEnvelope)
	}
	format := string(data[7:headerLen])
	if format != s.serializer.Format() {
		return nil, fmt.Errorf("%w: got %q want %q", ErrFormatMismatch, format, s.serializer.Format())
	}
	return data[headerLen:], nil
}
