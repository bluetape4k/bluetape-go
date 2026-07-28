package forynative

import (
	"errors"
	"fmt"
	"reflect"
	"sync"

	"github.com/apache/fory/go/fory"
)

var (
	errRegistrationFailed = errors.New("fory native registration failed")
	errProviderFailed     = errors.New("fory native provider failed")
)

// Profile uint8 공개 타입이며 cache key, miss, TTL, serialization 계약을 보존한다.
// 필드와 zero value, nil 허용 여부, 동시성 소유권은 생성자와 메서드의 한국어 주석 및 테스트 계약을 따른다.
type Profile uint8

const (
	// ProfileNativeFast 고정 schema Go-native serialization profile이다.
	ProfileNativeFast Profile = iota + 1
	// ProfileNativeCompatible Fory-compatible schema evolution metadata를 포함하는 profile이다.
	ProfileNativeCompatible
)

// Registration func 공개 타입이며 cache key, miss, TTL, serialization 계약을 보존한다.
// 필드와 zero value, nil 허용 여부, 동시성 소유권은 생성자와 메서드의 한국어 주석 및 테스트 계약을 따른다.
type Registration func(*fory.Fory) error

// Reason string 공개 타입이며 cache key, miss, TTL, serialization 계약을 보존한다.
// 필드와 zero value, nil 허용 여부, 동시성 소유권은 생성자와 메서드의 한국어 주석 및 테스트 계약을 따른다.
type Reason string

const (
	// ReasonConfiguration identifies invalid runtime configuration.
	ReasonConfiguration Reason = "configuration"
	// ReasonUninitialized identifies use of a zero-value runtime.
	ReasonUninitialized Reason = "uninitialized"
	// ReasonRegistration identifies deterministic registration failure.
	ReasonRegistration Reason = "registration"
	// ReasonPayloadTooLarge identifies a payload limit violation.
	ReasonPayloadTooLarge Reason = "payload-too-large"
	// ReasonUnsupportedValue identifies an unsupported generic root shape.
	ReasonUnsupportedValue Reason = "unsupported-value"
	// ReasonForyFailure identifies a provider serialization failure.
	ReasonForyFailure Reason = "fory-failure"
)

// Error struct 공개 타입이며 cache key, miss, TTL, serialization 계약을 보존한다.
// 필드와 zero value, nil 허용 여부, 동시성 소유권은 생성자와 메서드의 한국어 주석 및 테스트 계약을 따른다.
type Error struct {
	operation string
	reason    Reason
	cause     error
}

// Error Error 공개 API의 동작을 수행하며 cache key, miss, TTL, serialization 계약을 보존한다.
func (e *Error) Error() string {
	if e == nil {
		return "<nil>"
	}
	return fmt.Sprintf("fory native %s failed: %s", e.operation, e.reason)
}

// Unwrap Unwrap 공개 API의 동작을 수행하며 cache key, miss, TTL, serialization 계약을 보존한다.
//
// 반환 오류는 cache miss, 입력 검증 실패, context 취소, Redis/backend 실패, package sentinel error와 typed error를 그대로 드러낸다.
func (e *Error) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.cause
}

// Operation Operation 공개 API의 동작을 수행하며 cache key, miss, TTL, serialization 계약을 보존한다.
func (e *Error) Operation() string {
	if e == nil {
		return ""
	}
	return e.operation
}

// Reason Reason 공개 API의 동작을 수행하며 cache key, miss, TTL, serialization 계약을 보존한다.
func (e *Error) Reason() Reason {
	if e == nil {
		return ""
	}
	return e.reason
}

// Limits struct 공개 타입이며 cache key, miss, TTL, serialization 계약을 보존한다.
// 필드와 zero value, nil 허용 여부, 동시성 소유권은 생성자와 메서드의 한국어 주석 및 테스트 계약을 따른다.
type Limits struct {
	MaxPayloadBytes                 int
	MaxDepth                        int
	MaxTypeFields                   int
	MaxTypeMetaBytes                int
	MaxSchemaVersionsPerType        int
	MaxAverageSchemaVersionsPerType int
}

func (l Limits) withDefaults() Limits {
	if l.MaxPayloadBytes == 0 {
		l.MaxPayloadBytes = 1 << 20
	}
	if l.MaxDepth == 0 {
		l.MaxDepth = 20
	}
	if l.MaxTypeFields == 0 {
		l.MaxTypeFields = 512
	}
	if l.MaxTypeMetaBytes == 0 {
		l.MaxTypeMetaBytes = 4096
	}
	if l.MaxSchemaVersionsPerType == 0 {
		l.MaxSchemaVersionsPerType = 10
	}
	if l.MaxAverageSchemaVersionsPerType == 0 {
		l.MaxAverageSchemaVersionsPerType = 3
	}
	return l
}

// Runtime struct 공개 타입이며 cache key, miss, TTL, serialization 계약을 보존한다.
// 필드와 zero value, nil 허용 여부, 동시성 소유권은 생성자와 메서드의 한국어 주석 및 테스트 계약을 따른다.
type Runtime[V any] struct {
	state        *runtimeState
	limits       Limits
	rootIsStruct bool
}

type runtimeState struct {
	mu      sync.Mutex
	runtime *fory.Fory
}

// New New 공개 API의 동작을 수행하며 cache key, miss, TTL, serialization 계약을 보존한다.
//
// 매개변수:
//   - profile: New에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
//   - limits: New에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
//   - register: New에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
//
// 반환 오류는 cache miss, 입력 검증 실패, context 취소, Redis/backend 실패, package sentinel error와 typed error를 그대로 드러낸다.
func New[V any](profile Profile, limits Limits, register Registration) (*Runtime[V], error) {
	if profile != ProfileNativeFast && profile != ProfileNativeCompatible {
		return nil, newError("configure", ReasonConfiguration, nil)
	}
	resolved := limits.withDefaults()
	if !validLimits(resolved) {
		return nil, newError("configure", ReasonConfiguration, nil)
	}
	root := reflect.TypeOf((*V)(nil)).Elem()
	if !supportedRoot(root) {
		return nil, newError("configure", ReasonUnsupportedValue, nil)
	}
	if register == nil {
		return nil, newError("configure", ReasonRegistration, nil)
	}

	options := []fory.Option{
		fory.WithXlang(false),
		fory.WithCompatible(profile == ProfileNativeCompatible),
		fory.WithTrackRef(false),
		fory.WithMaxDepth(resolved.MaxDepth),
		fory.WithMaxTypeFields(resolved.MaxTypeFields),
		fory.WithMaxTypeMetaBytes(resolved.MaxTypeMetaBytes),
		fory.WithMaxSchemaVersionsPerType(resolved.MaxSchemaVersionsPerType),
		fory.WithMaxAverageSchemaVersionsPerType(resolved.MaxAverageSchemaVersionsPerType),
	}
	runtime, err := constructRuntime(options, register)
	if err != nil {
		return nil, newError("register", ReasonRegistration, errRegistrationFailed)
	}
	return &Runtime[V]{
		state:        &runtimeState{runtime: runtime},
		limits:       resolved,
		rootIsStruct: root.Kind() == reflect.Struct,
	}, nil
}

// Serialize Serialize 공개 API의 동작을 수행하며 cache key, miss, TTL, serialization 계약을 보존한다.
//
// 매개변수:
//   - value: 직렬화하거나 cache에 보관할 값이다. nil, zero value, aliasing 의미는 serializer/cache 계약을 따른다.
//
// 반환 오류는 cache miss, 입력 검증 실패, context 취소, Redis/backend 실패, package sentinel error와 typed error를 그대로 드러낸다.
func (r *Runtime[V]) Serialize(value V) ([]byte, error) {
	if r == nil || r.state == nil || r.state.runtime == nil {
		return nil, newError("serialize", ReasonUninitialized, nil)
	}
	var input any = value
	if r.rootIsStruct {
		input = &value
	}

	r.state.mu.Lock()
	defer r.state.mu.Unlock()

	var raw []byte
	err := recoverProviderPanic(func() error {
		var err error
		raw, err = r.state.runtime.Serialize(input)
		return err
	})
	if err != nil {
		return nil, newError("serialize", ReasonForyFailure, errProviderFailed)
	}
	if len(raw) > r.limits.MaxPayloadBytes {
		return nil, newError("serialize", ReasonPayloadTooLarge, nil)
	}
	return append([]byte(nil), raw...), nil
}

// Deserialize Deserialize 공개 API의 동작을 수행하며 cache key, miss, TTL, serialization 계약을 보존한다.
//
// 매개변수:
//   - raw: Deserialize에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
//
// 반환 오류는 cache miss, 입력 검증 실패, context 취소, Redis/backend 실패, package sentinel error와 typed error를 그대로 드러낸다.
func (r *Runtime[V]) Deserialize(raw []byte) (V, error) {
	var value V
	if r == nil || r.state == nil || r.state.runtime == nil {
		return value, newError("deserialize", ReasonUninitialized, nil)
	}
	if len(raw) > r.limits.MaxPayloadBytes {
		return value, newError("deserialize", ReasonPayloadTooLarge, nil)
	}

	r.state.mu.Lock()
	defer r.state.mu.Unlock()

	err := recoverProviderPanic(func() error {
		return r.state.runtime.Deserialize(raw, &value)
	})
	if err != nil {
		return value, newError("deserialize", ReasonForyFailure, errProviderFailed)
	}
	return value, nil
}

func constructRuntime(options []fory.Option, register Registration) (runtime *fory.Fory, err error) {
	defer func() {
		if recover() != nil {
			runtime = nil
			err = errRegistrationFailed
		}
	}()
	runtime = fory.New(options...)
	if err := register(runtime); err != nil {
		return nil, errRegistrationFailed
	}
	return runtime, nil
}

func recoverProviderPanic(call func() error) (err error) {
	defer func() {
		if recover() != nil {
			err = errProviderFailed
		}
	}()
	if err := call(); err != nil {
		return errProviderFailed
	}
	return nil
}

func validLimits(limits Limits) bool {
	values := [...]int{
		limits.MaxPayloadBytes,
		limits.MaxDepth,
		limits.MaxTypeFields,
		limits.MaxTypeMetaBytes,
		limits.MaxSchemaVersionsPerType,
		limits.MaxAverageSchemaVersionsPerType,
	}
	for _, value := range values {
		if value <= 0 {
			return false
		}
	}
	return uint64(limits.MaxPayloadBytes) <= uint64(^uint32(0))
}

func supportedRoot(root reflect.Type) bool {
	switch root.Kind() {
	case reflect.Bool,
		reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr,
		reflect.Float32, reflect.Float64,
		reflect.String, reflect.Struct:
		return true
	case reflect.Slice:
		return root.Elem().Kind() == reflect.Uint8
	default:
		return false
	}
}

func newError(operation string, reason Reason, cause error) *Error {
	return &Error{operation: operation, reason: reason, cause: cause}
}
