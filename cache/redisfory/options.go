package redisfory

import (
	"errors"
	"fmt"
	"reflect"
	"regexp"
	"strings"

	"github.com/apache/fory/go/fory"
	"github.com/bluetape4k/bluetape-go/cache/internal/forynative"
	btredis "github.com/bluetape4k/bluetape-go/redis"
	"github.com/redis/go-redis/v9"
)

var namespaceSegmentPattern = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)

// Registration func 공개 타입이며 Redis 값 캐시의 serialization, TTL, backend ownership 계약을 보존한다.
// 필드와 zero value, nil 허용 여부, 동시성 소유권은 생성자와 메서드의 한국어 주석 및 테스트 계약을 따른다.
type Registration func(*fory.Fory) error

// Profile string 공개 타입이며 Redis 값 캐시의 serialization, TTL, backend ownership 계약을 보존한다.
// 필드와 zero value, nil 허용 여부, 동시성 소유권은 생성자와 메서드의 한국어 주석 및 테스트 계약을 따른다.
type Profile string

const (
	// ProfileNativeFast 고정 schema Go-native serialization profile이다.
	ProfileNativeFast Profile = "native-fast"
	// ProfileNativeCompatible Go-native schema evolution metadata를 포함하는 profile이다.
	ProfileNativeCompatible Profile = "native-compatible"
)

// Options struct 공개 타입이며 Redis 값 캐시의 serialization, TTL, backend ownership 계약을 보존한다.
// 필드와 zero value, nil 허용 여부, 동시성 소유권은 생성자와 메서드의 한국어 주석 및 테스트 계약을 따른다.
type Options struct {
	// Client executes Redis commands and remains owned by the caller.
	Client redis.Cmdable
	// Namespace isolates this cache's keys from other callers.
	Namespace string
	// SchemaGeneration isolates intentionally incompatible value schemas.
	SchemaGeneration uint32
	// Register deterministically registers all cache-visible Fory types.
	Register Registration
	// MaxPayloadBytes BTFV header를 제외한 Fory byte payload 상한이다.
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

// ValueCache struct 공개 타입이며 Redis 값 캐시의 serialization, TTL, backend ownership 계약을 보존한다.
// 필드와 zero value, nil 허용 여부, 동시성 소유권은 생성자와 메서드의 한국어 주석 및 테스트 계약을 따른다.
type ValueCache[V any] struct {
	client     commandClient
	keys       btredis.KeyBuilder
	state      *cacheState[V]
	profile    Profile
	generation uint32
	maxPayload int
}

// NewNativeFast NewNativeFast 공개 API의 동작을 수행하며 Redis 값 캐시의 serialization, TTL, backend ownership 계약을 보존한다.
//
// 매개변수:
//   - options: 적용할 옵션 목록이다. nil이면 기본값만 사용한다.
//
// 반환 오류는 cache miss, 입력 검증 실패, context 취소, Redis/backend 실패, package sentinel error와 typed error를 그대로 드러낸다.
func NewNativeFast[V any](options Options) (*ValueCache[V], error) {
	return newValueCache[V](ProfileNativeFast, options)
}

// NewNativeCompatible NewNativeCompatible 공개 API의 동작을 수행하며 Redis 값 캐시의 serialization, TTL, backend ownership 계약을 보존한다.
//
// 매개변수:
//   - options: 적용할 옵션 목록이다. nil이면 기본값만 사용한다.
//
// 반환 오류는 cache miss, 입력 검증 실패, context 취소, Redis/backend 실패, package sentinel error와 typed error를 그대로 드러낸다.
func NewNativeCompatible[V any](options Options) (*ValueCache[V], error) {
	return newValueCache[V](ProfileNativeCompatible, options)
}

func newValueCache[V any](profile Profile, options Options) (*ValueCache[V], error) {
	if nilInterface(options.Client) {
		return nil, newCacheError("configure", profile, ReasonConfiguration, nil)
	}
	segments, ok := namespaceSegments(options.Namespace)
	if !ok || options.SchemaGeneration == 0 {
		return nil, newCacheError("configure", profile, ReasonConfiguration, nil)
	}

	nativeProfile := forynative.ProfileNativeFast
	if profile == ProfileNativeCompatible {
		nativeProfile = forynative.ProfileNativeCompatible
	}
	runtime, err := forynative.New[V](nativeProfile, forynative.Limits{
		MaxPayloadBytes:                 options.MaxPayloadBytes,
		MaxDepth:                        options.MaxDepth,
		MaxTypeFields:                   options.MaxTypeFields,
		MaxTypeMetaBytes:                options.MaxTypeMetaBytes,
		MaxSchemaVersionsPerType:        options.MaxSchemaVersionsPerType,
		MaxAverageSchemaVersionsPerType: options.MaxAverageSchemaVersionsPerType,
	}, forynative.Registration(options.Register))
	if err != nil {
		return nil, mapRuntimeError("configure", profile, err)
	}

	maxPayload := options.MaxPayloadBytes
	if maxPayload == 0 {
		maxPayload = 1 << 20
	}
	builder, err := btredis.NewKeyBuilder("bluetape:cache:fory")
	if err != nil {
		return nil, newCacheError("configure", profile, ReasonConfiguration, nil)
	}
	segments = append(segments, fmt.Sprintf("g%d", options.SchemaGeneration))
	builder, err = builder.Structural(segments...)
	if err != nil {
		return nil, newCacheError("configure", profile, ReasonConfiguration, nil)
	}
	return &ValueCache[V]{
		client:     options.Client,
		keys:       builder,
		state:      &cacheState[V]{codec: runtime},
		profile:    profile,
		generation: options.SchemaGeneration,
		maxPayload: maxPayload,
	}, nil
}

func (c *ValueCache[V]) key(logicalKey string) (btredis.Key, error) {
	if c == nil {
		return btredis.Key{}, newCacheError("key", "", ReasonUninitialized, nil)
	}
	return c.keys.LogicalKey(logicalKey)
}

func namespaceSegments(namespace string) ([]string, bool) {
	if namespace == "" || strings.TrimSpace(namespace) != namespace {
		return nil, false
	}
	segments := strings.Split(namespace, ":")
	for _, segment := range segments {
		if !namespaceSegmentPattern.MatchString(segment) {
			return nil, false
		}
	}
	return segments, true
}

func nilInterface(value any) bool {
	if value == nil {
		return true
	}
	rv := reflect.ValueOf(value)
	switch rv.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return rv.IsNil()
	default:
		return false
	}
}

func mapRuntimeError(operation string, profile Profile, err error) error {
	var runtimeErr *forynative.Error
	if !errors.As(err, &runtimeErr) {
		return newCacheError(operation, profile, ReasonForyFailure, errProviderFailed)
	}
	switch runtimeErr.Reason() {
	case forynative.ReasonConfiguration:
		return newCacheError(operation, profile, ReasonConfiguration, nil)
	case forynative.ReasonUninitialized:
		return newCacheError(operation, profile, ReasonUninitialized, nil)
	case forynative.ReasonRegistration:
		return newCacheError("register", profile, ReasonRegistration, errRegistrationFailed)
	case forynative.ReasonPayloadTooLarge:
		return newCacheError(operation, profile, ReasonPayloadTooLarge, nil)
	case forynative.ReasonUnsupportedValue:
		return newCacheError(operation, profile, ReasonUnsupportedValue, nil)
	default:
		return newCacheError(operation, profile, ReasonForyFailure, errProviderFailed)
	}
}

func newCacheError(operation string, profile Profile, reason Reason, cause error) *CacheError {
	return &CacheError{operation: operation, profile: profile, reason: reason, cause: cause}
}
