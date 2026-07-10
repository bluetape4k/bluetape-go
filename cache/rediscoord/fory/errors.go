package rediscoordfory

import "fmt"

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
	ReasonConfiguration      Reason = "configuration"
	ReasonUninitialized      Reason = "uninitialized"
	ReasonRegistration       Reason = "registration"
	ReasonPayloadTooLarge    Reason = "payload-too-large"
	ReasonInvalidMagic       Reason = "invalid-magic"
	ReasonUnsupportedVersion Reason = "unsupported-version"
	ReasonProfileMismatch    Reason = "profile-mismatch"
	ReasonLengthMismatch     Reason = "length-mismatch"
	ReasonUnsupportedValue   Reason = "unsupported-value"
	ReasonForyFailure        Reason = "fory-failure"
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

// Unwrap returns the underlying cause for errors.Is and errors.As.
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
