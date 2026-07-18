package redisvalue

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/bluetape4k/bluetape-go/cache"
	btredis "github.com/bluetape4k/bluetape-go/redis"
	"github.com/bluetape4k/bluetape-go/serialization"
	"github.com/google/go-cmp/cmp"
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

type loadTestRemote struct {
	cache    *ValueCache[string]
	getCalls atomic.Int64
	setCalls atomic.Int64
	setTTL   atomic.Int64
}

func newMissTieredCache(t *testing.T) (*TieredCache[string], *loadTestRemote) {
	t.Helper()
	remoteState := &loadTestRemote{}
	client := &fakeCommandClient{
		getRange: func(context.Context, string, int64, int64) *redis.StringCmd {
			remoteState.getCalls.Add(1)
			return redis.NewStringResult("", redis.Nil)
		},
		set: func(_ context.Context, _ string, _ any, ttl time.Duration) *redis.StatusCmd {
			remoteState.setCalls.Add(1)
			remoteState.setTTL.Store(int64(ttl))
			return redis.NewStatusResult("OK", nil)
		},
	}
	remoteState.cache = unitValueCache[string](client, serialization.StringSerializer{}, ValueConfig{RemoteTTL: time.Hour, MaxValueBytes: 1024, ClearBatchSize: 10})
	return mustTieredCache(t, cache.NewMemory[string, string](), remoteState.cache, nil), remoteState
}

func TestGetOrLoadRejectsNilLoaderBeforeCoordinatorCreation(t *testing.T) {
	tiered, remote := newMissTieredCache(t)
	if _, err := tiered.GetOrLoad(context.Background(), "shared", time.Minute, nil); !hasReason(err, ReasonConfiguration) {
		t.Fatalf("GetOrLoad() = %v", err)
	}
	if tiered.coordinators.active() != 0 || remote.getCalls.Load() != 0 || remote.setCalls.Load() != 0 {
		t.Fatalf("active/get/set = %d/%d/%d", tiered.coordinators.active(), remote.getCalls.Load(), remote.setCalls.Load())
	}
}

func TestGetOrLoadSharesOneLeaderResult(t *testing.T) {
	tiered, remote := newMissTieredCache(t)
	entered := make(chan struct{})
	release := make(chan struct{})
	var calls atomic.Int64
	loader := func(context.Context, string) (string, error) {
		if calls.Add(1) == 1 {
			close(entered)
		}
		<-release
		return "loaded", nil
	}
	type result struct {
		value string
		err   error
	}
	results := make(chan result, 16)
	for range 16 {
		go func() {
			value, err := tiered.GetOrLoad(context.Background(), "shared", time.Minute, loader)
			results <- result{value: value, err: err}
		}()
	}
	<-entered
	waitForFlightParticipants(t, tiered, "shared", 16)
	close(release)
	for range 16 {
		got := <-results
		if got.err != nil || got.value != "loaded" {
			t.Fatalf("result = %q/%v", got.value, got.err)
		}
	}
	if calls.Load() != 1 || remote.getCalls.Load() != 1 || remote.setCalls.Load() != 1 || tiered.coordinators.active() != 0 {
		t.Fatalf("loader/get/set/active = %d/%d/%d/%d", calls.Load(), remote.getCalls.Load(), remote.setCalls.Load(), tiered.coordinators.active())
	}
}

func TestGetOrLoadSharesOneLeaderError(t *testing.T) {
	tiered, remote := newMissTieredCache(t)
	loaderErr := errors.New("loader failed")
	entered := make(chan struct{})
	release := make(chan struct{})
	var calls atomic.Int64
	loader := func(context.Context, string) (string, error) {
		if calls.Add(1) == 1 {
			close(entered)
		}
		<-release
		return "", loaderErr
	}
	results := make(chan error, 8)
	for range 8 {
		go func() {
			_, err := tiered.GetOrLoad(context.Background(), "shared", time.Minute, loader)
			results <- err
		}()
	}
	<-entered
	waitForFlightParticipants(t, tiered, "shared", 8)
	close(release)
	for range 8 {
		if err := <-results; !errors.Is(err, loaderErr) {
			t.Fatalf("result error = %v", err)
		}
	}
	if calls.Load() != 1 || remote.setCalls.Load() != 0 || tiered.coordinators.active() != 0 {
		t.Fatalf("loader/set/active = %d/%d/%d", calls.Load(), remote.setCalls.Load(), tiered.coordinators.active())
	}
}

func TestGetOrLoadFirstLeaderOwnsTTLAndLoader(t *testing.T) {
	tiered, remote := newMissTieredCache(t)
	entered := make(chan struct{})
	release := make(chan struct{})
	var firstCalls atomic.Int64
	var followerCalls atomic.Int64
	firstResult := make(chan error, 1)
	go func() {
		_, err := tiered.GetOrLoad(context.Background(), "shared", 7*time.Second, func(context.Context, string) (string, error) {
			firstCalls.Add(1)
			close(entered)
			<-release
			return "leader", nil
		})
		firstResult <- err
	}()
	<-entered
	followerResult := make(chan string, 1)
	go func() {
		value, err := tiered.GetOrLoad(context.Background(), "shared", 33*time.Second, func(context.Context, string) (string, error) {
			followerCalls.Add(1)
			return "follower", nil
		})
		if err != nil {
			followerResult <- err.Error()
			return
		}
		followerResult <- value
	}()
	waitForFlightParticipants(t, tiered, "shared", 2)
	close(release)
	if err := <-firstResult; err != nil {
		t.Fatal(err)
	}
	if got := <-followerResult; got != "leader" {
		t.Fatalf("follower result = %q", got)
	}
	if firstCalls.Load() != 1 || followerCalls.Load() != 0 || time.Duration(remote.setTTL.Load()) != 7*time.Second {
		t.Fatalf("first/follower/ttl = %d/%d/%s", firstCalls.Load(), followerCalls.Load(), time.Duration(remote.setTTL.Load()))
	}
}

func TestGetOrLoadFollowerCancellationDoesNotCancelLeader(t *testing.T) {
	tiered, _ := newMissTieredCache(t)
	entered := make(chan struct{})
	release := make(chan struct{})
	leaderResult := make(chan error, 1)
	go func() {
		_, err := tiered.GetOrLoad(context.Background(), "shared", time.Minute, func(context.Context, string) (string, error) {
			close(entered)
			<-release
			return "loaded", nil
		})
		leaderResult <- err
	}()
	<-entered
	followerCtx, cancelFollower := context.WithCancel(context.Background())
	followerResult := make(chan error, 1)
	go func() {
		_, err := tiered.GetOrLoad(followerCtx, "shared", time.Hour, func(context.Context, string) (string, error) {
			return "wrong", nil
		})
		followerResult <- err
	}()
	waitForFlightParticipants(t, tiered, "shared", 2)
	cancelFollower()
	if err := <-followerResult; !errors.Is(err, context.Canceled) {
		t.Fatalf("follower error = %v", err)
	}
	close(release)
	if err := <-leaderResult; err != nil {
		t.Fatal(err)
	}
	if tiered.coordinators.active() != 0 {
		t.Fatalf("active coordinators = %d", tiered.coordinators.active())
	}
}

func TestGetOrLoadLeaderCancellationPublishesToFollowers(t *testing.T) {
	tiered, _ := newMissTieredCache(t)
	blocker := tiered.coordinators.acquire("shared")
	if err := blocker.acquireToken(context.Background()); err != nil {
		t.Fatal(err)
	}
	leaderCtx, cancelLeader := context.WithCancel(context.Background())
	leaderResult := make(chan error, 1)
	go func() {
		_, err := tiered.GetOrLoad(leaderCtx, "shared", time.Minute, func(context.Context, string) (string, error) {
			return "wrong", nil
		})
		leaderResult <- err
	}()
	waitForFlightParticipants(t, tiered, "shared", 1)
	followerResult := make(chan error, 1)
	go func() {
		_, err := tiered.GetOrLoad(context.Background(), "shared", time.Minute, func(context.Context, string) (string, error) {
			return "wrong follower", nil
		})
		followerResult <- err
	}()
	waitForFlightParticipants(t, tiered, "shared", 2)
	cancelLeader()
	if err := <-leaderResult; !errors.Is(err, context.Canceled) {
		t.Fatalf("leader error = %v", err)
	}
	if err := <-followerResult; !errors.Is(err, context.Canceled) {
		t.Fatalf("follower error = %v", err)
	}
	blocker.releaseToken()
	tiered.coordinators.release("shared", blocker)
	if tiered.coordinators.active() != 0 {
		t.Fatalf("active coordinators = %d", tiered.coordinators.active())
	}
}

func TestGetOrLoadDoesNotWriteAfterLoaderCancellation(t *testing.T) {
	tiered, remote := newMissTieredCache(t)
	ctx, cancel := context.WithCancel(context.Background())
	_, err := tiered.GetOrLoad(ctx, "shared", time.Minute, func(context.Context, string) (string, error) {
		cancel()
		return "loaded", nil
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("GetOrLoad() = %v", err)
	}
	if remote.setCalls.Load() != 0 {
		t.Fatalf("remote set calls = %d", remote.setCalls.Load())
	}
}

func TestGetOrLoadHealthyRepairWaitRetainsFlightLeadership(t *testing.T) {
	remoteEntered := make(chan struct{})
	remoteRelease := make(chan struct{})
	var getCalls atomic.Int64
	client := &fakeCommandClient{
		getRange: func(context.Context, string, int64, int64) *redis.StringCmd {
			if getCalls.Add(1) == 1 {
				close(remoteEntered)
				<-remoteRelease
			}
			return redis.NewStringResult("", redis.Nil)
		},
		set: func(context.Context, string, any, time.Duration) *redis.StatusCmd {
			return redis.NewStatusResult("OK", nil)
		},
	}
	remote := unitValueCache[string](client, serialization.StringSerializer{}, ValueConfig{RemoteTTL: time.Hour, MaxValueBytes: 32, ClearBatchSize: 10})
	tiered := mustTieredCache(t, cache.NewMemory[string, string](), remote, nil)
	var loaderCalls atomic.Int64
	loader := func(context.Context, string) (string, error) {
		loaderCalls.Add(1)
		return "loaded", nil
	}
	leaderResult := make(chan error, 1)
	go func() {
		_, err := tiered.GetOrLoad(context.Background(), "shared", time.Minute, loader)
		leaderResult <- err
	}()
	<-remoteEntered
	followerResult := make(chan error, 1)
	go func() {
		_, err := tiered.GetOrLoad(context.Background(), "shared", time.Hour, func(context.Context, string) (string, error) {
			return "wrong", nil
		})
		followerResult <- err
	}()
	waitForFlightParticipants(t, tiered, "shared", 2)
	repair, err := tiered.localState.beginRepair(context.Background(), repairExplicit)
	if err != nil {
		t.Fatal(err)
	}
	close(remoteRelease)
	time.Sleep(20 * time.Millisecond)
	if loaderCalls.Load() != 0 {
		t.Fatalf("loader ran during repair: %d", loaderCalls.Load())
	}
	if participants := currentFlightParticipants(tiered, "shared"); participants != 2 {
		t.Fatalf("flight participants during repair = %d", participants)
	}
	if !tiered.localState.finishRepair(repair, nil) {
		t.Fatal("repair did not finish")
	}
	if err := <-leaderResult; err != nil {
		t.Fatal(err)
	}
	if err := <-followerResult; err != nil {
		t.Fatal(err)
	}
	if loaderCalls.Load() != 1 || getCalls.Load() != 2 || tiered.coordinators.active() != 0 {
		t.Fatalf("loader/get/active = %d/%d/%d", loaderCalls.Load(), getCalls.Load(), tiered.coordinators.active())
	}
}

func TestGetOrLoadDifferentKeysProceedConcurrently(t *testing.T) {
	tiered, _ := newMissTieredCache(t)
	entered := make(chan string, 2)
	release := make(chan struct{})
	results := make(chan error, 2)
	for _, key := range []string{"a", "b"} {
		go func(key string) {
			value, err := tiered.GetOrLoad(context.Background(), key, time.Minute, func(context.Context, string) (string, error) {
				entered <- key
				<-release
				return key, nil
			})
			if err == nil && value != key {
				err = errors.New("wrong loaded value")
			}
			results <- err
		}(key)
	}
	for range 2 {
		select {
		case <-entered:
		case <-time.After(time.Second):
			t.Fatal("different-key loaders did not enter concurrently")
		}
	}
	close(release)
	for range 2 {
		if err := <-results; err != nil {
			t.Fatal(err)
		}
	}
	if tiered.coordinators.active() != 0 {
		t.Fatalf("active coordinators = %d", tiered.coordinators.active())
	}
}

func TestGetOrLoadDefaultUsesRemoteDefaultTTL(t *testing.T) {
	tiered, remote := newMissTieredCache(t)
	value, err := tiered.GetOrLoadDefault(context.Background(), "shared", func(context.Context, string) (string, error) {
		return "loaded", nil
	})
	if err != nil || value != "loaded" {
		t.Fatalf("GetOrLoadDefault() = %q/%v", value, err)
	}
	if time.Duration(remote.setTTL.Load()) != time.Hour {
		t.Fatalf("remote ttl = %s", time.Duration(remote.setTTL.Load()))
	}
}

func waitForFlightParticipants[V any](t *testing.T, tiered *TieredCache[V], key string, want int64) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for {
		participants := currentFlightParticipants(tiered, key)
		if participants >= want {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("flight participants = %d, want %d", participants, want)
		}
		time.Sleep(time.Millisecond)
	}
}

func currentFlightParticipants[V any](tiered *TieredCache[V], key string) int64 {
	tiered.coordinators.mu.Lock()
	defer tiered.coordinators.mu.Unlock()
	coordinator := tiered.coordinators.items[key]
	if coordinator == nil {
		return 0
	}
	coordinator.mu.Lock()
	defer coordinator.mu.Unlock()
	if coordinator.flight == nil {
		return 0
	}
	return coordinator.flight.participants.Load()
}

var _ cache.LoadingCache[string, valueTestRecord] = (*TieredCache[valueTestRecord])(nil)

type faultLocal[V any] struct {
	mu          sync.Mutex
	values      map[string]V
	getErr      error
	setErr      error
	deleteErr   error
	clearErr    error
	deleteBlock <-chan struct{}
	setValues   []V
	deleteCalls int
	clearCalls  int
	events      *[]string
}

func (l *faultLocal[V]) Get(ctx context.Context, key string) (V, error) {
	if err := ctx.Err(); err != nil {
		var zero V
		return zero, err
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.getErr != nil {
		var zero V
		return zero, l.getErr
	}
	value, ok := l.values[key]
	if !ok {
		var zero V
		return zero, cache.ErrCacheMiss
	}
	return value, nil
}

func (l *faultLocal[V]) Set(ctx context.Context, key string, value V, _ time.Duration) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.values == nil {
		l.values = make(map[string]V)
	}
	l.values[key] = value
	l.setValues = append(l.setValues, value)
	if l.events != nil {
		*l.events = append(*l.events, "local-set")
	}
	return l.setErr
}

func (l *faultLocal[V]) Delete(ctx context.Context, key string) error {
	if l.deleteBlock != nil {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-l.deleteBlock:
		}
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	l.deleteCalls++
	delete(l.values, key)
	if l.events != nil {
		*l.events = append(*l.events, "local-delete")
	}
	return l.deleteErr
}

func (l *faultLocal[V]) Clear(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	l.clearCalls++
	clear(l.values)
	if l.events != nil {
		*l.events = append(*l.events, "local-clear")
	}
	return l.clearErr
}

func newMutationTiered[V any](t *testing.T, local cache.Cache[string, V], client commandClient, serializer serialization.Serializer[V]) *TieredCache[V] {
	t.Helper()
	remote := unitValueCache[V](client, serializer, ValueConfig{RemoteTTL: time.Hour, MaxValueBytes: 1024, ClearBatchSize: 2})
	return mustTieredCache(t, local, remote, nil)
}

func TestTieredCacheSetPreservesReference(t *testing.T) {
	var events []string
	local := &faultLocal[*valueTestRecord]{events: &events}
	client := &fakeCommandClient{set: func(context.Context, string, any, time.Duration) *redis.StatusCmd {
		events = append(events, "redis-set")
		return redis.NewStatusResult("OK", nil)
	}}
	tiered := newMutationTiered(t, local, client, serialization.NewJSONSerializer[*valueTestRecord]())
	want := &valueTestRecord{Name: "original"}
	if err := tiered.Set(context.Background(), "item", want, time.Minute); err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff([]string{"redis-set", "local-set"}, events); diff != "" {
		t.Fatalf("events (-want +got):\n%s", diff)
	}
	if len(local.setValues) != 1 || local.setValues[0] != want {
		t.Fatal("L1 did not retain original reference")
	}
	got, err := tiered.Get(context.Background(), "item")
	if err != nil || got != want {
		t.Fatalf("Get() = %+v/%v", got, err)
	}
}

func TestTieredCacheSetSerializesSameKeyMutations(t *testing.T) {
	firstEntered := make(chan struct{})
	firstRelease := make(chan struct{})
	var calls atomic.Int64
	var orderMu sync.Mutex
	var order []string
	client := &fakeCommandClient{set: func(_ context.Context, _ string, value any, _ time.Duration) *redis.StatusCmd {
		encoded := string(value.([]byte))
		if calls.Add(1) == 1 {
			close(firstEntered)
			<-firstRelease
		}
		orderMu.Lock()
		order = append(order, encoded)
		orderMu.Unlock()
		return redis.NewStatusResult("OK", nil)
	}}
	tiered := newMutationTiered(t, cache.NewMemory[string, string](), client, serialization.StringSerializer{})
	firstDone := make(chan error, 1)
	go func() { firstDone <- tiered.Set(context.Background(), "item", "first", time.Minute) }()
	<-firstEntered
	secondDone := make(chan error, 1)
	go func() { secondDone <- tiered.Set(context.Background(), "item", "second", time.Minute) }()
	time.Sleep(20 * time.Millisecond)
	if calls.Load() != 1 {
		t.Fatalf("second mutation entered Redis early: %d", calls.Load())
	}
	close(firstRelease)
	if err := <-firstDone; err != nil {
		t.Fatal(err)
	}
	if err := <-secondDone; err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff([]string{"first", "second"}, order); diff != "" {
		t.Fatalf("Redis order (-want +got):\n%s", diff)
	}
}

func TestTieredCacheSetCommitUnknownRunsMandatoryCleanupAndRedactsJoinedErrors(t *testing.T) {
	providerErr := errors.New("redis-secret 127.0.0.1 raw:key payload")
	deleteErr := errors.New("local-secret 10.0.0.1 raw:key")
	local := &faultLocal[string]{values: map[string]string{"item": "stale"}, deleteErr: deleteErr}
	client := &fakeCommandClient{set: func(context.Context, string, any, time.Duration) *redis.StatusCmd {
		return redis.NewStatusResult("", providerErr)
	}}
	tiered := newMutationTiered(t, local, client, serialization.StringSerializer{})
	err := tiered.Set(context.Background(), "raw:key", "payload", time.Minute)
	if !hasReason(err, ReasonLocalBlocked) || !errors.Is(err, providerErr) || !errors.Is(err, deleteErr) || !errors.Is(err, btredis.ErrCommitUnknown) {
		t.Fatalf("Set() = %v", err)
	}
	for _, secret := range []string{"redis-secret", "local-secret", "127.0.0.1", "10.0.0.1", "raw:key", "payload"} {
		if strings.Contains(err.Error(), secret) {
			t.Fatalf("Set Error() leaked %q: %q", secret, err.Error())
		}
	}
	if local.deleteCalls != 1 || tiered.localState.phaseValue() != phaseBlocked {
		t.Fatalf("delete calls/phase = %d/%v", local.deleteCalls, tiered.localState.phaseValue())
	}
}

func TestTieredCacheSetKnownSuccessThenCancellationInvalidatesLocal(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	local := &faultLocal[string]{values: map[string]string{"item": "stale"}}
	client := &fakeCommandClient{set: func(context.Context, string, any, time.Duration) *redis.StatusCmd {
		cancel()
		return redis.NewStatusResult("OK", nil)
	}}
	tiered := newMutationTiered(t, local, client, serialization.StringSerializer{})
	err := tiered.Set(ctx, "item", "new", time.Minute)
	if !errors.Is(err, context.Canceled) || local.deleteCalls != 1 || len(local.setValues) != 0 {
		t.Fatalf("Set() = %v, deletes/sets=%d/%d", err, local.deleteCalls, len(local.setValues))
	}
}

func TestTieredCacheDeleteWritesRedisBeforeLocalCleanup(t *testing.T) {
	var events []string
	local := &faultLocal[string]{values: map[string]string{"item": "stale"}, events: &events}
	client := &fakeCommandClient{del: func(context.Context, ...string) *redis.IntCmd {
		events = append(events, "redis-delete")
		return redis.NewIntResult(1, nil)
	}}
	tiered := newMutationTiered(t, local, client, serialization.StringSerializer{})
	if err := tiered.Delete(context.Background(), "item"); err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff([]string{"redis-delete", "local-delete"}, events); diff != "" {
		t.Fatalf("events (-want +got):\n%s", diff)
	}
}

func TestInvalidateLocalDoesNotCallRedis(t *testing.T) {
	local := &faultLocal[string]{values: map[string]string{"item": "stale"}}
	tiered := newMutationTiered(t, local, &fakeCommandClient{}, serialization.StringSerializer{})
	if err := tiered.InvalidateLocal(context.Background(), "item"); err != nil {
		t.Fatal(err)
	}
	if local.deleteCalls != 1 {
		t.Fatalf("delete calls = %d", local.deleteCalls)
	}
}

func TestInvalidateLocalUsesShorterCleanupDeadlineAndBlocks(t *testing.T) {
	blockedDelete := make(chan struct{})
	local := &faultLocal[string]{deleteBlock: blockedDelete}
	config := TieredConfig{LocalTTL: time.Minute, InvalidationWaitTimeout: time.Second, LocalCleanupTimeout: 20 * time.Millisecond}
	remote := unitValueCache[string](&fakeCommandClient{}, serialization.StringSerializer{}, ValueConfig{RemoteTTL: time.Hour, MaxValueBytes: 32, ClearBatchSize: 10})
	tiered := mustTieredCache(t, local, remote, &config)
	started := time.Now()
	err := tiered.InvalidateLocal(context.Background(), "item")
	if !hasReason(err, ReasonLocalBlocked) || time.Since(started) > 300*time.Millisecond {
		t.Fatalf("InvalidateLocal() = %v after %s", err, time.Since(started))
	}
	if tiered.localState.phaseValue() != phaseBlocked {
		t.Fatalf("phase = %v", tiered.localState.phaseValue())
	}
}

func TestFailedMandatoryCleanupBlocksUntilClearLocal(t *testing.T) {
	local := &faultLocal[string]{deleteErr: errors.New("delete failed")}
	providerErr := errors.New("provider failed")
	client := &fakeCommandClient{set: func(context.Context, string, any, time.Duration) *redis.StatusCmd {
		return redis.NewStatusResult("", providerErr)
	}}
	tiered := newMutationTiered(t, local, client, serialization.StringSerializer{})
	err := tiered.Set(context.Background(), "item", "value", time.Minute)
	if !hasReason(err, ReasonLocalBlocked) {
		t.Fatalf("set error = %v", err)
	}
	if _, err := tiered.Get(context.Background(), "item"); !hasReason(err, ReasonLocalBlocked) {
		t.Fatalf("blocked get = %v", err)
	}
	local.deleteErr = nil
	local.clearErr = nil
	if err := tiered.ClearLocal(context.Background()); err != nil {
		t.Fatal(err)
	}
	if tiered.localState.phaseValue() != phaseHealthy {
		t.Fatalf("phase = %v", tiered.localState.phaseValue())
	}
}

func TestClearLocalFailureRemainsBlocked(t *testing.T) {
	localErr := errors.New("clear failed")
	local := &faultLocal[string]{clearErr: localErr}
	remote := unitValueCache[string](&fakeCommandClient{}, serialization.StringSerializer{}, ValueConfig{RemoteTTL: time.Hour, MaxValueBytes: 32, ClearBatchSize: 10})
	tiered := mustTieredCache(t, local, remote, nil)
	err := tiered.ClearLocal(context.Background())
	if !hasReason(err, ReasonLocalBlocked) || !errors.Is(err, localErr) || tiered.localState.phaseValue() != phaseBlocked {
		t.Fatalf("ClearLocal() = %v, phase=%v", err, tiered.localState.phaseValue())
	}
}

func TestTieredCacheClearRunsRemoteThenMandatoryLocalClear(t *testing.T) {
	var events []string
	local := &faultLocal[string]{values: map[string]string{"item": "stale"}, events: &events}
	client := &fakeCommandClient{scan: func(context.Context, uint64, string, int64) *redis.ScanCmd {
		events = append(events, "redis-scan")
		return redis.NewScanCmdResult(nil, 0, nil)
	}}
	tiered := newMutationTiered(t, local, client, serialization.StringSerializer{})
	if err := tiered.Clear(context.Background()); err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff([]string{"redis-scan", "local-clear"}, events); diff != "" {
		t.Fatalf("events (-want +got):\n%s", diff)
	}
}

func TestTieredCacheClearWhileBlockedRequiresExplicitClearLocal(t *testing.T) {
	local := &faultLocal[string]{}
	client := &fakeCommandClient{scan: func(context.Context, uint64, string, int64) *redis.ScanCmd {
		return redis.NewScanCmdResult(nil, 0, nil)
	}}
	tiered := newMutationTiered(t, local, client, serialization.StringSerializer{})
	tiered.localState.block()
	if err := tiered.Clear(context.Background()); !hasReason(err, ReasonLocalBlocked) {
		t.Fatalf("Clear() = %v", err)
	}
	if tiered.localState.phaseValue() != phaseBlocked {
		t.Fatalf("phase = %v", tiered.localState.phaseValue())
	}
	if err := tiered.ClearLocal(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestConcurrentClearLocalDuringRemoteClearWins(t *testing.T) {
	scanEntered := make(chan struct{})
	releaseScan := make(chan struct{})
	client := &fakeCommandClient{scan: func(context.Context, uint64, string, int64) *redis.ScanCmd {
		close(scanEntered)
		<-releaseScan
		return redis.NewScanCmdResult(nil, 0, nil)
	}}
	outerCleanupErr := errors.New("older clear must not run local cleanup")
	var clearCalls atomic.Int64
	local := &localFuncs[string]{clear: func(context.Context) error {
		if clearCalls.Add(1) == 1 {
			return nil
		}
		return outerCleanupErr
	}}
	tiered := newMutationTiered(t, local, client, serialization.StringSerializer{})
	outerDone := make(chan error, 1)
	go func() { outerDone <- tiered.Clear(context.Background()) }()
	<-scanEntered
	if err := tiered.ClearLocal(context.Background()); err != nil {
		t.Fatal(err)
	}
	close(releaseScan)
	if err := <-outerDone; err != nil {
		t.Fatalf("older Clear() = %v", err)
	}
	if clearCalls.Load() != 1 || tiered.localState.phaseValue() != phaseHealthy {
		t.Fatalf("clear calls/phase = %d/%v", clearCalls.Load(), tiered.localState.phaseValue())
	}
}

func TestTieredCacheClearDoesNotClearPeerDecoratorL1(t *testing.T) {
	client := &fakeCommandClient{scan: func(context.Context, uint64, string, int64) *redis.ScanCmd {
		return redis.NewScanCmdResult(nil, 0, nil)
	}}
	remote := unitValueCache[*valueTestRecord](client, serialization.NewJSONSerializer[*valueTestRecord](), ValueConfig{RemoteTTL: time.Hour, MaxValueBytes: 128, ClearBatchSize: 10})
	firstLocal := cache.NewMemory[string, *valueTestRecord]()
	secondLocal := cache.NewMemory[string, *valueTestRecord]()
	firstValue := &valueTestRecord{Name: "first"}
	secondValue := &valueTestRecord{Name: "second"}
	if err := firstLocal.Set(context.Background(), "item", firstValue, time.Hour); err != nil {
		t.Fatal(err)
	}
	if err := secondLocal.Set(context.Background(), "item", secondValue, time.Hour); err != nil {
		t.Fatal(err)
	}
	first := mustTieredCache(t, firstLocal, remote, nil)
	second := mustTieredCache(t, secondLocal, remote, nil)
	if err := first.Clear(context.Background()); err != nil {
		t.Fatal(err)
	}
	got, err := second.Get(context.Background(), "item")
	if err != nil || got != secondValue {
		t.Fatalf("peer Get() = %+v/%v", got, err)
	}
	if first.localState.phaseValue() != phaseHealthy || second.localState.phaseValue() != phaseHealthy {
		t.Fatalf("phases = %v/%v", first.localState.phaseValue(), second.localState.phaseValue())
	}
}

func TestTieredCacheZeroValueMutationMethodsReturnUninitialized(t *testing.T) {
	var tiered TieredCache[string]
	ctx := context.Background()
	tests := map[string]error{
		"set":              tiered.Set(ctx, "item", "value", time.Minute),
		"set-default":      tiered.SetDefault(ctx, "item", "value"),
		"delete":           tiered.Delete(ctx, "item"),
		"clear":            tiered.Clear(ctx),
		"invalidate-local": tiered.InvalidateLocal(ctx, "item"),
		"clear-local":      tiered.ClearLocal(ctx),
	}
	for name, err := range tests {
		if !hasReason(err, ReasonUninitialized) {
			t.Fatalf("%s = %v", name, err)
		}
	}
}
