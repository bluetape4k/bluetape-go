package imagekit

import (
	"errors"
	"fmt"
)

var (
	// ErrInvalidOptions reports an invalid transform request or nil I/O value.
	ErrInvalidOptions = errors.New("imagekit: invalid options")
	// ErrUnsupportedFormat reports an unsupported input or output format.
	ErrUnsupportedFormat = errors.New("imagekit: unsupported format")
	// ErrInputTooLarge reports that encoded input exceeds MaxInputBytes.
	ErrInputTooLarge = errors.New("imagekit: input too large")
	// ErrImageTooLarge reports that decoded or requested dimensions exceed limits.
	ErrImageTooLarge = errors.New("imagekit: image too large")
	// ErrDecode reports input read, config decode, or full decode failure.
	ErrDecode = errors.New("imagekit: decode failed")
	// ErrEncode reports output encode failure.
	ErrEncode = errors.New("imagekit: encode failed")
)

// Error preserves imagekit sentinel identity and an optional cause without
// exposing raw payload bytes, file paths, or cause text through Error().
type Error struct {
	Kind      error
	Operation string
	Format    string
	Cause     error
}

func (e *Error) Error() string {
	if e == nil {
		return ErrInvalidOptions.Error()
	}
	kind := e.Kind
	if kind == nil {
		kind = ErrInvalidOptions
	}
	if e.Format != "" && e.Operation != "" {
		return fmt.Sprintf("%v: %s: format=%s", kind, e.Operation, e.Format)
	}
	if e.Operation != "" {
		return fmt.Sprintf("%v: %s", kind, e.Operation)
	}
	if e.Format != "" {
		return fmt.Sprintf("%v: format=%s", kind, e.Format)
	}
	return kind.Error()
}

func (e *Error) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Cause
}

// Is reports matches against imagekit sentinel errors, context sentinel errors,
// and the wrapped cause.
func (e *Error) Is(target error) bool {
	if e == nil {
		return false
	}
	return target == e.Kind || errors.Is(e.Kind, target) || errors.Is(e.Cause, target)
}

func errorWith(kind error, operation string, format string, cause error) *Error {
	return &Error{Kind: kind, Operation: operation, Format: format, Cause: cause}
}
