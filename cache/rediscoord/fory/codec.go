package rediscoordfory

import (
	"errors"

	"github.com/apache/fory/go/fory"
	"github.com/bluetape4k/bluetape-go/cache/internal/forynative"
)

// Registration Redis 조정, stampede 방지, codec envelope에서 사용하는 함수 타입이다.
type Registration func(*fory.Fory) error

// Options Redis 조정, stampede 방지, codec envelope에서 사용하는 구조체다.
type Options struct {
	// Register deterministically registers all codec-visible Fory types.
	Register Registration
	// MaxPayloadBytes BTFY header를 제외한 Fory byte payload 상한이다.
	MaxPayloadBytes int
	// MaxDepth 중첩된 Fory value 깊이 상한이다.
	MaxDepth int
	// MaxTypeFields 하나의 Fory type에 포함할 수 있는 field 수 상한이다.
	MaxTypeFields int
	// MaxTypeMetaBytes encoding된 Fory type metadata byte 상한이다.
	MaxTypeMetaBytes int
	// MaxSchemaVersionsPerType type별로 보존할 schema version 수 상한이다.
	MaxSchemaVersionsPerType int
	// MaxAverageSchemaVersionsPerType type별 평균 보존 schema version 수 상한이다.
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

// Codec Redis 조정, stampede 방지, codec envelope에서 사용하는 구조체다.
type Codec[V any] struct {
	state      *codecState[V]
	profile    Profile
	maxPayload int
}

type codecState[V any] struct {
	runtime *forynative.Runtime[V]
}

// NewNativeFast Redis 조정, stampede 방지, codec envelope에 사용할 값을 생성한다.
//
// 매개변수:
//   - options: 적용할 옵션 목록이다. nil이면 기본값만 사용한다.
//
// 반환 오류는 cache miss, 입력 검증 실패, context 취소, Redis/backend 실패, package sentinel error와 typed error를 그대로 드러낸다.
func NewNativeFast[V any](options Options) (*Codec[V], error) {
	return newCodec[V](ProfileNativeFast, options)
}

// NewNativeCompatible Redis 조정, stampede 방지, codec envelope에 사용할 값을 생성한다.
//
// 매개변수:
//   - options: 적용할 옵션 목록이다. nil이면 기본값만 사용한다.
//
// 반환 오류는 cache miss, 입력 검증 실패, context 취소, Redis/backend 실패, package sentinel error와 typed error를 그대로 드러낸다.
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

// Marshal Redis 조정, stampede 방지, codec envelope 값을 직렬화한다.
//
// 매개변수:
//   - value: 직렬화하거나 cache에 보관할 값이다. nil, zero value, aliasing 의미는 serializer/cache 계약을 따른다.
//
// 반환 오류는 cache miss, 입력 검증 실패, context 취소, Redis/backend 실패, package sentinel error와 typed error를 그대로 드러낸다.
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

// Unmarshal Redis 조정, stampede 방지, codec envelope 값을 복원한다.
//
// 매개변수:
//   - data: Unmarshal에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
//
// 반환 오류는 cache miss, 입력 검증 실패, context 취소, Redis/backend 실패, package sentinel error와 typed error를 그대로 드러낸다.
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
