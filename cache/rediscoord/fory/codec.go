package rediscoordfory

import (
	"reflect"
	"sync"

	"github.com/apache/fory/go/fory"
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

// Codec implements rediscoord's Marshal/Unmarshal contract.
type Codec[V any] struct {
	noCopy     noCopy
	mu         sync.Mutex
	runtime    *fory.Fory
	profile    Profile
	maxPayload int
}

// noCopy makes accidental Codec value copies visible to go vet -copylocks.
type noCopy struct{}

func (*noCopy) Lock()   {}
func (*noCopy) Unlock() {}

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
	vals := []int{o.MaxPayloadBytes, o.MaxDepth, o.MaxTypeFields, o.MaxTypeMetaBytes, o.MaxSchemaVersionsPerType, o.MaxAverageSchemaVersionsPerType}
	for _, v := range vals {
		if v <= 0 {
			return nil, &CodecError{operation: "configure", profile: profile, reason: ReasonConfiguration}
		}
	}
	if uint64(o.MaxPayloadBytes) > uint64(^uint32(0)) {
		return nil, &CodecError{operation: "configure", profile: profile, reason: ReasonConfiguration}
	}
	t := reflect.TypeOf((*V)(nil)).Elem()
	if !supportedRoot(t) {
		return nil, &CodecError{operation: "configure", profile: profile, reason: ReasonUnsupportedValue}
	}
	if o.Register == nil {
		return nil, &CodecError{operation: "configure", profile: profile, reason: ReasonRegistration}
	}
	optionsList := []fory.Option{fory.WithXlang(false), fory.WithCompatible(profile == ProfileNativeCompatible), fory.WithTrackRef(false), fory.WithMaxDepth(o.MaxDepth), fory.WithMaxTypeFields(o.MaxTypeFields), fory.WithMaxTypeMetaBytes(o.MaxTypeMetaBytes), fory.WithMaxSchemaVersionsPerType(o.MaxSchemaVersionsPerType), fory.WithMaxAverageSchemaVersionsPerType(o.MaxAverageSchemaVersionsPerType)}
	r := fory.New(optionsList...)
	if err := o.Register(r); err != nil {
		return nil, &CodecError{operation: "register", profile: profile, reason: ReasonRegistration, cause: errRegistrationFailed}
	}
	return &Codec[V]{runtime: r, profile: profile, maxPayload: o.MaxPayloadBytes}, nil
}

func supportedRoot(t reflect.Type) bool {
	switch t.Kind() {
	case reflect.Bool,
		reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr,
		reflect.Float32, reflect.Float64,
		reflect.String, reflect.Struct:
		return true
	case reflect.Slice:
		return t.Elem().Kind() == reflect.Uint8
	default:
		return false
	}
}

// Marshal serializes a value and wraps it in the profile envelope.
func (c *Codec[V]) Marshal(value V) ([]byte, error) {
	if c == nil || c.runtime == nil {
		return nil, &CodecError{operation: "marshal", reason: ReasonUninitialized}
	}
	var input any = value
	if reflect.TypeOf(value).Kind() == reflect.Struct {
		input = &value
	}
	raw, tooLarge, err := c.serialize(input)
	if err != nil {
		return nil, &CodecError{operation: "marshal", profile: c.profile, reason: ReasonForyFailure, cause: errProviderFailed}
	}
	if tooLarge {
		return nil, &CodecError{operation: "marshal", profile: c.profile, reason: ReasonPayloadTooLarge}
	}
	return wrap(c.profile, raw), nil
}

func (c *Codec[V]) serialize(input any) ([]byte, bool, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	raw, err := c.runtime.Serialize(input)
	if err != nil {
		return nil, false, err
	}
	if len(raw) > c.maxPayload {
		return nil, true, nil
	}
	return append([]byte(nil), raw...), false, nil
}

// Unmarshal decodes a profile envelope into a value.
func (c *Codec[V]) Unmarshal(data []byte) (V, error) {
	var value V
	if c == nil || c.runtime == nil {
		return value, &CodecError{operation: "unmarshal", reason: ReasonUninitialized}
	}
	raw, err := unwrap(c.profile, data, c.maxPayload)
	if err != nil {
		return value, err
	}
	err = c.deserialize(raw, &value)
	if err != nil {
		return value, &CodecError{operation: "unmarshal", profile: c.profile, reason: ReasonForyFailure, cause: errProviderFailed}
	}
	return value, nil
}

func (c *Codec[V]) deserialize(raw []byte, value *V) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.runtime.Deserialize(raw, value)
}
