package redisfory

import (
	"errors"
	"fmt"
)

var (
	errProviderFailed     = errors.New("redisfory provider failed")
	errRegistrationFailed = errors.New("redisfory registration failed")
)

// Reason is a stable, low-cardinality cache failure category.
type Reason string

const (
	// ReasonConfiguration identifies invalid cache configuration.
	ReasonConfiguration Reason = "configuration"
	// ReasonUninitialized identifies use of a zero-value cache.
	ReasonUninitialized Reason = "uninitialized"
	// ReasonRegistration identifies deterministic Fory registration failure.
	ReasonRegistration Reason = "registration"
	// ReasonPayloadTooLarge identifies a configured payload limit violation.
	ReasonPayloadTooLarge Reason = "payload-too-large"
	// ReasonInvalidMagic identifies data without the BTFV envelope marker.
	ReasonInvalidMagic Reason = "invalid-magic"
	// ReasonUnsupportedVersion identifies an unsupported BTFV version.
	ReasonUnsupportedVersion Reason = "unsupported-version"
	// ReasonFormatMismatch identifies data written by another Fory profile.
	ReasonFormatMismatch Reason = "format-mismatch"
	// ReasonSchemaMismatch identifies data written for another schema generation.
	ReasonSchemaMismatch Reason = "schema-mismatch"
	// ReasonLengthMismatch identifies truncated or trailing envelope data.
	ReasonLengthMismatch Reason = "length-mismatch"
	// ReasonUnsupportedValue identifies a generic root type unsupported by this cache.
	ReasonUnsupportedValue Reason = "unsupported-value"
	// ReasonForyFailure identifies a sanitized Fory provider failure.
	ReasonForyFailure Reason = "fory-failure"
)

// CacheError describes a sanitized redisfory failure.
type CacheError struct {
	operation string
	profile   Profile
	reason    Reason
	cause     error
}

// Error returns a stable message without Redis keys, values, or provider details.
func (e *CacheError) Error() string {
	if e == nil {
		return "<nil>"
	}
	return fmt.Sprintf("redisfory %s failed: %s", e.operation, e.reason)
}

// Unwrap returns only a sanitized package cause.
func (e *CacheError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.cause
}

// Operation returns the stable operation name.
func (e *CacheError) Operation() string {
	if e == nil {
		return ""
	}
	return e.operation
}

// Profile returns the configured Fory profile.
func (e *CacheError) Profile() Profile {
	if e == nil {
		return ""
	}
	return e.profile
}

// Reason returns the stable failure category.
func (e *CacheError) Reason() Reason {
	if e == nil {
		return ""
	}
	return e.reason
}
