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

// Registration registers all Fory-visible application types before cache use.
type Registration func(*fory.Fory) error

// Profile identifies the Go-native Fory wire profile.
type Profile string

const (
	// ProfileNativeFast uses fixed-schema Go-native serialization.
	ProfileNativeFast Profile = "native-fast"
	// ProfileNativeCompatible enables Go-native schema evolution metadata.
	ProfileNativeCompatible Profile = "native-compatible"
)

// Options configures a Redis-backed Fory value cache.
type Options struct {
	// Client executes Redis commands and remains owned by the caller.
	Client redis.Cmdable
	// Namespace isolates this cache's keys from other callers.
	Namespace string
	// SchemaGeneration isolates intentionally incompatible value schemas.
	SchemaGeneration uint32
	// Register deterministically registers all cache-visible Fory types.
	Register Registration
	// MaxPayloadBytes bounds Fory bytes excluding the BTFV header.
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

// ValueCache stores one bounded Fory value type in Redis.
type ValueCache[V any] struct {
	client     commandClient
	keys       btredis.KeyBuilder
	state      *cacheState[V]
	profile    Profile
	generation uint32
	maxPayload int
}

// NewNativeFast creates a cache using fixed-schema Go-native Fory serialization.
func NewNativeFast[V any](options Options) (*ValueCache[V], error) {
	return newValueCache[V](ProfileNativeFast, options)
}

// NewNativeCompatible creates a cache using schema-compatible Go-native Fory serialization.
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
