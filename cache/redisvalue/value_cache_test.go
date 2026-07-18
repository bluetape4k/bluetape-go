package redisvalue

import (
	"context"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/bluetape4k/bluetape-go/cache"
	btredis "github.com/bluetape4k/bluetape-go/redis"
	"github.com/bluetape4k/bluetape-go/serialization"
	"github.com/redis/go-redis/v9"
)

type valueTestRecord struct {
	Name string `json:"name"`
}

type fakeCommandClient struct {
	readBounded func(context.Context, string, int64) ([]byte, bool, error)
	getRange    func(context.Context, string, int64, int64) *redis.StringCmd
	exists      func(context.Context, ...string) *redis.IntCmd
	set         func(context.Context, string, any, time.Duration) *redis.StatusCmd
	del         func(context.Context, ...string) *redis.IntCmd
	scan        func(context.Context, uint64, string, int64) *redis.ScanCmd
	unlink      func(context.Context, ...string) *redis.IntCmd
}

func (f *fakeCommandClient) ReadBounded(ctx context.Context, key string, end int64) ([]byte, bool, error) {
	if f.readBounded != nil {
		return f.readBounded(ctx, key, end)
	}
	encoded, err := f.GetRange(ctx, key, 0, end).Bytes()
	if errors.Is(err, redis.Nil) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	if len(encoded) > 0 {
		return encoded, true, nil
	}
	present, err := f.Exists(ctx, key).Result()
	return encoded, present != 0, err
}

func (f *fakeCommandClient) GetRange(ctx context.Context, key string, start, end int64) *redis.StringCmd {
	if f.getRange == nil {
		panic("unexpected GetRange")
	}
	return f.getRange(ctx, key, start, end)
}

func (f *fakeCommandClient) Exists(ctx context.Context, keys ...string) *redis.IntCmd {
	if f.exists == nil {
		panic("unexpected Exists")
	}
	return f.exists(ctx, keys...)
}

func (f *fakeCommandClient) Set(ctx context.Context, key string, value any, ttl time.Duration) *redis.StatusCmd {
	if f.set == nil {
		panic("unexpected Set")
	}
	return f.set(ctx, key, value, ttl)
}

func (f *fakeCommandClient) Del(ctx context.Context, keys ...string) *redis.IntCmd {
	if f.del == nil {
		panic("unexpected Del")
	}
	return f.del(ctx, keys...)
}

func (f *fakeCommandClient) Scan(ctx context.Context, cursor uint64, match string, count int64) *redis.ScanCmd {
	if f.scan == nil {
		panic("unexpected Scan")
	}
	return f.scan(ctx, cursor, match, count)
}

func (f *fakeCommandClient) Unlink(ctx context.Context, keys ...string) *redis.IntCmd {
	if f.unlink == nil {
		panic("unexpected Unlink")
	}
	return f.unlink(ctx, keys...)
}

type serializerFuncs[V any] struct {
	marshal   func(V) ([]byte, error)
	unmarshal func([]byte) (V, error)
}

func (s *serializerFuncs[V]) Marshal(value V) ([]byte, error) {
	return s.marshal(value)
}

func (s *serializerFuncs[V]) Unmarshal(data []byte) (V, error) {
	return s.unmarshal(data)
}

func unitValueCache[V any](client commandClient, serializer serialization.Serializer[V], config ValueConfig) *ValueCache[V] {
	builder, err := newValueKeyBuilder("catalog")
	if err != nil {
		panic(err)
	}
	return &ValueCache[V]{
		client:     client,
		serializer: serializer,
		keys:       builder,
		namespace:  "catalog",
		config:     config,
	}
}

func TestValueCacheConstructorValidatesAndCopiesInputs(t *testing.T) {
	client := redis.NewClient(&redis.Options{Addr: "127.0.0.1:1"})
	t.Cleanup(func() { _ = client.Close() })
	config := ValueConfig{RemoteTTL: time.Minute, MaxValueBytes: 32, ClearBatchSize: 5}

	c, err := NewValueCache(ValueOptions[string]{
		Client:     client,
		Namespace:  "catalog.prod",
		Serializer: serialization.StringSerializer{},
		Config:     &config,
	})
	if err != nil {
		t.Fatal(err)
	}
	config.RemoteTTL = 2 * time.Minute
	config.MaxValueBytes = 64
	if c.config.RemoteTTL != time.Minute || c.config.MaxValueBytes != 32 || c.namespace != "catalog.prod" {
		t.Fatalf("constructor retained mutable config: %+v", c.config)
	}
	key, err := c.key("sku:42")
	if err != nil {
		t.Fatal(err)
	}
	if key.Value != "bluetape:cache:value:catalog.prod:sku:42" {
		t.Fatalf("physical key = %q", key.Value)
	}
}

func TestValueCacheConstructorUsesValueDefaults(t *testing.T) {
	client := redis.NewClient(&redis.Options{Addr: "127.0.0.1:1"})
	t.Cleanup(func() { _ = client.Close() })
	c, err := NewValueCache(ValueOptions[string]{
		Client:     client,
		Namespace:  "catalog",
		Serializer: serialization.StringSerializer{},
	})
	if err != nil {
		t.Fatal(err)
	}
	if c.config != DefaultConfig().Value {
		t.Fatalf("config = %+v", c.config)
	}
}

func TestValueCacheConstructorRejectsInvalidDependencies(t *testing.T) {
	client := redis.NewClient(&redis.Options{Addr: "127.0.0.1:1"})
	t.Cleanup(func() { _ = client.Close() })
	var typedNilSerializer *serializerFuncs[string]
	invalidConfig := DefaultConfig().Value
	invalidConfig.MaxValueBytes = 0

	tests := []struct {
		name    string
		options ValueOptions[string]
		wantIs  error
	}{
		{name: "nil client", options: ValueOptions[string]{Namespace: "catalog", Serializer: serialization.StringSerializer{}}},
		{name: "nil serializer", options: ValueOptions[string]{Client: client, Namespace: "catalog"}},
		{name: "typed nil serializer", options: ValueOptions[string]{Client: client, Namespace: "catalog", Serializer: typedNilSerializer}},
		{name: "invalid namespace", options: ValueOptions[string]{Client: client, Namespace: "catalog*", Serializer: serialization.StringSerializer{}}, wantIs: btredis.ErrInvalidKey},
		{name: "invalid config", options: ValueOptions[string]{Client: client, Namespace: "catalog", Serializer: serialization.StringSerializer{}, Config: &invalidConfig}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewValueCache(tt.options)
			if !hasReason(err, ReasonConfiguration) {
				t.Fatalf("NewValueCache() = %v", err)
			}
			if tt.wantIs != nil && !errors.Is(err, tt.wantIs) {
				t.Fatalf("NewValueCache() = %v, want errors.Is(%v)", err, tt.wantIs)
			}
		})
	}
}

func TestValueCacheGetUsesBoundedRead(t *testing.T) {
	var end int64
	var existsCalls int
	client := &fakeCommandClient{
		getRange: func(_ context.Context, key string, start, gotEnd int64) *redis.StringCmd {
			if key != "bluetape:cache:value:catalog:sku:42" || start != 0 {
				t.Fatalf("GetRange(%q, %d, %d)", key, start, gotEnd)
			}
			end = gotEnd
			return redis.NewStringResult(`{"name":"keyboard"}`, nil)
		},
		exists: func(context.Context, ...string) *redis.IntCmd {
			existsCalls++
			return redis.NewIntResult(1, nil)
		},
	}
	c := unitValueCache[valueTestRecord](client, serialization.NewJSONSerializer[valueTestRecord](), ValueConfig{RemoteTTL: time.Hour, MaxValueBytes: 32, ClearBatchSize: 100})

	got, err := c.Get(context.Background(), "sku:42")
	if err != nil || got.Name != "keyboard" || end != 32 || existsCalls != 0 {
		t.Fatalf("get = %+v, %v; end=%d exists=%d", got, err, end, existsCalls)
	}
}

func TestValueCacheGetDistinguishesEmptyValueFromMiss(t *testing.T) {
	for _, tt := range []struct {
		name   string
		exists int64
		want   string
		wantIs error
	}{
		{name: "empty value", exists: 1, want: ""},
		{name: "miss", exists: 0, wantIs: cache.ErrCacheMiss},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var existsCalls int
			client := &fakeCommandClient{
				getRange: func(context.Context, string, int64, int64) *redis.StringCmd {
					return redis.NewStringResult("", nil)
				},
				exists: func(context.Context, ...string) *redis.IntCmd {
					existsCalls++
					return redis.NewIntResult(tt.exists, nil)
				},
			}
			c := unitValueCache[string](client, serialization.StringSerializer{}, ValueConfig{RemoteTTL: time.Hour, MaxValueBytes: 8, ClearBatchSize: 10})
			got, err := c.Get(context.Background(), "key")
			if !errors.Is(err, tt.wantIs) || got != tt.want || existsCalls != 1 {
				t.Fatalf("Get() = %q/%v, exists=%d", got, err, existsCalls)
			}
		})
	}
}

func TestValueCacheGetRedisNilIsExactMiss(t *testing.T) {
	client := &fakeCommandClient{
		getRange: func(context.Context, string, int64, int64) *redis.StringCmd {
			return redis.NewStringResult("", redis.Nil)
		},
	}
	c := unitValueCache[string](client, serialization.StringSerializer{}, ValueConfig{RemoteTTL: time.Hour, MaxValueBytes: 8, ClearBatchSize: 10})
	_, err := c.Get(context.Background(), "key")
	if err != cache.ErrCacheMiss { //nolint:errorlint // Exact sentinel identity is the public contract.
		t.Fatalf("Get() error = %v, want exact cache.ErrCacheMiss", err)
	}
}

func TestValueCacheGetRejectsOversizeBeforeUnmarshal(t *testing.T) {
	var unmarshalCalls int
	serializer := &serializerFuncs[string]{
		marshal: func(value string) ([]byte, error) { return []byte(value), nil },
		unmarshal: func(data []byte) (string, error) {
			unmarshalCalls++
			return string(data), nil
		},
	}
	client := &fakeCommandClient{
		getRange: func(context.Context, string, int64, int64) *redis.StringCmd {
			return redis.NewStringResult("12345", nil)
		},
	}
	c := unitValueCache[string](client, serializer, ValueConfig{RemoteTTL: time.Hour, MaxValueBytes: 4, ClearBatchSize: 10})
	_, err := c.Get(context.Background(), "key")
	if !hasReason(err, ReasonPayloadTooLarge) || unmarshalCalls != 0 {
		t.Fatalf("Get() = %v, unmarshal calls=%d", err, unmarshalCalls)
	}
}

func TestValueCacheGetPreservesMalformedPayloadCauseAndRedactsIt(t *testing.T) {
	cause := errors.New("decoder payload-secret at 127.0.0.1 for raw:key")
	var unmarshalCalls int
	serializer := &serializerFuncs[string]{
		marshal: func(value string) ([]byte, error) { return []byte(value), nil },
		unmarshal: func([]byte) (string, error) {
			unmarshalCalls++
			return "", cause
		},
	}
	client := &fakeCommandClient{
		getRange: func(context.Context, string, int64, int64) *redis.StringCmd {
			return redis.NewStringResult("bad", nil)
		},
	}
	c := unitValueCache[string](client, serializer, ValueConfig{RemoteTTL: time.Hour, MaxValueBytes: 8, ClearBatchSize: 10})
	_, err := c.Get(context.Background(), "raw:key")
	if !hasReason(err, ReasonInvalidPayload) || !errors.Is(err, cause) || unmarshalCalls != 1 {
		t.Fatalf("Get() = %v, unmarshal calls=%d", err, unmarshalCalls)
	}
	for _, secret := range []string{"payload-secret", "127.0.0.1", "raw:key", "decoder"} {
		if strings.Contains(err.Error(), secret) {
			t.Fatalf("Error() leaked %q: %q", secret, err.Error())
		}
	}
}

func TestValueCacheSetMarshalsBeforeRedisAndNormalizesTTL(t *testing.T) {
	var marshaled bool
	serializer := &serializerFuncs[string]{
		marshal: func(value string) ([]byte, error) {
			marshaled = true
			return []byte(value), nil
		},
		unmarshal: func(data []byte) (string, error) { return string(data), nil },
	}
	client := &fakeCommandClient{
		set: func(_ context.Context, key string, value any, ttl time.Duration) *redis.StatusCmd {
			if !marshaled || key != "bluetape:cache:value:catalog:key" || string(value.([]byte)) != "value" || ttl != time.Millisecond {
				t.Fatalf("Set(%q, %v, %s), marshaled=%v", key, value, ttl, marshaled)
			}
			return redis.NewStatusResult("OK", nil)
		},
	}
	c := unitValueCache[string](client, serializer, ValueConfig{RemoteTTL: time.Hour, MaxValueBytes: 8, ClearBatchSize: 10})
	if err := c.Set(context.Background(), "key", "value", time.Nanosecond); err != nil {
		t.Fatal(err)
	}
}

func TestValueCacheSetRejectsMarshalAndOversizeBeforeRedis(t *testing.T) {
	cause := errors.New("marshal secret payload at 127.0.0.1")
	for _, tt := range []struct {
		name       string
		marshal    func(string) ([]byte, error)
		wantReason Reason
		wantIs     error
	}{
		{name: "marshal failure", marshal: func(string) ([]byte, error) { return nil, cause }, wantReason: ReasonSerialization, wantIs: cause},
		{name: "oversize", marshal: func(string) ([]byte, error) { return []byte("12345"), nil }, wantReason: ReasonPayloadTooLarge},
	} {
		t.Run(tt.name, func(t *testing.T) {
			serializer := &serializerFuncs[string]{marshal: tt.marshal, unmarshal: func(data []byte) (string, error) { return string(data), nil }}
			c := unitValueCache[string](&fakeCommandClient{}, serializer, ValueConfig{RemoteTTL: time.Hour, MaxValueBytes: 4, ClearBatchSize: 10})
			err := c.Set(context.Background(), "key", "value", time.Second)
			if !hasReason(err, tt.wantReason) || (tt.wantIs != nil && !errors.Is(err, tt.wantIs)) {
				t.Fatalf("Set() = %v", err)
			}
			for _, secret := range []string{"secret payload", "127.0.0.1"} {
				if strings.Contains(err.Error(), secret) {
					t.Fatalf("Error() leaked %q: %q", secret, err.Error())
				}
			}
		})
	}
}

func TestValueCacheMutationsPreserveCommitUnknown(t *testing.T) {
	providerErr := errors.New("provider 127.0.0.1:6379 raw:key")
	client := &fakeCommandClient{
		set: func(context.Context, string, any, time.Duration) *redis.StatusCmd {
			return redis.NewStatusResult("", providerErr)
		},
		del: func(context.Context, ...string) *redis.IntCmd {
			return redis.NewIntResult(0, providerErr)
		},
	}
	c := unitValueCache[string](client, serialization.StringSerializer{}, ValueConfig{RemoteTTL: time.Hour, MaxValueBytes: 16, ClearBatchSize: 10})

	for name, err := range map[string]error{
		"set":    c.Set(context.Background(), "raw:key", "value", time.Second),
		"delete": c.Delete(context.Background(), "raw:key"),
	} {
		if !hasReason(err, ReasonProviderFailure) || !errors.Is(err, providerErr) || !errors.Is(err, btredis.ErrCommitUnknown) {
			t.Fatalf("%s error = %v", name, err)
		}
		var opErr *btredis.OpError
		if !errors.As(err, &opErr) {
			t.Fatalf("%s does not wrap redis.OpError: %v", name, err)
		}
		for _, secret := range []string{"raw:key", "127.0.0.1", "6379"} {
			if strings.Contains(err.Error(), secret) {
				t.Fatalf("%s Error() leaked %q: %q", name, secret, err.Error())
			}
		}
	}
}

func TestValueCacheInvalidInputAndCancellationCauseNoCommand(t *testing.T) {
	c := unitValueCache[string](&fakeCommandClient{}, serialization.StringSerializer{}, ValueConfig{RemoteTTL: time.Hour, MaxValueBytes: 16, ClearBatchSize: 10})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if _, err := c.Get(ctx, "key"); !errors.Is(err, context.Canceled) {
		t.Fatalf("Get(canceled) = %v", err)
	}
	if err := c.Set(context.Background(), "key", "value", -1); !errors.Is(err, btredis.ErrInvalidTTL) {
		t.Fatalf("Set(invalid ttl) = %v", err)
	}
	if err := c.Delete(context.Background(), " "); !errors.Is(err, btredis.ErrInvalidKey) {
		t.Fatalf("Delete(invalid key) = %v", err)
	}
}

func TestValueCacheSetDefaultUsesCopiedTTL(t *testing.T) {
	var gotTTL time.Duration
	client := &fakeCommandClient{
		set: func(_ context.Context, _ string, _ any, ttl time.Duration) *redis.StatusCmd {
			gotTTL = ttl
			return redis.NewStatusResult("OK", nil)
		},
	}
	c := unitValueCache[string](client, serialization.StringSerializer{}, ValueConfig{RemoteTTL: 7 * time.Second, MaxValueBytes: 16, ClearBatchSize: 10})
	if err := c.SetDefault(context.Background(), "key", "value"); err != nil {
		t.Fatal(err)
	}
	if gotTTL != 7*time.Second {
		t.Fatalf("SetDefault ttl = %s", gotTTL)
	}
}

func TestValueCacheZeroValueReturnsUninitialized(t *testing.T) {
	var c ValueCache[string]
	if _, err := c.Get(context.Background(), "key"); !hasReason(err, ReasonUninitialized) {
		t.Fatalf("get = %v", err)
	}
	if err := c.Set(context.Background(), "key", "value", 0); !hasReason(err, ReasonUninitialized) {
		t.Fatalf("set = %v", err)
	}
	if err := c.SetDefault(context.Background(), "key", "value"); !hasReason(err, ReasonUninitialized) {
		t.Fatalf("set default = %v", err)
	}
	if err := c.Delete(context.Background(), "key"); !hasReason(err, ReasonUninitialized) {
		t.Fatalf("delete = %v", err)
	}
}

func TestValueCacheDifferentKeySerializerCallsProceedConcurrently(t *testing.T) {
	marshalEntered := make(chan struct{}, 2)
	marshalRelease := make(chan struct{})
	unmarshalEntered := make(chan struct{}, 2)
	unmarshalRelease := make(chan struct{})
	var marshalCalls atomic.Int64
	var unmarshalCalls atomic.Int64
	serializer := &serializerFuncs[string]{
		marshal: func(value string) ([]byte, error) {
			marshalCalls.Add(1)
			marshalEntered <- struct{}{}
			<-marshalRelease
			return []byte(value), nil
		},
		unmarshal: func(data []byte) (string, error) {
			unmarshalCalls.Add(1)
			unmarshalEntered <- struct{}{}
			<-unmarshalRelease
			return string(data), nil
		},
	}
	client := &fakeCommandClient{
		set: func(context.Context, string, any, time.Duration) *redis.StatusCmd {
			return redis.NewStatusResult("OK", nil)
		},
		getRange: func(_ context.Context, key string, _, _ int64) *redis.StringCmd {
			return redis.NewStringResult(key, nil)
		},
	}
	c := unitValueCache[string](client, serializer, ValueConfig{RemoteTTL: time.Hour, MaxValueBytes: 128, ClearBatchSize: 10})

	var setWG sync.WaitGroup
	setErrors := make(chan error, 2)
	for _, key := range []string{"a", "b"} {
		setWG.Add(1)
		go func(key string) {
			defer setWG.Done()
			setErrors <- c.Set(context.Background(), key, key, time.Second)
		}(key)
	}
	waitForEntries(t, marshalEntered, 2)
	close(marshalRelease)
	setWG.Wait()
	close(setErrors)
	for err := range setErrors {
		if err != nil {
			t.Fatal(err)
		}
	}

	var getWG sync.WaitGroup
	getErrors := make(chan error, 2)
	for _, key := range []string{"a", "b"} {
		getWG.Add(1)
		go func(key string) {
			defer getWG.Done()
			_, err := c.Get(context.Background(), key)
			getErrors <- err
		}(key)
	}
	waitForEntries(t, unmarshalEntered, 2)
	close(unmarshalRelease)
	getWG.Wait()
	close(getErrors)
	for err := range getErrors {
		if err != nil {
			t.Fatal(err)
		}
	}
	if marshalCalls.Load() != 2 || unmarshalCalls.Load() != 2 {
		t.Fatalf("serializer calls = %d/%d", marshalCalls.Load(), unmarshalCalls.Load())
	}
}

func waitForEntries(t *testing.T, entered <-chan struct{}, count int) {
	t.Helper()
	timer := time.NewTimer(time.Second)
	defer timer.Stop()
	for range count {
		select {
		case <-entered:
		case <-timer.C:
			t.Fatalf("only some serializer calls entered concurrently")
		}
	}
}
