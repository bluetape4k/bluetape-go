package redismap

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	btredis "github.com/bluetape4k/bluetape-go/redis"
	"github.com/bluetape4k/bluetape-go/serialization"
	"github.com/redis/go-redis/v9"
)

type fakeClient struct {
	mu           sync.Mutex
	values       map[string][]byte
	calls        []string
	lastKey      string
	lastPayload  []byte
	lastTTL      time.Duration
	setErr       error
	setNXErr     error
	deleteErr    error
	getErr       error
	evalErr      error
	evalResult   any
	evalOverride bool
	afterSet     func()
	afterSetNX   func()
	afterDelete  func()
	afterEval    func()
}

func newFakeClient() *fakeClient {
	return &fakeClient{values: make(map[string][]byte)}
}

func (f *fakeClient) Get(_ context.Context, key string) *redis.StringCmd {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, "get")
	f.lastKey = key
	if f.getErr != nil {
		return redis.NewStringResult("", f.getErr)
	}
	value, ok := f.values[key]
	if !ok {
		return redis.NewStringResult("", redis.Nil)
	}
	return redis.NewStringResult(string(append([]byte(nil), value...)), nil)
}

func (f *fakeClient) Set(_ context.Context, key string, value any, ttl time.Duration) *redis.StatusCmd {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, "set")
	f.lastKey, f.lastTTL, f.lastPayload = key, ttl, cloneBytes(value)
	if f.setErr != nil {
		return redis.NewStatusResult("", f.setErr)
	}
	f.values[key] = append([]byte(nil), f.lastPayload...)
	if f.afterSet != nil {
		f.afterSet()
	}
	return redis.NewStatusResult("OK", nil)
}

func (f *fakeClient) SetNX(_ context.Context, key string, value any, ttl time.Duration) *redis.BoolCmd {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, "setnx")
	f.lastKey, f.lastTTL, f.lastPayload = key, ttl, cloneBytes(value)
	if f.setNXErr != nil {
		return redis.NewBoolResult(false, f.setNXErr)
	}
	if _, exists := f.values[key]; exists {
		if f.afterSetNX != nil {
			f.afterSetNX()
		}
		return redis.NewBoolResult(false, nil)
	}
	f.values[key] = append([]byte(nil), f.lastPayload...)
	if f.afterSetNX != nil {
		f.afterSetNX()
	}
	return redis.NewBoolResult(true, nil)
}

func (f *fakeClient) Del(_ context.Context, keys ...string) *redis.IntCmd {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, "del")
	if len(keys) > 0 {
		f.lastKey = keys[0]
	}
	if f.deleteErr != nil {
		return redis.NewIntResult(0, f.deleteErr)
	}
	var removed int64
	for _, key := range keys {
		if _, ok := f.values[key]; ok {
			delete(f.values, key)
			removed++
		}
	}
	if f.afterDelete != nil {
		f.afterDelete()
	}
	return redis.NewIntResult(removed, nil)
}

func (f *fakeClient) Eval(_ context.Context, script string, keys []string, args ...any) *redis.Cmd {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, "eval")
	if len(keys) > 0 {
		f.lastKey = keys[0]
	}
	if f.afterEval != nil {
		f.afterEval()
	}
	if f.evalOverride {
		return redis.NewCmdResult(f.evalResult, f.evalErr)
	}
	if len(keys) == 0 {
		return redis.NewCmdResult(nil, errors.New("missing key"))
	}
	switch {
	case strings.Contains(script, "redis.call(\"DEL\""):
		value, ok := f.values[keys[0]]
		if !ok {
			return redis.NewCmdResult([]interface{}{int64(0)}, f.evalErr)
		}
		delete(f.values, keys[0])
		return redis.NewCmdResult([]interface{}{int64(1), append([]byte(nil), value...)}, f.evalErr)
	case strings.Contains(script, "ARGV[3]"):
		if len(args) != 3 {
			return redis.NewCmdResult(int64(9), f.evalErr)
		}
		expected, replacement := cloneBytes(args[0]), cloneBytes(args[1])
		current, exists := f.values[keys[0]]
		if !exists || !bytes.Equal(current, expected) {
			return redis.NewCmdResult(int64(0), f.evalErr)
		}
		f.values[keys[0]] = replacement
		return redis.NewCmdResult(int64(1), f.evalErr)
	default:
		return redis.NewCmdResult(f.evalResult, f.evalErr)
	}
}

func cloneBytes(value any) []byte {
	switch typed := value.(type) {
	case []byte:
		return append([]byte(nil), typed...)
	case string:
		return []byte(typed)
	default:
		return []byte(fmt.Sprint(typed))
	}
}

func mapForTest[V any](t *testing.T, fake Client, serializer serialization.Serializer[V]) *MapCache[V] {
	t.Helper()
	cache, err := New(fake, Options[V]{Namespace: "tenant:catalog", HashTag: "tenant", Serializer: serializer})
	if err != nil {
		t.Fatal(err)
	}
	return cache
}

func TestConstructorRejectsInvalidDependenciesAndZeroValue(t *testing.T) {
	var typedNilClient *fakeClient
	var typedNilSerializer *testSerializer
	if _, err := New[string](typedNilClient, Options[string]{Namespace: "catalog", Serializer: serialization.StringSerializer{}}); !errors.Is(err, ErrInvalidClient) {
		t.Fatalf("typed nil client error = %v", err)
	}
	for _, serializer := range []serialization.Serializer[string]{nil, typedNilSerializer} {
		if _, err := New[string](newFakeClient(), Options[string]{Namespace: "catalog", Serializer: serializer}); !errors.Is(err, ErrInvalidOptions) {
			t.Fatalf("invalid serializer error = %v", err)
		}
	}
	var zero MapCache[string]
	if _, _, err := zero.Get(context.Background(), "key"); !errors.Is(err, ErrUninitialized) {
		t.Fatalf("zero Get() = %v", err)
	}
}

func TestMapCacheRoundTripPreservesEntryKeyAndTTL(t *testing.T) {
	fake := newFakeClient()
	cache := mapForTest(t, fake, serialization.StringSerializer{})
	logicalKey := " sku:{42}:raw "
	if err := cache.Set(context.Background(), logicalKey, "value", 1500*time.Microsecond); err != nil {
		t.Fatal(err)
	}
	fake.mu.Lock()
	physicalKey, ttl := fake.lastKey, fake.lastTTL
	fake.mu.Unlock()
	if physicalKey != "tenant:catalog:map:{tenant}:"+logicalKey {
		t.Fatalf("physical key = %q", physicalKey)
	}
	if ttl != time.Millisecond {
		t.Fatalf("normalized ttl = %s", ttl)
	}
	got, hit, err := cache.Get(context.Background(), logicalKey)
	if err != nil || !hit || got != "value" {
		t.Fatalf("Get() = %q, %v, %v", got, hit, err)
	}
}

func TestMapCacheConditionalDeleteAndCAS(t *testing.T) {
	fake := newFakeClient()
	cache := mapForTest(t, fake, serialization.StringSerializer{})
	ctx := context.Background()
	if ok, err := cache.SetIfAbsent(ctx, "one", "first", 0); err != nil || !ok {
		t.Fatalf("first SetIfAbsent() = %v, %v", ok, err)
	}
	if ok, err := cache.SetIfAbsent(ctx, "one", "second", 0); err != nil || ok {
		t.Fatalf("second SetIfAbsent() = %v, %v", ok, err)
	}
	if ok, err := cache.CompareAndSet(ctx, "one", "first", "next", time.Second); err != nil || !ok {
		t.Fatalf("CompareAndSet() = %v, %v", ok, err)
	}
	got, hit, err := cache.GetAndDelete(ctx, "one")
	if err != nil || !hit || got != "next" {
		t.Fatalf("GetAndDelete() = %q, %v, %v", got, hit, err)
	}
	if err := cache.Delete(ctx, "one"); err != nil {
		t.Fatal(err)
	}
}

func TestMapCacheConcurrentCASHasOneWinner(t *testing.T) {
	fake := newFakeClient()
	cache := mapForTest(t, fake, serialization.StringSerializer{})
	if err := cache.Set(context.Background(), "race", "expected", 0); err != nil {
		t.Fatal(err)
	}
	const workers = 16
	results := make(chan bool, workers)
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ok, err := cache.CompareAndSet(context.Background(), "race", "expected", "winner", 0)
			if err != nil {
				t.Errorf("CompareAndSet() error = %v", err)
			}
			results <- ok
		}()
	}
	wg.Wait()
	close(results)
	var winners int
	for ok := range results {
		if ok {
			winners++
		}
	}
	if winners != 1 {
		t.Fatalf("CAS winners = %d, want 1", winners)
	}
}

func TestMapCacheCancellationAndRedaction(t *testing.T) {
	fake := newFakeClient()
	cache := mapForTest(t, fake, serialization.StringSerializer{})
	ctx, cancel := context.WithCancel(context.Background())
	fake.afterSet = cancel
	if err := cache.Set(ctx, "secret-key", "payload", time.Second); !errors.Is(err, context.Canceled) {
		t.Fatalf("post-dispatch Set() = %v", err)
	}
	fake.mu.Lock()
	fake.setErr = errors.New("raw redis secret-key")
	fake.afterSet = nil
	fake.mu.Unlock()
	err := cache.Set(context.Background(), "secret-key", "payload", time.Second)
	if !errors.Is(err, ErrCommitUnknown) || !errors.Is(err, btredis.ErrCommitUnknown) {
		t.Fatalf("Set() = %v, want commit unknown", err)
	}
	if strings.Contains(err.Error(), "raw redis") || strings.Contains(err.Error(), "secret-key") {
		t.Fatalf("Set() leaked sensitive text: %v", err)
	}
	if err := cache.Set(nil, "key", "value", 0); !errors.Is(err, ErrInvalidContext) { //nolint:staticcheck // nil context is the contract input under test.
		t.Fatalf("Set(nil) = %v", err)
	}
}

func TestMapCacheRejectsMalformedLuaResult(t *testing.T) {
	fake := newFakeClient()
	cache := mapForTest(t, fake, serialization.StringSerializer{})
	fake.mu.Lock()
	fake.evalOverride = true
	fake.evalResult = []interface{}{int64(1)}
	fake.mu.Unlock()
	if _, _, err := cache.GetAndDelete(context.Background(), "key"); !errors.Is(err, ErrMalformedResult) || !errors.Is(err, ErrCommitUnknown) {
		t.Fatalf("malformed GetAndDelete() = %v", err)
	}
}

func TestMapCacheRejectsNegativeTTLAndCodecFailures(t *testing.T) {
	fake := newFakeClient()
	marshalCache := mapForTest(t, fake, &marshalErrorSerializer{})
	if err := marshalCache.Set(context.Background(), "key", "value", -time.Millisecond); !errors.Is(err, btredis.ErrInvalidTTL) {
		t.Fatalf("negative TTL error = %v", err)
	}
	if err := marshalCache.Set(context.Background(), "key", "value", 0); !errors.Is(err, ErrSerialization) {
		t.Fatalf("marshal error = %v", err)
	}
	fake.values["tenant:catalog:map:{tenant}:key"] = []byte("payload")
	unmarshalCache := mapForTest(t, fake, &unmarshalErrorSerializer{})
	if _, _, err := unmarshalCache.Get(context.Background(), "key"); !errors.Is(err, ErrInvalidPayload) {
		t.Fatalf("unmarshal error = %v", err)
	}
}

type testSerializer struct{}

func (*testSerializer) Marshal(string) ([]byte, error)   { return nil, errors.New("marshal secret") }
func (*testSerializer) Unmarshal([]byte) (string, error) { return "", errors.New("unmarshal secret") }

type marshalErrorSerializer struct{}

func (*marshalErrorSerializer) Marshal(string) ([]byte, error) {
	return nil, errors.New("marshal secret")
}
func (*marshalErrorSerializer) Unmarshal(data []byte) (string, error) { return string(data), nil }

type unmarshalErrorSerializer struct{}

func (*unmarshalErrorSerializer) Marshal(value string) ([]byte, error) { return []byte(value), nil }
func (*unmarshalErrorSerializer) Unmarshal([]byte) (string, error) {
	return "", errors.New("unmarshal secret")
}

var _ serialization.Serializer[string] = (*testSerializer)(nil)
