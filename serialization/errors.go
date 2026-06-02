package serialization

import "errors"

var (
	// ErrInvalidEnvelope reports malformed or unsupported versioned payloads.
	ErrInvalidEnvelope = errors.New("invalid serialization envelope")
	// ErrFormatMismatch reports an envelope format that does not match the serializer.
	ErrFormatMismatch = errors.New("serialization format mismatch")
	// ErrUnsupportedVersion reports an envelope version newer than the serializer supports.
	ErrUnsupportedVersion = errors.New("unsupported serialization version")
)
