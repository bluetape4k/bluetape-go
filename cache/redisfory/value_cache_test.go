package redisfory

import (
	"encoding/binary"
	"errors"
	"strconv"
	"strings"
	"testing"
	"unsafe"

	"github.com/apache/fory/go/fory"
	btredis "github.com/bluetape4k/bluetape-go/redis"
	"github.com/redis/go-redis/v9"
)

type testValue struct {
	Name  string
	Count int
}

func registerTestValue(runtime *fory.Fory) error {
	return runtime.RegisterStructByName(testValue{}, "redisfory.testValue")
}

func testClient(t *testing.T) *redis.Client {
	t.Helper()
	client := redis.NewClient(&redis.Options{Addr: "127.0.0.1:1"})
	t.Cleanup(func() {
		if err := client.Close(); err != nil {
			t.Fatalf("close Redis client: %v", err)
		}
	})
	return client
}

func validOptions(t *testing.T) Options {
	t.Helper()
	return Options{
		Client:           testClient(t),
		Namespace:        "catalog.prod",
		SchemaGeneration: 7,
		Register:         registerTestValue,
	}
}

func TestConstructorsExposeProfilesAndBoundedDefaults(t *testing.T) {
	tests := []struct {
		name    string
		new     func(Options) (*ValueCache[testValue], error)
		profile Profile
		format  byte
	}{
		{name: "fast", new: NewNativeFast[testValue], profile: ProfileNativeFast, format: 1},
		{name: "compatible", new: NewNativeCompatible[testValue], profile: ProfileNativeCompatible, format: 2},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cache, err := tc.new(validOptions(t))
			if err != nil {
				t.Fatal(err)
			}
			if cache.profile != tc.profile || cache.format != tc.format {
				t.Fatalf("profile/format = %q/%d", cache.profile, cache.format)
			}
			if cache.maxPayload != 1<<20 {
				t.Fatalf("max payload = %d", cache.maxPayload)
			}
		})
	}
}

func TestConstructorsRejectInvalidConfiguration(t *testing.T) {
	var typedNil *redis.Client
	tests := []struct {
		name    string
		options func(*testing.T) Options
		reason  Reason
	}{
		{name: "nil-client", options: func(t *testing.T) Options {
			o := validOptions(t)
			o.Client = nil
			return o
		}, reason: ReasonConfiguration},
		{name: "typed-nil-client", options: func(t *testing.T) Options {
			o := validOptions(t)
			o.Client = typedNil
			return o
		}, reason: ReasonConfiguration},
		{name: "empty-namespace", options: func(t *testing.T) Options {
			o := validOptions(t)
			o.Namespace = ""
			return o
		}, reason: ReasonConfiguration},
		{name: "zero-generation", options: func(t *testing.T) Options {
			o := validOptions(t)
			o.SchemaGeneration = 0
			return o
		}, reason: ReasonConfiguration},
		{name: "nil-registration", options: func(t *testing.T) Options {
			o := validOptions(t)
			o.Register = nil
			return o
		}, reason: ReasonRegistration},
		{name: "negative-limit", options: func(t *testing.T) Options {
			o := validOptions(t)
			o.MaxDepth = -1
			return o
		}, reason: ReasonConfiguration},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := NewNativeFast[testValue](tc.options(t))
			assertCacheReason(t, err, tc.reason)
		})
	}
	if strconv.IntSize > 32 {
		o := validOptions(t)
		o.MaxPayloadBytes = int(uint64(^uint32(0)) + 1)
		_, err := NewNativeFast[testValue](o)
		assertCacheReason(t, err, ReasonConfiguration)
	}
}

func TestConstructorsRejectCleanupUnsafeNamespaces(t *testing.T) {
	for _, namespace := range []string{
		"tenant*", "tenant?", "tenant[1]", "tenant\\prod", "tenant value",
		" tenant", "tenant ", "tenant:{tag}", "tenant::prod", "tenant\nprod",
	} {
		t.Run(strconv.Quote(namespace), func(t *testing.T) {
			o := validOptions(t)
			o.Namespace = namespace
			_, err := NewNativeFast[testValue](o)
			assertCacheReason(t, err, ReasonConfiguration)
		})
	}
}

func TestConstructorsRejectUnsupportedRootShapes(t *testing.T) {
	tests := []struct {
		name      string
		construct func(Options) error
	}{
		{name: "pointer", construct: func(o Options) error { _, err := NewNativeFast[*testValue](o); return err }},
		{name: "map", construct: func(o Options) error { _, err := NewNativeFast[map[string]string](o); return err }},
		{name: "non-byte-slice", construct: func(o Options) error { _, err := NewNativeFast[[]int](o); return err }},
		{name: "array", construct: func(o Options) error { _, err := NewNativeFast[[1]byte](o); return err }},
		{name: "complex", construct: func(o Options) error { _, err := NewNativeFast[complex64](o); return err }},
		{name: "interface", construct: func(o Options) error { _, err := NewNativeFast[any](o); return err }},
		{name: "function", construct: func(o Options) error { _, err := NewNativeFast[func()](o); return err }},
		{name: "channel", construct: func(o Options) error { _, err := NewNativeFast[chan int](o); return err }},
		{name: "unsafe-pointer", construct: func(o Options) error { _, err := NewNativeFast[unsafe.Pointer](o); return err }},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			o := validOptions(t)
			o.Register = func(*fory.Fory) error { return nil }
			assertCacheReason(t, tc.construct(o), ReasonUnsupportedValue)
		})
	}
}

func TestRegistrationFailureIsSanitized(t *testing.T) {
	const marker = "registration-secret-marker"
	for _, tc := range []struct {
		name     string
		register Registration
	}{
		{name: "error", register: func(*fory.Fory) error { return errors.New(marker) }},
		{name: "panic", register: func(*fory.Fory) error { panic(marker) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			o := validOptions(t)
			o.Register = tc.register
			_, err := NewNativeFast[testValue](o)
			assertCacheReason(t, err, ReasonRegistration)
			assertErrorRedacted(t, err, marker)
		})
	}
}

func TestCacheErrorExposesStableAccessors(t *testing.T) {
	err := &CacheError{
		operation: "decode",
		profile:   ProfileNativeFast,
		reason:    ReasonSchemaMismatch,
		cause:     errProviderFailed,
	}
	if err.Operation() != "decode" || err.Profile() != ProfileNativeFast || err.Reason() != ReasonSchemaMismatch {
		t.Fatalf("accessors = %q/%q/%q", err.Operation(), err.Profile(), err.Reason())
	}
	if !errors.Is(err, errProviderFailed) {
		t.Fatalf("unwrap = %v", errors.Unwrap(err))
	}
}

func TestBTFVLayoutAndValidation(t *testing.T) {
	encoded := wrap(ProfileNativeFast, 7, []byte{1, 2, 3})
	if len(encoded) != 17 {
		t.Fatalf("length = %d", len(encoded))
	}
	if string(encoded[:4]) != "BTFV" || encoded[4] != 1 || encoded[5] != 1 {
		t.Fatalf("header = %x", encoded[:14])
	}
	if binary.BigEndian.Uint32(encoded[6:10]) != 7 || binary.BigEndian.Uint32(encoded[10:14]) != 3 {
		t.Fatalf("metadata = %x", encoded[6:14])
	}
	payload, err := unwrap(ProfileNativeFast, 7, encoded, 3)
	if err != nil {
		t.Fatal(err)
	}
	if string(payload) != string([]byte{1, 2, 3}) {
		t.Fatalf("payload = %x", payload)
	}
}

func TestBTFVRejectsMalformedInput(t *testing.T) {
	base := wrap(ProfileNativeFast, 7, []byte{1, 2, 3})
	tests := []struct {
		name   string
		mutate func([]byte) []byte
		reason Reason
	}{
		{name: "too-short", mutate: func([]byte) []byte { return []byte("BTFV") }, reason: ReasonInvalidMagic},
		{name: "magic", mutate: func(b []byte) []byte { b[0] = 'X'; return b }, reason: ReasonInvalidMagic},
		{name: "version", mutate: func(b []byte) []byte { b[4] = 2; return b }, reason: ReasonUnsupportedVersion},
		{name: "format", mutate: func(b []byte) []byte { b[5] = 2; return b }, reason: ReasonFormatMismatch},
		{name: "schema", mutate: func(b []byte) []byte { binary.BigEndian.PutUint32(b[6:10], 8); return b }, reason: ReasonSchemaMismatch},
		{name: "declared-oversize", mutate: func(b []byte) []byte { binary.BigEndian.PutUint32(b[10:14], 4); return b }, reason: ReasonPayloadTooLarge},
		{name: "truncated", mutate: func(b []byte) []byte { return b[:len(b)-1] }, reason: ReasonLengthMismatch},
		{name: "trailing", mutate: func(b []byte) []byte { return append(b, 0) }, reason: ReasonPayloadTooLarge},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			bad := tc.mutate(append([]byte(nil), base...))
			_, err := unwrap(ProfileNativeFast, 7, bad, 3)
			assertCacheReason(t, err, tc.reason)
		})
	}
}

func TestBTFVRejectsTotalPayloadBeforeParsing(t *testing.T) {
	_, err := unwrap(ProfileNativeFast, 7, make([]byte, 18), 3)
	assertCacheReason(t, err, ReasonPayloadTooLarge)
}

func TestInvalidLogicalKeyUsesRedisSentinel(t *testing.T) {
	cache, err := NewNativeFast[testValue](validOptions(t))
	if err != nil {
		t.Fatal(err)
	}
	_, err = cache.key(" ")
	if !errors.Is(err, btredis.ErrInvalidKey) {
		t.Fatalf("error = %v", err)
	}
}

func assertCacheReason(t *testing.T, err error, want Reason) {
	t.Helper()
	var cacheErr *CacheError
	if !errors.As(err, &cacheErr) {
		t.Fatalf("error = %v, want *CacheError", err)
	}
	if cacheErr.Reason() != want {
		t.Fatalf("reason = %q, want %q", cacheErr.Reason(), want)
	}
}

func assertErrorRedacted(t *testing.T, err error, markers ...string) {
	t.Helper()
	for _, marker := range markers {
		if strings.Contains(err.Error(), marker) {
			t.Fatalf("error leaked %q: %v", marker, err)
		}
		if cause := errors.Unwrap(err); cause != nil && strings.Contains(cause.Error(), marker) {
			t.Fatalf("unwrapped error leaked %q: %v", marker, cause)
		}
	}
}
