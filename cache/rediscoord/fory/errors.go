package rediscoordfory

import (
	"errors"
	"fmt"
)

var (
	errRegistrationFailed = errors.New("fory codec registration failed")
	errProviderFailed     = errors.New("fory codec provider failed")
)

// Profile identifies the wire profile used by a codec.
type Profile string

const (
	// ProfileNativeFast is the fixed-schema Go-native profile.
	ProfileNativeFast Profile = "native-fast"
	// ProfileNativeCompatible is the schema-compatible Go-native profile.
	ProfileNativeCompatible Profile = "native-compatible"
)

// Reason is a stable, low-detail error category safe for logs and metrics.
type Reason string

const (
	// ReasonConfiguration identifies invalid codec configuration.
	ReasonConfiguration Reason = "configuration"
	// ReasonUninitialized identifies use of a zero-value codec.
	ReasonUninitialized Reason = "uninitialized"
	// ReasonRegistration identifies deterministic type registration failure.
	ReasonRegistration Reason = "registration"
	// ReasonPayloadTooLarge identifies a configured payload bound violation.
	ReasonPayloadTooLarge Reason = "payload-too-large"
	// ReasonInvalidMagic identifies a non-BTFY payload.
	ReasonInvalidMagic Reason = "invalid-magic"
	// ReasonUnsupportedVersion identifies an unknown BTFY wrapper version.
	ReasonUnsupportedVersion Reason = "unsupported-version"
	// ReasonProfileMismatch identifies a wrapper from another Fory profile.
	ReasonProfileMismatch Reason = "profile-mismatch"
	// ReasonLengthMismatch identifies truncated or trailing wrapper bytes.
	ReasonLengthMismatch Reason = "length-mismatch"
	// ReasonUnsupportedValue identifies an unsupported generic root shape.
	ReasonUnsupportedValue Reason = "unsupported-value"
	// ReasonForyFailure identifies a provider marshal or unmarshal failure.
	ReasonForyFailure Reason = "fory-failure"
)

// CodecError describes a codec failure without formatting payload or provider details.
type CodecError struct {
	operation string
	profile   Profile
	reason    Reason
	cause     error
}

func (e *CodecError) Error() string {
	if e == nil {
		return "<nil>"
	}
	return fmt.Sprintf("fory codec %s failed (%s): %s", e.operation, e.profile, e.reason)
}

// Unwrap returns a sanitized package cause for errors.Is and errors.As.
func (e *CodecError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.cause
}

// Operation returns the stable operation label.
func (e *CodecError) Operation() string { return e.operation }

// Profile returns the codec profile involved in the failure.
func (e *CodecError) Profile() Profile { return e.profile }

// Reason returns the stable low-cardinality failure reason.
func (e *CodecError) Reason() Reason { return e.reason }
