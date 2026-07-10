package rediscoordfory

import (
	"errors"

	"github.com/apache/fory/go/fory"
	"github.com/bluetape4k/bluetape-go/cache/internal/forynative"
)

// Registration registers every Fory struct, enum, and extension type used by a codec.
type Registration func(*fory.Fory) error

// Options configures a native Fory codec. Zero limit values use the documented
// bounded defaults; negative values are rejected.
type Options struct {
	// Register deterministically registers all codec-visible Fory types.
	Register Registration
	// MaxPayloadBytes bounds Fory bytes excluding the BTFY header.
	MaxPayloadBytes int
	// MaxDepth bounds nested Fory values.
	MaxDepth int
	// MaxTypeFields bounds fields in one Fory type.
	MaxTypeFields int
	// MaxTypeMetaBytes bounds encoded Fory type metadata.
	MaxTypeMetaBytes int
	// MaxSchemaVersionsPerType bounds retained schema versions per type.
	MaxSchemaVersionsPerType int
	// MaxAverageSchemaVersionsPerType bounds average retained schema versions.
	MaxAverageSchemaVersionsPerType int
}

func (o Options) withDefaults() Options {
	if o.MaxPayloadBytes == 0 {
		o.MaxPayloadBytes = 1 << 20
	}
	if o.MaxDepth == 0 {
		o.MaxDepth = 20
	}
	if o.MaxTypeFields == 0 {
		o.MaxTypeFields = 512
	}
	if o.MaxTypeMetaBytes == 0 {
		o.MaxTypeMetaBytes = 4096
	}
	if o.MaxSchemaVersionsPerType == 0 {
		o.MaxSchemaVersionsPerType = 10
	}
	if o.MaxAverageSchemaVersionsPerType == 0 {
		o.MaxAverageSchemaVersionsPerType = 3
	}
	return o
}

// Codec implements rediscoord's Marshal/Unmarshal contract. Copies share the
// same internal Fory runtime and synchronization state.
type Codec[V any] struct {
	state      *codecState[V]
	profile    Profile
	maxPayload int
}

type codecState[V any] struct {
	runtime *forynative.Runtime[V]
}

// NewNativeFast creates a native, non-compatible Fory codec.
func NewNativeFast[V any](options Options) (*Codec[V], error) {
	return newCodec[V](ProfileNativeFast, options)
}

// NewNativeCompatible creates a native schema-compatible Fory codec.
func NewNativeCompatible[V any](options Options) (*Codec[V], error) {
	return newCodec[V](ProfileNativeCompatible, options)
}

func newCodec[V any](profile Profile, options Options) (*Codec[V], error) {
	o := options.withDefaults()
	nativeProfile := forynative.ProfileNativeFast
	if profile == ProfileNativeCompatible {
		nativeProfile = forynative.ProfileNativeCompatible
	}
	runtime, err := forynative.New[V](nativeProfile, forynative.Limits{
		MaxPayloadBytes:                 o.MaxPayloadBytes,
		MaxDepth:                        o.MaxDepth,
		MaxTypeFields:                   o.MaxTypeFields,
		MaxTypeMetaBytes:                o.MaxTypeMetaBytes,
		MaxSchemaVersionsPerType:        o.MaxSchemaVersionsPerType,
		MaxAverageSchemaVersionsPerType: o.MaxAverageSchemaVersionsPerType,
	}, forynative.Registration(o.Register))
	if err != nil {
		return nil, mapRuntimeError("configure", profile, err)
	}
	return &Codec[V]{state: &codecState[V]{runtime: runtime}, profile: profile, maxPayload: o.MaxPayloadBytes}, nil
}

// Marshal serializes a value and wraps it in the profile envelope.
func (c *Codec[V]) Marshal(value V) ([]byte, error) {
	if c == nil || c.state == nil || c.state.runtime == nil {
		return nil, &CodecError{operation: "marshal", reason: ReasonUninitialized}
	}
	raw, err := c.state.runtime.Serialize(value)
	if err != nil {
		return nil, mapRuntimeError("marshal", c.profile, err)
	}
	return wrap(c.profile, raw), nil
}

// Unmarshal decodes a profile envelope into a value.
func (c *Codec[V]) Unmarshal(data []byte) (V, error) {
	var value V
	if c == nil || c.state == nil || c.state.runtime == nil {
		return value, &CodecError{operation: "unmarshal", reason: ReasonUninitialized}
	}
	raw, err := unwrap(c.profile, data, c.maxPayload)
	if err != nil {
		return value, err
	}
	value, err = c.state.runtime.Deserialize(raw)
	if err != nil {
		return value, mapRuntimeError("unmarshal", c.profile, err)
	}
	return value, nil
}

func mapRuntimeError(operation string, profile Profile, err error) error {
	var runtimeErr *forynative.Error
	if !errors.As(err, &runtimeErr) {
		return &CodecError{operation: operation, profile: profile, reason: ReasonForyFailure, cause: errProviderFailed}
	}
	switch runtimeErr.Reason() {
	case forynative.ReasonConfiguration:
		return &CodecError{operation: operation, profile: profile, reason: ReasonConfiguration}
	case forynative.ReasonUninitialized:
		return &CodecError{operation: operation, profile: profile, reason: ReasonUninitialized}
	case forynative.ReasonRegistration:
		return &CodecError{operation: "register", profile: profile, reason: ReasonRegistration, cause: errRegistrationFailed}
	case forynative.ReasonPayloadTooLarge:
		return &CodecError{operation: operation, profile: profile, reason: ReasonPayloadTooLarge}
	case forynative.ReasonUnsupportedValue:
		return &CodecError{operation: operation, profile: profile, reason: ReasonUnsupportedValue}
	default:
		return &CodecError{operation: operation, profile: profile, reason: ReasonForyFailure, cause: errProviderFailed}
	}
}
