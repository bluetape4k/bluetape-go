package redisfory

import (
	"context"
	"encoding/binary"
	"errors"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"
	"unsafe"

	"github.com/apache/fory/go/fory"
	"github.com/bluetape4k/bluetape-go/cache"
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
	}{
		{name: "fast", new: NewNativeFast[testValue], profile: ProfileNativeFast},
		{name: "compatible", new: NewNativeCompatible[testValue], profile: ProfileNativeCompatible},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cache, err := tc.new(validOptions(t))
			if err != nil {
				t.Fatal(err)
			}
			if cache.profile != tc.profile {
				t.Fatalf("profile = %q", cache.profile)
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

type fakeCommandClient struct {
	getRange func(context.Context, string, int64, int64) *redis.StringCmd
	get      func(context.Context, string) *redis.StringCmd
	exists   func(context.Context, ...string) *redis.IntCmd
	set      func(context.Context, string, any, time.Duration) *redis.StatusCmd
	del      func(context.Context, ...string) *redis.IntCmd
}

func (f *fakeCommandClient) GetRange(ctx context.Context, key string, start, end int64) *redis.StringCmd {
	if f.getRange != nil {
		return f.getRange(ctx, key, start, end)
	}
	return f.get(ctx, key)
}

func (f *fakeCommandClient) Exists(ctx context.Context, keys ...string) *redis.IntCmd {
	return f.exists(ctx, keys...)
}

func (f *fakeCommandClient) Set(ctx context.Context, key string, value any, ttl time.Duration) *redis.StatusCmd {
	return f.set(ctx, key, value, ttl)
}

func (f *fakeCommandClient) Del(ctx context.Context, keys ...string) *redis.IntCmd {
	return f.del(ctx, keys...)
}

type fakeValueCodec[V any] struct {
	serialize   func(V) ([]byte, error)
	deserialize func([]byte) (V, error)
}

func (f fakeValueCodec[V]) Serialize(value V) ([]byte, error) { return f.serialize(value) }
func (f fakeValueCodec[V]) Deserialize(raw []byte) (V, error) { return f.deserialize(raw) }

func unitCache[V any](client commandClient, codec valueCodec[V]) *ValueCache[V] {
	builder, err := btredis.NewKeyBuilder("bluetape:cache:fory")
	if err != nil {
		panic(err)
	}
	builder, err = builder.Structural("unit", "g1")
	if err != nil {
		panic(err)
	}
	return &ValueCache[V]{
		client: client, keys: builder, state: &cacheState[V]{codec: codec},
		profile: ProfileNativeFast, generation: 1, maxPayload: 64,
	}
}

func TestValueCacheSetStoresBTFVWithTTL(t *testing.T) {
	var calls int
	client := &fakeCommandClient{set: func(_ context.Context, key string, value any, ttl time.Duration) *redis.StatusCmd {
		calls++
		if key != "bluetape:cache:fory:unit:g1:item:42" || ttl != time.Minute {
			t.Fatalf("key/ttl = %q/%s", key, ttl)
		}
		raw, ok := value.([]byte)
		if !ok || string(raw[:4]) != "BTFV" || string(raw[14:]) != "encoded" {
			t.Fatalf("value = %T %x", value, value)
		}
		return redis.NewStatusResult("OK", nil)
	}}
	codec := fakeValueCodec[string]{
		serialize:   func(string) ([]byte, error) { return []byte("encoded"), nil },
		deserialize: func([]byte) (string, error) { return "", nil },
	}
	if err := unitCache[string](client, codec).Set(context.Background(), "item:42", "value", time.Minute); err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Fatalf("SET calls = %d", calls)
	}
}

func TestValueCacheSetRechecksContextAfterSerialization(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	var calls int
	client := &fakeCommandClient{set: func(context.Context, string, any, time.Duration) *redis.StatusCmd {
		calls++
		return redis.NewStatusResult("OK", nil)
	}}
	codec := fakeValueCodec[string]{
		serialize:   func(string) ([]byte, error) { cancel(); return []byte("encoded"), nil },
		deserialize: func([]byte) (string, error) { return "", nil },
	}
	err := unitCache[string](client, codec).Set(ctx, "key", "value", time.Second)
	if !errors.Is(err, context.Canceled) || calls != 0 {
		t.Fatalf("error/calls = %v/%d", err, calls)
	}
}

func TestValueCacheGetMapsRedisNilToCacheMiss(t *testing.T) {
	client := &fakeCommandClient{get: func(context.Context, string) *redis.StringCmd {
		return redis.NewStringResult("", redis.Nil)
	}}
	codec := fakeValueCodec[string]{serialize: func(string) ([]byte, error) { return nil, nil }, deserialize: func([]byte) (string, error) { return "", nil }}
	_, err := unitCache[string](client, codec).Get(context.Background(), "missing")
	if !errors.Is(err, cache.ErrCacheMiss) {
		t.Fatalf("error = %v", err)
	}
}

func TestValueCacheGetValidatesBeforeDecode(t *testing.T) {
	var decodes atomic.Int32
	client := &fakeCommandClient{get: func(context.Context, string) *redis.StringCmd {
		return redis.NewStringResult("not-btfv", nil)
	}}
	codec := fakeValueCodec[string]{
		serialize:   func(string) ([]byte, error) { return nil, nil },
		deserialize: func([]byte) (string, error) { decodes.Add(1); return "", nil },
	}
	_, err := unitCache[string](client, codec).Get(context.Background(), "key")
	assertCacheReason(t, err, ReasonInvalidMagic)
	if decodes.Load() != 0 {
		t.Fatalf("deserialize calls = %d", decodes.Load())
	}
}

func TestValueCacheGetBoundsRedisReadBeforeDecode(t *testing.T) {
	var decodes atomic.Int32
	client := &fakeCommandClient{getRange: func(_ context.Context, _ string, start, end int64) *redis.StringCmd {
		if start != 0 || end != envelopeHeaderSize+64 {
			t.Fatalf("GETRANGE bounds = %d..%d", start, end)
		}
		return redis.NewStringResult(string(make([]byte, envelopeHeaderSize+65)), nil)
	}}
	codec := fakeValueCodec[string]{
		serialize:   func(string) ([]byte, error) { return nil, nil },
		deserialize: func([]byte) (string, error) { decodes.Add(1); return "", nil },
	}
	_, err := unitCache[string](client, codec).Get(context.Background(), "key")
	assertCacheReason(t, err, ReasonPayloadTooLarge)
	if decodes.Load() != 0 {
		t.Fatalf("deserialize calls = %d", decodes.Load())
	}
}

func TestValueCacheGetRechecksContextAfterRedisReadBeforeDecode(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	var decodes atomic.Int32
	client := &fakeCommandClient{get: func(context.Context, string) *redis.StringCmd {
		cancel()
		return redis.NewStringResult(string(wrap(ProfileNativeFast, 1, []byte("encoded"))), nil)
	}}
	codec := fakeValueCodec[string]{
		serialize:   func(string) ([]byte, error) { return nil, nil },
		deserialize: func([]byte) (string, error) { decodes.Add(1); return "", nil },
	}
	_, err := unitCache[string](client, codec).Get(ctx, "key")
	if !errors.Is(err, context.Canceled) || decodes.Load() != 0 {
		t.Fatalf("error/decodes = %v/%d", err, decodes.Load())
	}
}

func TestValueCacheDeleteValidatesKeyAndIsIdempotent(t *testing.T) {
	var calls int
	client := &fakeCommandClient{del: func(_ context.Context, _ ...string) *redis.IntCmd {
		calls++
		return redis.NewIntResult(0, nil)
	}}
	codec := fakeValueCodec[string]{serialize: func(string) ([]byte, error) { return nil, nil }, deserialize: func([]byte) (string, error) { return "", nil }}
	c := unitCache[string](client, codec)
	if err := c.Delete(context.Background(), " "); !errors.Is(err, btredis.ErrInvalidKey) {
		t.Fatalf("invalid key error = %v", err)
	}
	if err := c.Delete(context.Background(), "key"); err != nil || calls != 1 {
		t.Fatalf("delete error/calls = %v/%d", err, calls)
	}
}

func TestValueCacheMethodsNormalizeNilContext(t *testing.T) {
	client := &fakeCommandClient{
		get: func(ctx context.Context, _ string) *redis.StringCmd {
			if ctx == nil {
				t.Fatal("nil GET context")
			}
			return redis.NewStringResult("", redis.Nil)
		},
		set: func(ctx context.Context, _ string, _ any, _ time.Duration) *redis.StatusCmd {
			if ctx == nil {
				t.Fatal("nil SET context")
			}
			return redis.NewStatusResult("OK", nil)
		},
		del: func(ctx context.Context, _ ...string) *redis.IntCmd {
			if ctx == nil {
				t.Fatal("nil DEL context")
			}
			return redis.NewIntResult(0, nil)
		},
	}
	codec := fakeValueCodec[string]{serialize: func(string) ([]byte, error) { return []byte("x"), nil }, deserialize: func([]byte) (string, error) { return "x", nil }}
	c := unitCache[string](client, codec)
	var nilCtx context.Context
	if err := c.Set(nilCtx, "key", "value", time.Second); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Get(nilCtx, "missing"); !errors.Is(err, cache.ErrCacheMiss) {
		t.Fatal(err)
	}
	if err := c.Delete(nilCtx, "key"); err != nil {
		t.Fatal(err)
	}
}

func TestValueCacheCommandContextErrorsRemainInspectable(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	client := &fakeCommandClient{get: func(context.Context, string) *redis.StringCmd {
		cancel()
		return redis.NewStringResult("", errors.New("provider canceled request"))
	}}
	codec := fakeValueCodec[string]{serialize: func(string) ([]byte, error) { return nil, nil }, deserialize: func([]byte) (string, error) { return "", nil }}
	_, err := unitCache[string](client, codec).Get(ctx, "secret-key")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v", err)
	}
	assertErrorRedacted(t, err, "provider canceled request", "secret-key")
}

func TestValueCacheCommandDeadlineErrorsRemainInspectable(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	client := &fakeCommandClient{get: func(ctx context.Context, _ string) *redis.StringCmd {
		<-ctx.Done()
		return redis.NewStringResult("", errors.New("provider deadline detail"))
	}}
	codec := fakeValueCodec[string]{serialize: func(string) ([]byte, error) { return nil, nil }, deserialize: func([]byte) (string, error) { return "", nil }}
	_, err := unitCache[string](client, codec).Get(ctx, "deadline-secret-key")
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("error = %v", err)
	}
	assertErrorRedacted(t, err, "provider deadline detail", "deadline-secret-key")
}

func TestValueCacheMethodsSanitizeRedisProviderErrors(t *testing.T) {
	const marker = "redis-provider-secret"
	providerErr := errors.New(marker)
	client := &fakeCommandClient{
		get: func(context.Context, string) *redis.StringCmd { return redis.NewStringResult("", providerErr) },
		set: func(context.Context, string, any, time.Duration) *redis.StatusCmd {
			return redis.NewStatusResult("", providerErr)
		},
		del: func(context.Context, ...string) *redis.IntCmd { return redis.NewIntResult(0, providerErr) },
	}
	codec := fakeValueCodec[string]{serialize: func(string) ([]byte, error) { return []byte("x"), nil }, deserialize: func([]byte) (string, error) { return "", nil }}
	c := unitCache[string](client, codec)
	setErr := c.Set(context.Background(), "secret-key", "value", time.Second)
	_, getErr := c.Get(context.Background(), "secret-key")
	delErr := c.Delete(context.Background(), "secret-key")
	for _, err := range []error{setErr, getErr, delErr} {
		assertErrorRedacted(t, err, marker, "secret-key")
		if !errors.Is(err, errProviderFailed) {
			t.Fatalf("error = %v", err)
		}
	}
}

func TestZeroValueCacheReturnsUninitialized(t *testing.T) {
	var c ValueCache[string]
	if err := c.Set(context.Background(), "key", "value", time.Second); err == nil {
		t.Fatal("Set succeeded")
	}
	if _, err := c.Get(context.Background(), "key"); err == nil {
		t.Fatal("Get succeeded")
	}
	if err := c.Delete(context.Background(), "key"); err == nil {
		t.Fatal("Delete succeeded")
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
