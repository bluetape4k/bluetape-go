package rediscoord

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/bluetape4k/bluetape-go/cache"
	"github.com/bluetape4k/bluetape-go/cache/redisnear"
	btredis "github.com/bluetape4k/bluetape-go/redis"
	redistestcontainer "github.com/bluetape4k/bluetape-go/testcontainers/redis"
	bttesting "github.com/bluetape4k/bluetape-go/testing"
	concurrencytest "github.com/bluetape4k/bluetape-go/testing/concurrency"
	"github.com/redis/go-redis/v9"
)

func TestReconcileUnlockRetriesCommitUnknownOnSameLease(t *testing.T) {
	lease := &scriptedUnlocker{results: []unlockResult{
		{err: errors.Join(errors.New("lost response"), btredis.ErrCommitUnknown)},
		{released: false},
	}}
	coord := &StampedeCache[string]{}

	if err := coord.reconcileUnlock(lease); err != nil {
		t.Fatalf("reconcile unlock: %v", err)
	}
	if lease.calls != 2 {
		t.Fatalf("unlock calls = %d, want 2", lease.calls)
	}
}

type unlockResult struct {
	released bool
	err      error
}

type scriptedUnlocker struct {
	results []unlockResult
	calls   int
}

func (s *scriptedUnlocker) Unlock(context.Context) (bool, error) {
	result := s.results[s.calls]
	s.calls++
	return result.released, result.err
}

func TestNewStampedeCacheRejectsInvalidOptions(t *testing.T) {
	client := redis.NewClient(&redis.Options{Addr: "127.0.0.1:0"})
	t.Cleanup(func() {
		_ = client.Close()
	})
	local := cache.NewMemory[string, string]()

	tests := []struct {
		name string
		opts Options[string]
	}{
		{name: "missing client", opts: Options[string]{Cache: local, Codec: JSONCodec[string]{}}},
		{name: "missing cache", opts: Options[string]{Client: client, Codec: JSONCodec[string]{}}},
		{name: "missing codec", opts: Options[string]{Client: client, Cache: local}},
		{name: "blank namespace", opts: Options[string]{Client: client, Cache: local, Codec: JSONCodec[string]{}, Namespace: "  "}},
		{name: "negative lock ttl", opts: Options[string]{Client: client, Cache: local, Codec: JSONCodec[string]{}, LockTTL: -time.Second}},
		{name: "negative result ttl", opts: Options[string]{Client: client, Cache: local, Codec: JSONCodec[string]{}, ResultTTL: -time.Second}},
		{name: "negative poll interval", opts: Options[string]{Client: client, Cache: local, Codec: JSONCodec[string]{}, PollInterval: -time.Second}},
		{name: "negative max result bytes", opts: Options[string]{Client: client, Cache: local, Codec: JSONCodec[string]{}, MaxResultBytes: -1}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := NewStampedeCache[string](tt.opts); err == nil {
				t.Fatal("invalid options should fail")
			}
		})
	}
}

func TestNewStampedeCacheAppliesDefaults(t *testing.T) {
	client := redis.NewClient(&redis.Options{Addr: "127.0.0.1:0"})
	t.Cleanup(func() {
		_ = client.Close()
	})

	coord, err := NewStampedeCache[string](Options[string]{
		Client: client,
		Cache:  cache.NewMemory[string, string](),
		Codec:  JSONCodec[string]{},
	})
	if err != nil {
		t.Fatalf("new stampede cache: %v", err)
	}
	if coord.cfg.namespace != defaultNamespace {
		t.Fatalf("namespace = %q, want %q", coord.cfg.namespace, defaultNamespace)
	}
	if coord.cfg.lockTTL != defaultLockTTL {
		t.Fatalf("lock ttl = %s, want %s", coord.cfg.lockTTL, defaultLockTTL)
	}
	if coord.cfg.resultTTL != defaultResultTTL {
		t.Fatalf("result ttl = %s, want %s", coord.cfg.resultTTL, defaultResultTTL)
	}
	if coord.cfg.pollInterval != defaultPollInterval {
		t.Fatalf("poll interval = %s, want %s", coord.cfg.pollInterval, defaultPollInterval)
	}
	if coord.cfg.maxResultBytes != 0 {
		t.Fatalf("max result bytes = %d, want unlimited", coord.cfg.maxResultBytes)
	}
}

func TestStampedeCacheMaxResultBytesBoundsWriteAndRead(t *testing.T) {
	ctx := context.Background()
	client := redisClient(ctx, t)
	coord, err := NewStampedeCache[string](Options[string]{
		Client: client, Cache: cache.NewMemory[string, string](), Namespace: "result-limit",
		Codec: JSONCodec[string]{}, MaxResultBytes: 32,
	})
	if err != nil {
		t.Fatal(err)
	}

	err = coord.storeResult(ctx, "write-key", "owner-token", []byte("payload-too-large-for-envelope"))
	if !errors.Is(err, ErrResultTooLarge) {
		t.Fatalf("store error = %v", err)
	}
	if exists, err := client.Exists(ctx, coord.resultKey("write-key")).Result(); err != nil || exists != 0 {
		t.Fatalf("oversized result published: exists=%d err=%v", exists, err)
	}

	readKey := coord.resultKey("read-key")
	if err := client.Set(ctx, readKey, []byte(strings.Repeat("x", 33)), time.Minute).Err(); err != nil {
		t.Fatal(err)
	}
	_, _, err = coord.readOwnerResult(ctx, "read-key", time.Minute, "owner-token")
	if !errors.Is(err, ErrResultTooLarge) {
		t.Fatalf("read error = %v", err)
	}
}

func TestJSONCodecRoundTrips(t *testing.T) {
	type payload struct {
		Name  string `json:"name"`
		Count int    `json:"count"`
	}

	codec := JSONCodec[payload]{}
	encoded, err := codec.Marshal(payload{Name: "cache", Count: 3})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	decoded, err := codec.Unmarshal(encoded)
	if err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if decoded.Name != "cache" || decoded.Count != 3 {
		t.Fatalf("decoded mismatch: %+v", decoded)
	}
}

func TestResultEnvelopeRequiresMatchingToken(t *testing.T) {
	encoded, err := encodeResult("owner-a", []byte("value"))
	if err != nil {
		t.Fatalf("encode result: %v", err)
	}
	payload, ok, err := decodeResult(encoded, "owner-a")
	if err != nil {
		t.Fatalf("decode matching result: %v", err)
	}
	if !ok || string(payload) != "value" {
		t.Fatalf("matching result not decoded: ok=%t payload=%q", ok, payload)
	}
	if payload, ok, err := decodeResult(encoded, "owner-b"); err != nil || ok || payload != nil {
		t.Fatalf("wrong token should be ignored, ok=%t payload=%q err=%v", ok, payload, err)
	}
}

func TestEncodedResultSizeMatchesJSONEnvelope(t *testing.T) {
	for _, tc := range []struct {
		token   string
		payload []byte
	}{
		{token: "owner", payload: []byte("value")},
		{token: "owner<>&\u2028", payload: []byte{}},
	} {
		encoded, err := encodeResult(tc.token, tc.payload)
		if err != nil {
			t.Fatal(err)
		}
		size, err := encodedResultSize(tc.token, tc.payload)
		if err != nil {
			t.Fatal(err)
		}
		if size != len(encoded) {
			t.Fatalf("size = %d, encoded = %d", size, len(encoded))
		}
	}
}

func TestStampedeCacheRejectsInvalidGetOrLoadInput(t *testing.T) {
	ctx := context.Background()
	client := redisClient(ctx, t)
	coord := newMemoryCoordinator(t, client, "invalid-input")

	if _, err := coord.GetOrLoad(ctx, "item", 0, nil); err == nil {
		t.Fatal("nil loader should fail")
	}
	if _, err := coord.GetOrLoad(ctx, "item", -time.Second, func(context.Context, string) (string, error) {
		return "value", nil
	}); err == nil {
		t.Fatal("negative ttl should fail")
	}
}

func TestStampedeCacheCollapsesNearCacheLoadsAfterInvalidation(t *testing.T) {
	ctx := context.Background()
	clientA, clientB := redisClients(ctx, t)
	const namespace = "coord-collapse-after-invalidation"

	nearA := newNearCache(ctx, t, clientA, namespace, "origin-a")
	nearB := newNearCache(ctx, t, clientB, namespace, "origin-b")
	coordA := newNearCoordinator(t, clientA, nearA, namespace)
	coordB := newNearCoordinator(t, clientB, nearB, namespace)

	primeNearCaches(ctx, t, nearA, nearB, "item")
	if err := nearA.Delete(ctx, "item"); err != nil {
		t.Fatalf("delete cache a: %v", err)
	}
	assertEventuallyMiss(t, nearB, "item")

	var loads int32
	releaseLoader := make(chan struct{})
	loader := func(ctx context.Context, _ string) (string, error) {
		atomic.AddInt32(&loads, 1)
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-releaseLoader:
			return "fresh", nil
		}
	}

	resultA := make(chan loadResult[string], 1)
	resultB := make(chan loadResult[string], 1)
	go func() {
		value, err := coordA.GetOrLoad(ctx, "item", time.Second, loader)
		resultA <- loadResult[string]{value: value, err: err}
	}()
	go func() {
		value, err := coordB.GetOrLoad(ctx, "item", time.Second, loader)
		resultB <- loadResult[string]{value: value, err: err}
	}()

	bttesting.Eventually(t, 5*time.Second, func() bool {
		return atomic.LoadInt32(&loads) == 1
	})
	time.Sleep(50 * time.Millisecond)
	close(releaseLoader)

	assertLoadResult(t, <-resultA, "fresh")
	assertLoadResult(t, <-resultB, "fresh")
	if got := atomic.LoadInt32(&loads); got != 1 {
		t.Fatalf("loader should run once across near caches, got %d", got)
	}
	assertEventuallyValue(t, nearA, "item", "fresh")
	assertEventuallyValue(t, nearB, "item", "fresh")
}

func TestStampedeCacheSameKeyStressUsesOneLoader(t *testing.T) {
	ctx := context.Background()
	clientA, clientB := redisClients(ctx, t)
	const namespace = "coord-stress"

	nearA := newNearCache(ctx, t, clientA, namespace, "origin-a")
	nearB := newNearCache(ctx, t, clientB, namespace, "origin-b")
	coordA := newNearCoordinator(t, clientA, nearA, namespace)
	coordB := newNearCoordinator(t, clientB, nearB, namespace)
	var loads int32
	var sequence int32

	task := func(ctx context.Context) error {
		coord := coordA
		if atomic.AddInt32(&sequence, 1)%2 == 0 {
			coord = coordB
		}
		value, err := coord.GetOrLoad(ctx, "stress-key", time.Second, func(_ context.Context, _ string) (string, error) {
			atomic.AddInt32(&loads, 1)
			time.Sleep(50 * time.Millisecond)
			return "shared", nil
		})
		if err != nil {
			return err
		}
		if value != "shared" {
			return fmt.Errorf("value = %q, want shared", value)
		}
		return nil
	}

	tester := concurrencytest.NewGoroutineStressTester(concurrencytest.Options{
		Workers:       16,
		RoundsPerTask: 16,
		Timeout:       10 * time.Second,
	})
	report := tester.RunT(t, task)
	if report.Completed != 16 {
		t.Fatalf("stress should complete every round, got %+v", report)
	}
	if got := atomic.LoadInt32(&loads); got != 1 {
		t.Fatalf("stress should share one loader result, got %d loads", got)
	}
}

func TestStampedeCacheAsyncWaiterCancellation(t *testing.T) {
	ctx := context.Background()
	client := redisClient(ctx, t)
	coord := newMemoryCoordinator(t, client, "waiter-cancel")

	if err := client.Set(ctx, coord.lockKey("item"), "manual-owner", time.Second).Err(); err != nil {
		t.Fatalf("seed manual owner: %v", err)
	}
	t.Cleanup(func() {
		_ = client.Del(context.Background(), coord.lockKey("item")).Err()
	})

	task := func(context.Context) error {
		waitCtx, cancel := context.WithTimeout(context.Background(), 25*time.Millisecond)
		defer cancel()
		_, err := coord.GetOrLoad(waitCtx, "item", time.Second, func(context.Context, string) (string, error) {
			return "unexpected", nil
		})
		if !errors.Is(err, context.DeadlineExceeded) {
			return fmt.Errorf("waiter should preserve deadline, got %w", err)
		}
		return nil
	}

	tester := concurrencytest.NewAsyncJobTester(concurrencytest.Options{
		Workers:       4,
		RoundsPerTask: 8,
		Timeout:       5 * time.Second,
	})
	report := tester.RunT(t, task)
	if report.Completed != 8 {
		t.Fatalf("async cancellation should complete every round, got %+v", report)
	}
}

func TestStampedeCacheLeaseExpiryLetsPeerRecover(t *testing.T) {
	ctx := context.Background()
	client := redisClient(ctx, t)
	coord := newMemoryCoordinator(t, client, "lease-expiry")
	coord.cfg.lockTTL = 50 * time.Millisecond
	coord.cfg.pollInterval = 5 * time.Millisecond

	if err := client.Set(ctx, coord.lockKey("item"), "abandoned-owner", 50*time.Millisecond).Err(); err != nil {
		t.Fatalf("seed abandoned owner: %v", err)
	}

	value, err := coord.GetOrLoad(ctx, "item", time.Second, func(context.Context, string) (string, error) {
		return "recovered", nil
	})
	if err != nil {
		t.Fatalf("get or load after abandoned owner: %v", err)
	}
	if value != "recovered" {
		t.Fatalf("value = %q, want recovered", value)
	}
}

func redisClient(ctx context.Context, t *testing.T) *redis.Client {
	t.Helper()

	client := redis.NewClient(&redis.Options{Addr: redistestcontainer.Start(ctx, t)})
	t.Cleanup(func() {
		_ = client.Close()
	})
	waitForRedis(t, client)
	return client
}

func redisClients(ctx context.Context, t *testing.T) (*redis.Client, *redis.Client) {
	t.Helper()

	addr := redistestcontainer.Start(ctx, t)
	clientA := redis.NewClient(&redis.Options{Addr: addr})
	clientB := redis.NewClient(&redis.Options{Addr: addr})
	t.Cleanup(func() {
		_ = clientA.Close()
		_ = clientB.Close()
	})
	waitForRedis(t, clientA)
	waitForRedis(t, clientB)
	return clientA, clientB
}

func waitForRedis(t *testing.T, client *redis.Client) {
	t.Helper()

	bttesting.Eventually(t, 5*time.Second, func() bool {
		return client.Ping(context.Background()).Err() == nil
	})
}

func newMemoryCoordinator(t *testing.T, client redis.Cmdable, namespace string) *StampedeCache[string] {
	t.Helper()

	coord, err := NewStampedeCache[string](Options[string]{
		Client:       client,
		Cache:        cache.NewMemory[string, string](),
		Namespace:    namespace,
		Codec:        JSONCodec[string]{},
		LockTTL:      200 * time.Millisecond,
		ResultTTL:    200 * time.Millisecond,
		PollInterval: 5 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("new memory coordinator: %v", err)
	}
	return coord
}

func newNearCache(
	ctx context.Context,
	t *testing.T,
	client redisnear.Client,
	namespace string,
	originID string,
) *redisnear.NearCache[string] {
	t.Helper()

	near, err := redisnear.NewPubSub[string](ctx, redisnear.Options[string]{
		Client:    client,
		Namespace: namespace,
		OriginID:  originID,
	})
	if err != nil {
		t.Fatalf("new near cache: %v", err)
	}
	t.Cleanup(func() {
		_ = near.Close()
	})
	return near
}

func newNearCoordinator(
	t *testing.T,
	client redis.Cmdable,
	near *redisnear.NearCache[string],
	namespace string,
) *StampedeCache[string] {
	t.Helper()

	coord, err := NewStampedeCache[string](Options[string]{
		Client:       client,
		Cache:        near,
		Namespace:    namespace,
		Codec:        JSONCodec[string]{},
		LockTTL:      time.Second,
		ResultTTL:    time.Second,
		PollInterval: 5 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("new near coordinator: %v", err)
	}
	return coord
}

func primeNearCaches(
	ctx context.Context,
	t *testing.T,
	nearA *redisnear.NearCache[string],
	nearB *redisnear.NearCache[string],
	key string,
) {
	t.Helper()

	if _, err := nearA.GetOrLoad(ctx, key, time.Second, func(context.Context, string) (string, error) {
		return "stale-a", nil
	}); err != nil {
		t.Fatalf("prime near cache a: %v", err)
	}
	if _, err := nearB.GetOrLoad(ctx, key, time.Second, func(context.Context, string) (string, error) {
		return "stale-b", nil
	}); err != nil {
		t.Fatalf("prime near cache b: %v", err)
	}
}

func assertEventuallyMiss(t *testing.T, near *redisnear.NearCache[string], key string) {
	t.Helper()

	bttesting.Eventually(t, 5*time.Second, func() bool {
		_, err := near.Get(context.Background(), key)
		return errors.Is(err, cache.ErrCacheMiss)
	})
}

func assertEventuallyValue(t *testing.T, near *redisnear.NearCache[string], key string, expected string) {
	t.Helper()

	bttesting.Eventually(t, 5*time.Second, func() bool {
		value, err := near.Get(context.Background(), key)
		return err == nil && value == expected
	})
}

type loadResult[V any] struct {
	value V
	err   error
}

func assertLoadResult(t *testing.T, result loadResult[string], expected string) {
	t.Helper()

	if result.err != nil {
		t.Fatalf("load result error: %v", result.err)
	}
	if result.value != expected {
		t.Fatalf("value = %q, want %q", result.value, expected)
	}
}
