package redisbucket

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	btredis "github.com/bluetape4k/bluetape-go/redis"
	"github.com/bluetape4k/bluetape-go/serialization"
	"github.com/redis/go-redis/v9"
)

type fakeClient struct {
	mu          sync.Mutex
	values      map[string][]byte
	calls       []string
	lastKey     string
	lastPayload []byte
	lastTTL     time.Duration
	setErr      error
	setNXErr    error
	deleteErr   error
	getErr      error
	evalResult  any
	evalErr     error
	afterSet    func()
	afterSetNX  func()
	afterDelete func()
	afterEval   func()
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
	f.lastKey = key
	f.lastTTL = ttl
	f.lastPayload = cloneBytes(value)
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
	f.lastKey = key
	f.lastTTL = ttl
	f.lastPayload = cloneBytes(value)
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
	if len(keys) > 0 {
		switch result := f.evalResult.(type) {
		case []interface{}:
			if len(result) > 0 && result[0] == int64(1) && strings.Contains(script, "redis.call(\"DEL\"") {
				delete(f.values, keys[0])
			}
		case int64:
			if result == 1 && strings.Contains(script, "ARGV[3]") && len(args) > 1 {
				f.values[keys[0]] = cloneBytes(args[1])
			}
		}
	}
	return redis.NewCmdResult(f.evalResult, f.evalErr)
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

func bucketForTest[V any](t *testing.T, fake Client, serializer serialization.Serializer[V]) *Bucket[V] {
	t.Helper()
	bucket, err := New(fake, Options[V]{Namespace: "tenant:catalog", HashTag: "tenant", Serializer: serializer})
	if err != nil {
		t.Fatal(err)
	}
	return bucket
}

func TestConstructorRejectsNilDependenciesAndZeroValueIsSafe(t *testing.T) {
	var typedNilClient *fakeClient
	var typedNilSerializer *testSerializer
	for name, options := range map[string]Options[string]{
		"nil serializer":       {Namespace: "catalog", Serializer: nil},
		"typed nil serializer": {Namespace: "catalog", Serializer: typedNilSerializer},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := New[string](newFakeClient(), options)
			if !errors.Is(err, ErrInvalidOptions) {
				t.Fatalf("New() error = %v, want ErrInvalidOptions", err)
			}
		})
	}
	if _, err := New[string](typedNilClient, Options[string]{Namespace: "catalog", Serializer: serialization.StringSerializer{}}); !errors.Is(err, ErrInvalidClient) {
		t.Fatalf("typed nil client error = %v", err)
	}
	var zero Bucket[string]
	if _, _, err := zero.Get(context.Background(), "key"); !errors.Is(err, ErrUninitialized) {
		t.Fatalf("zero Get() error = %v", err)
	}
}

func TestBucketRoundTripAndExactKeyTTL(t *testing.T) {
	fake := newFakeClient()
	bucket := bucketForTest(t, fake, serialization.StringSerializer{})
	logicalKey := " sku:{42}:raw "
	if err := bucket.Set(context.Background(), logicalKey, "value", 1500*time.Microsecond); err != nil {
		t.Fatal(err)
	}
	fake.mu.Lock()
	physicalKey, ttl, payload := fake.lastKey, fake.lastTTL, append([]byte(nil), fake.lastPayload...)
	fake.mu.Unlock()
	if physicalKey != "tenant:catalog:bucket:{tenant}:"+logicalKey {
		t.Fatalf("physical key = %q", physicalKey)
	}
	if ttl != time.Millisecond || !reflect.DeepEqual(payload, []byte("value")) {
		t.Fatalf("write capture key=%q ttl=%s payload=%q", physicalKey, ttl, payload)
	}
	got, hit, err := bucket.Get(context.Background(), logicalKey)
	if err != nil || !hit || got != "value" {
		t.Fatalf("Get() = %q, %v, %v", got, hit, err)
	}
}

func TestBucketConditionalAndLuaOperations(t *testing.T) {
	fake := newFakeClient()
	bucket := bucketForTest(t, fake, serialization.StringSerializer{})
	ctx := context.Background()
	if ok, err := bucket.SetIfAbsent(ctx, "key", "first", 0); err != nil || !ok {
		t.Fatalf("first SetIfAbsent() = %v, %v", ok, err)
	}
	if ok, err := bucket.SetIfAbsent(ctx, "key", "second", 0); err != nil || ok {
		t.Fatalf("second SetIfAbsent() = %v, %v", ok, err)
	}
	fake.mu.Lock()
	fake.evalResult = []interface{}{int64(1), []byte("first")}
	fake.mu.Unlock()
	got, hit, err := bucket.GetAndDelete(ctx, "key")
	if err != nil || !hit || got != "first" {
		t.Fatalf("GetAndDelete() = %q, %v, %v", got, hit, err)
	}
	fake.mu.Lock()
	remaining := append([]byte(nil), fake.values[fake.lastKey]...)
	fake.evalResult = int64(0)
	fake.mu.Unlock()
	if ok, err := bucket.CompareAndSet(ctx, "cas", "missing", "next", time.Second); err != nil || ok {
		t.Fatalf("missing CompareAndSet() = %v, %v", ok, err)
	}
	fake.mu.Lock()
	fake.values["tenant:catalog:bucket:{tenant}:cas"] = []byte("expected")
	fake.evalResult = int64(1)
	fake.mu.Unlock()
	if ok, err := bucket.CompareAndSet(ctx, "cas", "expected", "next", 0); err != nil || !ok {
		t.Fatalf("successful CompareAndSet() = %v, %v", ok, err)
	}
	if err := bucket.Delete(ctx, "cas"); err != nil {
		t.Fatal(err)
	}
	if len(remaining) != 0 {
		t.Fatalf("GetAndDelete fake retained value = %q", remaining)
	}
}

func TestBucketErrorsCancellationAndRedaction(t *testing.T) {
	fake := newFakeClient()
	bucket := bucketForTest(t, fake, serialization.StringSerializer{})
	ctx, cancel := context.WithCancel(context.Background())
	fake.afterSet = cancel
	if err := bucket.Set(ctx, "secret-key", "payload", time.Second); !errors.Is(err, context.Canceled) {
		t.Fatalf("post-dispatch Set() = %v", err)
	}
	fake.mu.Lock()
	fake.setErr = errors.New("raw redis password secret-key")
	fake.afterSet = nil
	fake.mu.Unlock()
	err := bucket.Set(context.Background(), "secret-key", "payload", time.Second)
	if !errors.Is(err, ErrCommitUnknown) || !errors.Is(err, btredis.ErrCommitUnknown) {
		t.Fatalf("Set() error = %v, want commit unknown", err)
	}
	if stringsContains(err.Error(), "raw redis password") || stringsContains(err.Error(), "secret-key") {
		t.Fatalf("Set() leaked sensitive text: %v", err)
	}
	if err := bucket.Set(nil, "key", "value", 0); !errors.Is(err, ErrInvalidContext) { //nolint:staticcheck // nil context is the contract input under test.
		t.Fatalf("Set(nil) = %v", err)
	}
}

func TestBucketRejectsMalformedResultsAndPayload(t *testing.T) {
	fake := newFakeClient()
	bucket := bucketForTest(t, fake, serialization.StringSerializer{})
	fake.mu.Lock()
	fake.evalResult = []interface{}{int64(1)}
	fake.mu.Unlock()
	if _, _, err := bucket.GetAndDelete(context.Background(), "key"); !errors.Is(err, ErrMalformedResult) {
		t.Fatalf("malformed GetAndDelete() = %v", err)
	}
	fake.mu.Lock()
	fake.values["tenant:catalog:bucket:{tenant}:bad"] = []byte("payload")
	fake.getErr = errors.New("redis unavailable")
	fake.mu.Unlock()
	if _, _, err := bucket.Get(context.Background(), "bad"); err == nil {
		t.Fatal("Get() unexpectedly succeeded with provider failure")
	}
}

func TestBucketRejectsNegativeTTLAndCodecFailures(t *testing.T) {
	fake := newFakeClient()
	marshalBucket := bucketForTest(t, fake, &marshalErrorSerializer{})
	if err := marshalBucket.Set(context.Background(), "key", "value", -time.Millisecond); !errors.Is(err, btredis.ErrInvalidTTL) {
		t.Fatalf("negative TTL error = %v", err)
	}
	if err := marshalBucket.Set(context.Background(), "key", "value", 0); !errors.Is(err, ErrSerialization) {
		t.Fatalf("marshal error = %v", err)
	}
	fake.values["tenant:catalog:bucket:{tenant}:key"] = []byte("payload")
	unmarshalBucket := bucketForTest(t, fake, &unmarshalErrorSerializer{})
	if _, _, err := unmarshalBucket.Get(context.Background(), "key"); !errors.Is(err, ErrInvalidPayload) {
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

func stringsContains(value, substring string) bool {
	return len(substring) == 0 || (len(value) >= len(substring) && contains(value, substring))
}

func contains(value, substring string) bool {
	for i := 0; i+len(substring) <= len(value); i++ {
		if value[i:i+len(substring)] == substring {
			return true
		}
	}
	return false
}
