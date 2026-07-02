package encrypt

import (
	"errors"
	"fmt"
)

var (
	// ErrInvalidKey reports missing or unsupported AES key material.
	ErrInvalidKey = errors.New("encrypt: invalid key")
	// ErrMalformedCiphertext reports a ciphertext envelope that cannot be parsed.
	ErrMalformedCiphertext = errors.New("encrypt: malformed ciphertext")
	// ErrAuthenticationFailed reports tamper, wrong key, or wrong associated data.
	ErrAuthenticationFailed = errors.New("encrypt: authentication failed")
	// ErrInvalidOptions reports invalid encryptor options.
	ErrInvalidOptions = errors.New("encrypt: invalid options")
)

// Error preserves encrypt sentinel identity and an optional cause without
// exposing plaintext, ciphertext, key bytes, or associated data through Error().
type Error struct {
	Kind      error
	Operation string
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
	if e.Operation != "" {
		return fmt.Sprintf("%v: %s", kind, e.Operation)
	}
	return kind.Error()
}

func (e *Error) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Cause
}

// Is reports matches against encrypt sentinel errors and the wrapped cause.
func (e *Error) Is(target error) bool {
	if e == nil {
		return false
	}
	return target == e.Kind || errors.Is(e.Kind, target) || errors.Is(e.Cause, target)
}

func errorWith(kind error, operation string, cause error) *Error {
	return &Error{Kind: kind, Operation: operation, Cause: cause}
}
