package redisvalue

import (
	"context"
	"encoding/json"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/bluetape4k/bluetape-go/cache"
	"github.com/bluetape4k/bluetape-go/serialization"
	"github.com/redis/go-redis/v9"
)

type localFuncs[V any] struct {
	get    func(context.Context, string) (V, error)
	set    func(context.Context, string, V, time.Duration) error
	delete func(context.Context, string) error
	clear  func(context.Context) error
}

func (l *localFuncs[V]) Get(ctx context.Context, key string) (V, error) {
	if l.get != nil {
		return l.get(ctx, key)
	}
	var zero V
	return zero, cache.ErrCacheMiss
}

func (l *localFuncs[V]) Set(ctx context.Context, key string, value V, ttl time.Duration) error {
	if l.set != nil {
		return l.set(ctx, key, value, ttl)
	}
	return nil
}

func (l *localFuncs[V]) Delete(ctx context.Context, key string) error {
	if l.delete != nil {
		return l.delete(ctx, key)
	}
	return nil
}

func (l *localFuncs[V]) Clear(ctx context.Context) error {
	if l.clear != nil {
		return l.clear(ctx)
	}
	return nil
}

type pointerSerializer struct {
	marshalCalls   atomic.Int64
	unmarshalCalls atomic.Int64
}

func (s *pointerSerializer) Marshal(value *valueTestRecord) ([]byte, error) {
	s.marshalCalls.Add(1)
	return json.Marshal(value)
}

func (s *pointerSerializer) Unmarshal(data []byte) (*valueTestRecord, error) {
	s.unmarshalCalls.Add(1)
	var value valueTestRecord
	if err := json.Unmarshal(data, &value); err != nil {
		return nil, err
	}
	return &value, nil
}

func unitRemotePointerCache(value *valueTestRecord) (*ValueCache[*valueTestRecord], *pointerSerializer, *atomic.Int64) {
	encoded, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}
	var redisCalls atomic.Int64
	client := &fakeCommandClient{
		getRange: func(context.Context, string, int64, int64) *redis.StringCmd {
			redisCalls.Add(1)
			return redis.NewStringResult(string(encoded), nil)
		},
	}
	serializer := &pointerSerializer{}
	remote := unitValueCache[*valueTestRecord](client, serializer, ValueConfig{RemoteTTL: time.Hour, MaxValueBytes: 1024, ClearBatchSize: 10})
	return remote, serializer, &redisCalls
}

func mustTieredCache[V any](t *testing.T, local cache.Cache[string, V], remote *ValueCache[V], config *TieredConfig) *TieredCache[V] {
	t.Helper()
	tiered, err := NewTieredCache(TieredOptions[V]{Local: local, Remote: remote, Config: config})
	if err != nil {
		t.Fatal(err)
	}
	return tiered
}

func TestTieredCacheConstructorValidatesAndCopiesInputs(t *testing.T) {
	remote, _, _ := unitRemotePointerCache(&valueTestRecord{Name: "remote"})
	local := cache.NewMemory[string, *valueTestRecord]()
	config := TieredConfig{LocalTTL: time.Minute, InvalidationWaitTimeout: 2 * time.Second, LocalCleanupTimeout: time.Second}
	tiered := mustTieredCache(t, local, remote, &config)
	config.LocalTTL = 2 * time.Minute
	if tiered.config.LocalTTL != time.Minute {
		t.Fatalf("constructor retained mutable config: %+v", tiered.config)
	}
	if tiered.coordinators == nil || tiered.localState == nil {
		t.Fatal("constructor omitted coordinator or local state")
	}
}

func TestTieredCacheConstructorRejectsInvalidDependencies(t *testing.T) {
	remote, _, _ := unitRemotePointerCache(&valueTestRecord{Name: "remote"})
	local := cache.NewMemory[string, *valueTestRecord]()
	var typedNilLocal *localFuncs[*valueTestRecord]
	invalidRemote := &ValueCache[*valueTestRecord]{}
	invalidConfig := DefaultConfig().Tiered
	invalidConfig.LocalTTL = 0
	tooLong := DefaultConfig().Tiered
	tooLong.LocalTTL = remote.config.RemoteTTL + time.Nanosecond

	tests := []struct {
		name    string
		options TieredOptions[*valueTestRecord]
	}{
		{name: "nil local", options: TieredOptions[*valueTestRecord]{Remote: remote}},
		{name: "typed nil local", options: TieredOptions[*valueTestRecord]{Local: typedNilLocal, Remote: remote}},
		{name: "nil remote", options: TieredOptions[*valueTestRecord]{Local: local}},
		{name: "uninitialized remote", options: TieredOptions[*valueTestRecord]{Local: local, Remote: invalidRemote}},
		{name: "invalid config", options: TieredOptions[*valueTestRecord]{Local: local, Remote: remote, Config: &invalidConfig}},
		{name: "local ttl exceeds remote", options: TieredOptions[*valueTestRecord]{Local: local, Remote: remote, Config: &tooLong}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewTieredCache(tt.options)
			if !hasReason(err, ReasonConfiguration) {
				t.Fatalf("NewTieredCache() = %v", err)
			}
		})
	}
}

func TestTieredCacheHealthyL1SkipsRemoteAndSerializer(t *testing.T) {
	local := cache.NewMemory[string, *valueTestRecord]()
	want := &valueTestRecord{Name: "local"}
	if err := local.Set(context.Background(), "item", want, time.Hour); err != nil {
		t.Fatal(err)
	}
	remote, serializer, redisCalls := unitRemotePointerCache(&valueTestRecord{Name: "remote"})
	tiered := mustTieredCache(t, local, remote, nil)

	var got *valueTestRecord
	var getErr error
	allocs := testing.AllocsPerRun(1000, func() {
		got, getErr = tiered.Get(context.Background(), "item")
	})
	if getErr != nil || got != want {
		t.Fatalf("Get() = %+v/%v", got, getErr)
	}
	if serializer.unmarshalCalls.Load() != 0 || serializer.marshalCalls.Load() != 0 || redisCalls.Load() != 0 {
		t.Fatalf("serializer/redis calls = %d/%d/%d", serializer.marshalCalls.Load(), serializer.unmarshalCalls.Load(), redisCalls.Load())
	}
	if tiered.coordinators.active() != 0 {
		t.Fatalf("coordinators = %d", tiered.coordinators.active())
	}
	if allocs != 0 {
		t.Fatalf("healthy L1 allocations = %f", allocs)
	}
}

func TestTieredCacheL2HitStoresDecodedReference(t *testing.T) {
	local := cache.NewMemory[string, *valueTestRecord]()
	remote, serializer, _ := unitRemotePointerCache(&valueTestRecord{Name: "remote"})
	tiered := mustTieredCache(t, local, remote, nil)

	first, err := tiered.Get(context.Background(), "item")
	if err != nil {
		t.Fatal(err)
	}
	second, err := tiered.Get(context.Background(), "item")
	if err != nil {
		t.Fatal(err)
	}
	if first != second {
		t.Fatal("L1 did not preserve pointer identity")
	}
	if serializer.unmarshalCalls.Load() != 1 || serializer.marshalCalls.Load() != 0 {
		t.Fatalf("serializer calls = %d/%d", serializer.marshalCalls.Load(), serializer.unmarshalCalls.Load())
	}
}

func TestTieredCachePointerIsolationAcrossColdDecorators(t *testing.T) {
	remote, serializer, _ := unitRemotePointerCache(&valueTestRecord{Name: "remote"})
	first := mustTieredCache(t, cache.NewMemory[string, *valueTestRecord](), remote, nil)
	second := mustTieredCache(t, cache.NewMemory[string, *valueTestRecord](), remote, nil)
	firstValue, err := first.Get(context.Background(), "item")
	if err != nil {
		t.Fatal(err)
	}
	secondValue, err := second.Get(context.Background(), "item")
	if err != nil {
		t.Fatal(err)
	}
	if firstValue == secondValue {
		t.Fatal("cold decorators shared deserialized pointer")
	}
	if serializer.unmarshalCalls.Load() != 2 {
		t.Fatalf("unmarshal calls = %d", serializer.unmarshalCalls.Load())
	}
}

func TestTieredCacheL1FailureDoesNotFallThroughToRemote(t *testing.T) {
	localErr := errors.New("local get failed")
	local := &localFuncs[string]{get: func(context.Context, string) (string, error) { return "", localErr }}
	remote := unitValueCache[string](&fakeCommandClient{}, serialization.StringSerializer{}, ValueConfig{RemoteTTL: time.Hour, MaxValueBytes: 32, ClearBatchSize: 10})
	tiered := mustTieredCache(t, local, remote, nil)
	_, err := tiered.Get(context.Background(), "item")
	if !hasReason(err, ReasonLocalFailure) || !errors.Is(err, localErr) {
		t.Fatalf("Get() = %v", err)
	}
}

func TestTieredCacheL2FailureDoesNotBecomeMiss(t *testing.T) {
	remoteErr := errors.New("redis failed")
	client := &fakeCommandClient{getRange: func(context.Context, string, int64, int64) *redis.StringCmd {
		return redis.NewStringResult("", remoteErr)
	}}
	remote := unitValueCache[string](client, serialization.StringSerializer{}, ValueConfig{RemoteTTL: time.Hour, MaxValueBytes: 32, ClearBatchSize: 10})
	tiered := mustTieredCache(t, cache.NewMemory[string, string](), remote, nil)
	_, err := tiered.Get(context.Background(), "item")
	if !hasReason(err, ReasonProviderFailure) || !errors.Is(err, remoteErr) {
		t.Fatalf("Get() = %v", err)
	}
}

func TestTieredCacheL2PopulationFailureDeletesLocal(t *testing.T) {
	setErr := errors.New("local set failed")
	var deleteCalls atomic.Int64
	local := &localFuncs[string]{
		get: func(context.Context, string) (string, error) { return "", cache.ErrCacheMiss },
		set: func(context.Context, string, string, time.Duration) error { return setErr },
		delete: func(context.Context, string) error {
			deleteCalls.Add(1)
			return nil
		},
	}
	client := &fakeCommandClient{getRange: func(context.Context, string, int64, int64) *redis.StringCmd {
		return redis.NewStringResult("remote", nil)
	}}
	remote := unitValueCache[string](client, serialization.StringSerializer{}, ValueConfig{RemoteTTL: time.Hour, MaxValueBytes: 32, ClearBatchSize: 10})
	tiered := mustTieredCache(t, local, remote, nil)
	_, err := tiered.Get(context.Background(), "item")
	if !hasReason(err, ReasonLocalFailure) || !errors.Is(err, setErr) || deleteCalls.Load() != 1 {
		t.Fatalf("Get() = %v, delete calls=%d", err, deleteCalls.Load())
	}
}

type blockingHitLocal struct {
	slowEntered chan struct{}
	slowRelease chan struct{}
	values      map[string]*valueTestRecord
}

func (l *blockingHitLocal) Get(_ context.Context, key string) (*valueTestRecord, error) {
	if key == "slow" {
		close(l.slowEntered)
		<-l.slowRelease
	}
	return l.values[key], nil
}

func (*blockingHitLocal) Set(context.Context, string, *valueTestRecord, time.Duration) error {
	return nil
}
func (*blockingHitLocal) Delete(context.Context, string) error { return nil }
func (*blockingHitLocal) Clear(context.Context) error          { return nil }

func TestTieredCacheDifferentKeyL1HitsDoNotSerialize(t *testing.T) {
	local := &blockingHitLocal{
		slowEntered: make(chan struct{}),
		slowRelease: make(chan struct{}),
		values: map[string]*valueTestRecord{
			"slow": {Name: "slow"},
			"fast": {Name: "fast"},
		},
	}
	remote, serializer, redisCalls := unitRemotePointerCache(&valueTestRecord{Name: "remote"})
	tiered := mustTieredCache(t, local, remote, nil)
	slowDone := make(chan error, 1)
	go func() {
		_, err := tiered.Get(context.Background(), "slow")
		slowDone <- err
	}()
	<-local.slowEntered
	fastDone := make(chan error, 1)
	go func() {
		value, err := tiered.Get(context.Background(), "fast")
		if err == nil && value.Name != "fast" {
			err = errors.New("wrong fast value")
		}
		fastDone <- err
	}()
	select {
	case err := <-fastDone:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("fast L1 hit blocked behind slow key")
	}
	close(local.slowRelease)
	if err := <-slowDone; err != nil {
		t.Fatal(err)
	}
	if serializer.unmarshalCalls.Load() != 0 || redisCalls.Load() != 0 || tiered.coordinators.active() != 0 {
		t.Fatalf("unmarshal/redis/coordinators = %d/%d/%d", serializer.unmarshalCalls.Load(), redisCalls.Load(), tiered.coordinators.active())
	}
}

func TestTieredCacheGetZeroValueReturnsUninitialized(t *testing.T) {
	var tiered TieredCache[string]
	if _, err := tiered.Get(context.Background(), "item"); !hasReason(err, ReasonUninitialized) {
		t.Fatalf("Get() = %v", err)
	}
}

func TestTieredCacheGetReturnsExactMiss(t *testing.T) {
	client := &fakeCommandClient{getRange: func(context.Context, string, int64, int64) *redis.StringCmd {
		return redis.NewStringResult("", redis.Nil)
	}}
	remote := unitValueCache[string](client, serialization.StringSerializer{}, ValueConfig{RemoteTTL: time.Hour, MaxValueBytes: 32, ClearBatchSize: 10})
	tiered := mustTieredCache(t, cache.NewMemory[string, string](), remote, nil)
	_, err := tiered.Get(context.Background(), "item")
	if err != cache.ErrCacheMiss {
		t.Fatalf("Get() = %v, want exact cache.ErrCacheMiss", err)
	}
}

func TestTieredCacheL1HitAndL2HitUseIndependentLocalCaches(t *testing.T) {
	remote, _, _ := unitRemotePointerCache(&valueTestRecord{Name: "remote"})
	local := cache.NewMemory[string, *valueTestRecord]()
	warm := &valueTestRecord{Name: "warm"}
	if err := local.Set(context.Background(), "warm", warm, time.Hour); err != nil {
		t.Fatal(err)
	}
	tiered := mustTieredCache(t, local, remote, nil)
	got, err := tiered.Get(context.Background(), "warm")
	if err != nil || got != warm {
		t.Fatalf("warm Get() = %+v/%v", got, err)
	}
}
