package cache_test

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	btcache "github.com/bluetape4k/bluetape-go/cache"
	"github.com/bluetape4k/bluetape-go/cache/redisnear"
	"github.com/bluetape4k/bluetape-go/cache/redisvalue"
	"github.com/bluetape4k/bluetape-go/serialization"
	redistestcontainer "github.com/bluetape4k/bluetape-go/testcontainers/redis"
	"github.com/redis/go-redis/v9"
)

const (
	cacheProviderBenchmarkEnv     = "BLUETAPE_CACHE_PROVIDER_BENCH"
	cacheProviderStartupLimit     = 90 * time.Second
	cacheProviderOperationLimit   = 10 * time.Second
	cacheProviderObservationLimit = 2 * time.Second
	cacheProviderCleanupLimit     = 10 * time.Second
	cacheProviderTTL              = time.Hour
	cacheRedisVersionAuthority    = "7.4"
	cacheRedisImageReference      = "redis:7.4-alpine@sha256:6ab0b6e7381779332f97b8ca76193e45b0756f38d4c0dcda72dbb3c32061ab99"
)

type cachePayloadProfile struct {
	name string
	size int
}

var cachePayloadProfiles = []cachePayloadProfile{
	{name: "128B", size: 128},
	{name: "4KiB", size: 4 << 10},
}

var (
	cacheProviderRecordSink *benchmarkCacheRecord
	cacheProviderBytesSink  []byte
)

type benchmarkCacheRecord struct {
	ID      string `json:"id"`
	Payload []byte `json:"payload"`
}

func benchmarkCacheValue(size int) *benchmarkCacheRecord {
	payload := make([]byte, size)
	for i := range payload {
		payload[i] = byte(i % 251)
	}
	return &benchmarkCacheRecord{ID: "benchmark-record", Payload: payload}
}

func newBenchmarkCacheID() (string, error) {
	var value [16]byte
	if _, err := rand.Read(value[:]); err != nil {
		return "", fmt.Errorf("generate cache benchmark id: %w", err)
	}
	return hex.EncodeToString(value[:]), nil
}

func observePeerInvalidation(ctx context.Context, invalidated func() bool) error {
	if ctx == nil {
		return errors.New("peer invalidation context must not be nil")
	}
	if invalidated == nil {
		return errors.New("peer invalidation observer must not be nil")
	}
	if invalidated() {
		return nil
	}
	ticker := time.NewTicker(100 * time.Microsecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if invalidated() {
				return nil
			}
		}
	}
}

type countingSerializer[V any] struct {
	delegate       serialization.Serializer[V]
	marshalCalls   atomic.Int64
	unmarshalCalls atomic.Int64
}

func newCountingSerializer[V any](delegate serialization.Serializer[V]) *countingSerializer[V] {
	return &countingSerializer[V]{delegate: delegate}
}

func (s *countingSerializer[V]) Marshal(value V) ([]byte, error) {
	s.marshalCalls.Add(1)
	return s.delegate.Marshal(value)
}

func (s *countingSerializer[V]) Unmarshal(data []byte) (V, error) {
	s.unmarshalCalls.Add(1)
	return s.delegate.Unmarshal(data)
}

func (s *countingSerializer[V]) counts() (int64, int64) {
	return s.marshalCalls.Load(), s.unmarshalCalls.Load()
}

func (s *countingSerializer[V]) reset() {
	s.marshalCalls.Store(0)
	s.unmarshalCalls.Store(0)
}

type cacheProviderCleanup struct {
	deleteNamespace func(context.Context) error
	closeNearCaches func(context.Context) error
	closeClient     func(context.Context) error
	once            sync.Once
	err             error
}

func (cleanup *cacheProviderCleanup) run(ctx context.Context) error {
	if cleanup == nil {
		return nil
	}
	cleanup.once.Do(func() {
		cleanup.err = errors.Join(
			runCacheProviderCleanupStage(ctx, cleanup.deleteNamespace),
			runCacheProviderCleanupStage(ctx, cleanup.closeNearCaches),
			runCacheProviderCleanupStage(ctx, cleanup.closeClient),
		)
	})
	return cleanup.err
}

func runCacheProviderCleanupStage(ctx context.Context, stage func(context.Context) error) error {
	if stage == nil {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	stageCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), cacheProviderCleanupLimit)
	defer cancel()
	return stage(stageCtx)
}

type cacheProviderNamespaces struct {
	mu     sync.Mutex
	values map[string]func(context.Context) error
}

func newCacheProviderNamespaces() *cacheProviderNamespaces {
	return &cacheProviderNamespaces{values: make(map[string]func(context.Context) error)}
}

func (namespaces *cacheProviderNamespaces) add(namespace string, clearNamespace func(context.Context) error) {
	namespaces.mu.Lock()
	defer namespaces.mu.Unlock()
	namespaces.values[namespace] = clearNamespace
}

func (namespaces *cacheProviderNamespaces) clear(ctx context.Context) error {
	namespaces.mu.Lock()
	keys := make([]string, 0, len(namespaces.values))
	for key := range namespaces.values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	clearers := make([]func(context.Context) error, 0, len(keys))
	for _, key := range keys {
		clearers = append(clearers, namespaces.values[key])
	}
	namespaces.mu.Unlock()

	var joined error
	for _, clear := range clearers {
		joined = errors.Join(joined, clear(ctx))
	}
	return joined
}

type nearErrorCollector struct {
	mu     sync.Mutex
	errors []error
}

func (collector *nearErrorCollector) report(_ context.Context, err error) {
	if err == nil {
		return
	}
	collector.mu.Lock()
	defer collector.mu.Unlock()
	collector.errors = append(collector.errors, err)
}

func (collector *nearErrorCollector) err() error {
	collector.mu.Lock()
	defer collector.mu.Unlock()
	return errors.Join(collector.errors...)
}

type cacheProviderNearResource struct {
	close     func() error
	collector *nearErrorCollector
}

type cacheProviderNearResources struct {
	mu        sync.Mutex
	resources []cacheProviderNearResource
}

func (resources *cacheProviderNearResources) add(cacheClose func() error, collector *nearErrorCollector) {
	resources.mu.Lock()
	defer resources.mu.Unlock()
	resources.resources = append(resources.resources, cacheProviderNearResource{close: cacheClose, collector: collector})
}

func (resources *cacheProviderNearResources) closeAll(ctx context.Context) error {
	resources.mu.Lock()
	owned := append([]cacheProviderNearResource(nil), resources.resources...)
	resources.mu.Unlock()

	results := make(chan error, len(owned))
	for _, resource := range owned {
		go func() {
			closeErr := resource.close()
			if resource.collector != nil {
				closeErr = errors.Join(closeErr, resource.collector.err())
			}
			results <- closeErr
		}()
	}
	var joined error
	for range owned {
		joined = errors.Join(joined, <-results)
	}
	return errors.Join(joined, ctx.Err())
}

type cacheProviderFixture struct {
	client          *redis.Client
	providerVersion string
	imageReference  string
	namespaces      *cacheProviderNamespaces
	nearResources   *cacheProviderNearResources
}

func newCacheProviderFixture(tb testing.TB) *cacheProviderFixture {
	tb.Helper()
	setupCtx, setupCancel := context.WithTimeout(context.Background(), cacheProviderStartupLimit)
	defer setupCancel()

	server := redistestcontainer.StartServer(setupCtx, tb)
	details, err := server.ConnectionDetails(setupCtx)
	if err != nil {
		tb.Fatalf("Redis connection details: %v", err)
	}
	address, err := details.Require(redistestcontainer.AddressKey)
	if err != nil {
		tb.Fatalf("Redis address: %v", err)
	}
	client := redis.NewClient(&redis.Options{Addr: address})
	namespaces := newCacheProviderNamespaces()
	nearResources := &cacheProviderNearResources{}
	cleanup := &cacheProviderCleanup{
		deleteNamespace: namespaces.clear,
		closeNearCaches: nearResources.closeAll,
		closeClient: func(context.Context) error {
			return client.Close()
		},
	}
	tb.Cleanup(func() {
		if err := cleanup.run(context.Background()); err != nil {
			tb.Errorf("cleanup Redis cache provider fixture: %v", err)
		}
	})

	if err := client.Ping(setupCtx).Err(); err != nil {
		tb.Fatalf("ping Redis cache provider: %v", err)
	}
	serverInfo, err := client.Info(setupCtx, "server").Result()
	if err != nil {
		tb.Fatalf("query Redis server version: %v", err)
	}
	providerVersion, err := parseCacheRedisVersion(serverInfo)
	if err != nil {
		tb.Fatal(err)
	}
	if !cacheProviderVersionMatchesAuthority(providerVersion, cacheRedisVersionAuthority) {
		tb.Fatalf(
			"Redis provider version %q does not match pinned image authority %q",
			sanitizeCacheProviderMetadata(providerVersion),
			cacheRedisVersionAuthority,
		)
	}
	return &cacheProviderFixture{
		client:          client,
		providerVersion: providerVersion,
		imageReference:  cacheRedisImageReference,
		namespaces:      namespaces,
		nearResources:   nearResources,
	}
}

func newBenchmarkValueCache(
	fixture *cacheProviderFixture,
	namespace string,
	serializer serialization.Serializer[*benchmarkCacheRecord],
	override func(*redisvalue.Config),
) (*redisvalue.ValueCache[*benchmarkCacheRecord], redisvalue.Config, error) {
	config := redisvalue.DefaultConfig()
	if override != nil {
		override(&config)
	}
	remote, err := redisvalue.NewValueCache(redisvalue.ValueOptions[*benchmarkCacheRecord]{
		Client:     fixture.client,
		Namespace:  namespace,
		Serializer: serializer,
		Config:     &config.Value,
	})
	if err != nil {
		return nil, redisvalue.Config{}, err
	}
	fixture.namespaces.add(namespace, remote.Clear)
	return remote, config, nil
}

func newBenchmarkTieredCache(
	fixture *cacheProviderFixture,
	namespace string,
	serializer serialization.Serializer[*benchmarkCacheRecord],
	override func(*redisvalue.Config),
) (*redisvalue.TieredCache[*benchmarkCacheRecord], *btcache.Memory[string, *benchmarkCacheRecord], error) {
	remote, config, err := newBenchmarkValueCache(fixture, namespace, serializer, override)
	if err != nil {
		return nil, nil, err
	}
	local := btcache.NewMemory[string, *benchmarkCacheRecord]()
	tiered, err := redisvalue.NewTieredCache(redisvalue.TieredOptions[*benchmarkCacheRecord]{
		Local:  local,
		Remote: remote,
		Config: &config.Tiered,
	})
	if err != nil {
		return nil, nil, err
	}
	return tiered, local, nil
}

func cacheProviderOperationContext() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), cacheProviderOperationLimit)
}

func mustBenchmarkCacheID(tb testing.TB) string {
	tb.Helper()
	value, err := newBenchmarkCacheID()
	if err != nil {
		tb.Fatal(err)
	}
	return value
}

func parseCacheRedisVersion(serverInfo string) (string, error) {
	for line := range strings.SplitSeq(serverInfo, "\n") {
		line = strings.TrimSpace(line)
		if version, ok := strings.CutPrefix(line, "redis_version:"); ok {
			version = strings.TrimSpace(version)
			if version != "" {
				return version, nil
			}
		}
	}
	return "", errors.New("Redis server info has no reported version")
}

func cacheProviderVersionMatchesAuthority(reported, authority string) bool {
	reported = strings.TrimSpace(reported)
	authority = strings.TrimSpace(authority)
	return reported != "" && authority != "" && (reported == authority || strings.HasPrefix(reported, authority+"."))
}

func sanitizeCacheProviderMetadata(value string) string {
	return strings.Map(func(r rune) rune {
		if r < 0x20 || r == 0x7f {
			return -1
		}
		return r
	}, value)
}

func BenchmarkProviderCacheLocal(b *testing.B) {
	b.Run("Memory", func(b *testing.B) {
		for _, scenario := range []string{"GetHit", "GetMiss", "Set", "GetOrLoadHot"} {
			b.Run(scenario, func(b *testing.B) {
				for _, profile := range cachePayloadProfiles {
					b.Run(profile.name, func(b *testing.B) {
						runMemoryCacheBenchmark(b, scenario, profile.size)
					})
				}
			})
		}
	})
	b.Run("SerializationBaseline", func(b *testing.B) {
		for _, scenario := range []string{"Marshal", "Unmarshal"} {
			b.Run(scenario, func(b *testing.B) {
				for _, profile := range cachePayloadProfiles {
					b.Run(profile.name, func(b *testing.B) {
						runCacheSerializationBenchmark(b, scenario, profile.size)
					})
				}
			})
		}
	})
}

func runMemoryCacheBenchmark(b *testing.B, scenario string, payloadSize int) {
	b.Helper()
	b.ReportAllocs()
	b.StopTimer()
	local := btcache.NewMemory[string, *benchmarkCacheRecord]()
	value := benchmarkCacheValue(payloadSize)
	key := mustBenchmarkCacheID(b)
	if scenario == "GetHit" || scenario == "GetOrLoadHot" {
		ctx, cancel := cacheProviderOperationContext()
		err := local.Set(ctx, key, value, cacheProviderTTL)
		cancel()
		if err != nil {
			b.Fatalf("seed memory cache: %v", err)
		}
	}
	var loaderCalls atomic.Int64

	for range b.N {
		ctx, cancel := cacheProviderOperationContext()
		b.StartTimer()
		var got *benchmarkCacheRecord
		var err error
		switch scenario {
		case "GetHit", "GetMiss":
			got, err = local.Get(ctx, key)
		case "Set":
			err = local.Set(ctx, key, value, cacheProviderTTL)
		case "GetOrLoadHot":
			got, err = local.GetOrLoad(ctx, key, cacheProviderTTL, func(context.Context, string) (*benchmarkCacheRecord, error) {
				loaderCalls.Add(1)
				return value, nil
			})
		default:
			err = fmt.Errorf("unknown memory cache scenario %q", scenario)
		}
		b.StopTimer()
		cancel()

		if scenario == "GetMiss" {
			if !errors.Is(err, btcache.ErrCacheMiss) {
				b.Fatalf("memory cache miss: %v", err)
			}
			continue
		}
		if err != nil {
			b.Fatalf("memory cache %s: %v", scenario, err)
		}
		if (scenario == "GetHit" || scenario == "GetOrLoadHot") && got != value {
			b.Fatalf("memory cache %s did not preserve the stored reference", scenario)
		}
		cacheProviderRecordSink = got
	}
	if scenario == "GetOrLoadHot" && loaderCalls.Load() != 0 {
		b.Fatalf("hot loader calls = %d, want 0", loaderCalls.Load())
	}
}

func runCacheSerializationBenchmark(b *testing.B, scenario string, payloadSize int) {
	b.Helper()
	b.ReportAllocs()
	b.StopTimer()
	serializer := newCountingSerializer[*benchmarkCacheRecord](serialization.NewJSONSerializer[*benchmarkCacheRecord]())
	value := benchmarkCacheValue(payloadSize)
	encoded, err := serializer.Marshal(value)
	if err != nil {
		b.Fatalf("seed serialization baseline: %v", err)
	}
	serializer.reset()

	for range b.N {
		b.StartTimer()
		switch scenario {
		case "Marshal":
			cacheProviderBytesSink, err = serializer.Marshal(value)
		case "Unmarshal":
			cacheProviderRecordSink, err = serializer.Unmarshal(encoded)
		default:
			err = fmt.Errorf("unknown serialization scenario %q", scenario)
		}
		b.StopTimer()
		if err != nil {
			b.Fatalf("serialization baseline %s: %v", scenario, err)
		}
	}
	marshalCalls, unmarshalCalls := serializer.counts()
	if scenario == "Marshal" && (marshalCalls != int64(b.N) || unmarshalCalls != 0) {
		b.Fatalf("marshal baseline calls = %d/%d, want %d/0", marshalCalls, unmarshalCalls, b.N)
	}
	if scenario == "Unmarshal" && (marshalCalls != 0 || unmarshalCalls != int64(b.N)) {
		b.Fatalf("unmarshal baseline calls = %d/%d, want 0/%d", marshalCalls, unmarshalCalls, b.N)
	}
}

func BenchmarkProviderCacheRedis(b *testing.B) {
	if os.Getenv(cacheProviderBenchmarkEnv) != "1" {
		b.Skipf("set %s=1 to run Redis cache provider benchmarks", cacheProviderBenchmarkEnv)
	}
	b.StopTimer()
	fixture := newCacheProviderFixture(b)
	if err := runCacheProviderPreflight(fixture); err != nil {
		b.Fatalf("Redis cache provider preflight: %v", err)
	}
	b.Logf(
		"provider_version=%q image_reference=%q",
		sanitizeCacheProviderMetadata(fixture.providerVersion),
		sanitizeCacheProviderMetadata(fixture.imageReference),
	)

	b.Run("RedisL2", func(b *testing.B) {
		for _, scenario := range []string{"GetHit", "GetMiss", "Set", "Delete"} {
			b.Run(scenario, func(b *testing.B) {
				for _, profile := range cachePayloadProfiles {
					b.Run(profile.name, func(b *testing.B) {
						runRedisL2CacheBenchmark(b, fixture, scenario, profile.size)
					})
				}
			})
		}
	})
	b.Run("Tiered", func(b *testing.B) {
		for _, scenario := range []string{"L1Hit", "L2Hit", "LoadMiss", "WriteThrough"} {
			b.Run(scenario, func(b *testing.B) {
				for _, profile := range cachePayloadProfiles {
					b.Run(profile.name, func(b *testing.B) {
						runTieredCacheBenchmark(b, fixture, scenario, profile.size)
					})
				}
			})
		}
	})
	b.Run("NearCachePubSub", func(b *testing.B) {
		for _, scenario := range []string{"LocalHit", "LocalMiss", "PublishSet", "PublishDelete", "PeerInvalidation"} {
			b.Run(scenario, func(b *testing.B) {
				for _, profile := range cachePayloadProfiles {
					b.Run(profile.name, func(b *testing.B) {
						runNearCacheBenchmark(b, fixture, scenario, profile.size)
					})
				}
			})
		}
	})
}

func runCacheProviderPreflight(fixture *cacheProviderFixture) error {
	namespace, err := newBenchmarkCacheID()
	if err != nil {
		return err
	}
	serializer := newCountingSerializer[*benchmarkCacheRecord](serialization.NewJSONSerializer[*benchmarkCacheRecord]())
	tiered, _, err := newBenchmarkTieredCache(fixture, namespace, serializer, func(config *redisvalue.Config) {
		config.Value.RemoteTTL = cacheProviderTTL
		config.Tiered.LocalTTL = time.Minute
	})
	if err != nil {
		return err
	}
	want := benchmarkCacheValue(128)
	ctx, cancel := cacheProviderOperationContext()
	defer cancel()
	if err := tiered.Set(ctx, "semantic-preflight", want, cacheProviderTTL); err != nil {
		return err
	}
	if marshal, unmarshal := serializer.counts(); marshal != 1 || unmarshal != 0 {
		return fmt.Errorf("preflight write serialization calls = %d/%d, want 1/0", marshal, unmarshal)
	}
	got, err := tiered.Get(ctx, "semantic-preflight")
	if err != nil {
		return err
	}
	if got != want {
		return errors.New("preflight L1 hit did not preserve the decoded reference")
	}
	if marshal, unmarshal := serializer.counts(); marshal != 1 || unmarshal != 0 {
		return fmt.Errorf("preflight L1 hit serialization calls = %d/%d, want 1/0", marshal, unmarshal)
	}
	if err := tiered.InvalidateLocal(ctx, "semantic-preflight"); err != nil {
		return err
	}
	got, err = tiered.Get(ctx, "semantic-preflight")
	if err != nil {
		return err
	}
	if got == want || !reflect.DeepEqual(got, want) {
		return errors.New("preflight L2 hit did not decode an equivalent fresh value")
	}
	if marshal, unmarshal := serializer.counts(); marshal != 1 || unmarshal != 1 {
		return fmt.Errorf("preflight L2 hit serialization calls = %d/%d, want 1/1", marshal, unmarshal)
	}
	return tiered.Clear(ctx)
}

func runRedisL2CacheBenchmark(b *testing.B, fixture *cacheProviderFixture, scenario string, payloadSize int) {
	b.Helper()
	b.ReportAllocs()
	b.StopTimer()
	namespace := mustBenchmarkCacheID(b)
	serializer := newCountingSerializer[*benchmarkCacheRecord](serialization.NewJSONSerializer[*benchmarkCacheRecord]())
	remote, _, err := newBenchmarkValueCache(fixture, namespace, serializer, func(config *redisvalue.Config) {
		config.Value.RemoteTTL = cacheProviderTTL
	})
	if err != nil {
		b.Fatalf("new Redis L2 cache: %v", err)
	}
	key := mustBenchmarkCacheID(b)
	value := benchmarkCacheValue(payloadSize)
	if scenario == "GetHit" {
		ctx, cancel := cacheProviderOperationContext()
		err = remote.Set(ctx, key, value, cacheProviderTTL)
		cancel()
		if err != nil {
			b.Fatalf("seed Redis L2 hit: %v", err)
		}
	}

	for range b.N {
		if scenario == "Delete" {
			ctx, cancel := cacheProviderOperationContext()
			err = remote.Set(ctx, key, value, cacheProviderTTL)
			cancel()
			if err != nil {
				b.Fatalf("seed Redis L2 delete: %v", err)
			}
		}
		serializer.reset()
		ctx, cancel := cacheProviderOperationContext()
		var got *benchmarkCacheRecord
		b.StartTimer()
		switch scenario {
		case "GetHit", "GetMiss":
			got, err = remote.Get(ctx, key)
		case "Set":
			err = remote.Set(ctx, key, value, cacheProviderTTL)
		case "Delete":
			err = remote.Delete(ctx, key)
		default:
			err = fmt.Errorf("unknown Redis L2 scenario %q", scenario)
		}
		b.StopTimer()
		cancel()

		if scenario == "GetMiss" {
			if !errors.Is(err, btcache.ErrCacheMiss) {
				b.Fatalf("Redis L2 miss: %v", err)
			}
		} else if err != nil {
			b.Fatalf("Redis L2 %s: %v", scenario, err)
		}
		if scenario == "GetHit" && !reflect.DeepEqual(got, value) {
			b.Fatalf("Redis L2 hit = %#v, want %#v", got, value)
		}
		if err := verifyCacheSerializationCalls(scenario, serializer); err != nil {
			b.Fatal(err)
		}
		cacheProviderRecordSink = got
	}
}

func verifyCacheSerializationCalls(scenario string, serializer *countingSerializer[*benchmarkCacheRecord]) error {
	marshalCalls, unmarshalCalls := serializer.counts()
	wantMarshal, wantUnmarshal := int64(0), int64(0)
	switch scenario {
	case "GetHit", "L2Hit":
		wantUnmarshal = 1
	case "Set", "LoadMiss", "WriteThrough":
		wantMarshal = 1
	case "GetMiss", "Delete", "L1Hit":
	default:
		return fmt.Errorf("unknown serialization expectation for %q", scenario)
	}
	if marshalCalls != wantMarshal || unmarshalCalls != wantUnmarshal {
		return fmt.Errorf("%s serialization calls = %d/%d, want %d/%d", scenario, marshalCalls, unmarshalCalls, wantMarshal, wantUnmarshal)
	}
	return nil
}

func runTieredCacheBenchmark(b *testing.B, fixture *cacheProviderFixture, scenario string, payloadSize int) {
	b.Helper()
	b.ReportAllocs()
	b.StopTimer()
	namespace := mustBenchmarkCacheID(b)
	serializer := newCountingSerializer[*benchmarkCacheRecord](serialization.NewJSONSerializer[*benchmarkCacheRecord]())
	tiered, local, err := newBenchmarkTieredCache(fixture, namespace, serializer, func(config *redisvalue.Config) {
		config.Value.RemoteTTL = cacheProviderTTL
		config.Tiered.LocalTTL = time.Minute
	})
	if err != nil {
		b.Fatalf("new tiered cache: %v", err)
	}
	key := mustBenchmarkCacheID(b)
	value := benchmarkCacheValue(payloadSize)
	if scenario == "L1Hit" {
		ctx, cancel := cacheProviderOperationContext()
		err = tiered.Set(ctx, key, value, cacheProviderTTL)
		cancel()
		if err != nil {
			b.Fatalf("seed tiered L1 hit: %v", err)
		}
	}

	for range b.N {
		var loaded *benchmarkCacheRecord
		switch scenario {
		case "L2Hit":
			ctx, cancel := cacheProviderOperationContext()
			err = tiered.Set(ctx, key, value, cacheProviderTTL)
			if err == nil {
				err = tiered.InvalidateLocal(ctx, key)
			}
			cancel()
		case "LoadMiss":
			ctx, cancel := cacheProviderOperationContext()
			err = tiered.Delete(ctx, key)
			cancel()
		}
		if err != nil {
			b.Fatalf("prepare tiered %s: %v", scenario, err)
		}
		if scenario == "LoadMiss" {
			loaded = benchmarkCacheValue(payloadSize)
		}
		serializer.reset()
		ctx, cancel := cacheProviderOperationContext()
		var got *benchmarkCacheRecord
		b.StartTimer()
		switch scenario {
		case "L1Hit", "L2Hit":
			got, err = tiered.Get(ctx, key)
		case "LoadMiss":
			got, err = tiered.GetOrLoad(ctx, key, cacheProviderTTL, func(context.Context, string) (*benchmarkCacheRecord, error) {
				return loaded, nil
			})
		case "WriteThrough":
			err = tiered.Set(ctx, key, value, cacheProviderTTL)
		default:
			err = fmt.Errorf("unknown tiered cache scenario %q", scenario)
		}
		b.StopTimer()
		cancel()
		if err != nil {
			b.Fatalf("tiered cache %s: %v", scenario, err)
		}
		if err := verifyCacheSerializationCalls(scenario, serializer); err != nil {
			b.Fatal(err)
		}
		if scenario == "L1Hit" && got != value {
			b.Fatal("tiered L1 hit did not preserve the decoded reference")
		}
		if scenario == "L2Hit" && (got == value || !reflect.DeepEqual(got, value)) {
			b.Fatal("tiered L2 hit did not decode an equivalent fresh value")
		}
		if scenario == "LoadMiss" && got != loaded {
			b.Fatal("tiered load miss did not retain the loader reference in L1")
		}
		if scenario == "WriteThrough" {
			verifyCtx, verifyCancel := cacheProviderOperationContext()
			localValue, localErr := local.Get(verifyCtx, key)
			verifyCancel()
			if localErr != nil || localValue != value {
				b.Fatalf("tiered write-through L1 reference = %p/%v, want %p", localValue, localErr, value)
			}
		}
		cacheProviderRecordSink = got
	}
}

func runNearCacheBenchmark(b *testing.B, fixture *cacheProviderFixture, scenario string, payloadSize int) {
	b.Helper()
	b.ReportAllocs()
	b.StopTimer()
	namespace := mustBenchmarkCacheID(b)
	value := benchmarkCacheValue(payloadSize)
	key := mustBenchmarkCacheID(b)
	localA := btcache.NewMemory[string, *benchmarkCacheRecord]()
	nearA, errorsA, err := newBenchmarkNearCache(fixture, namespace, localA)
	if err != nil {
		b.Fatalf("new near cache: %v", err)
	}
	var nearB *redisnear.NearCache[*benchmarkCacheRecord]
	var localB *btcache.Memory[string, *benchmarkCacheRecord]
	var errorsB *nearErrorCollector
	if scenario == "PeerInvalidation" {
		localB = btcache.NewMemory[string, *benchmarkCacheRecord]()
		nearB, errorsB, err = newBenchmarkNearCache(fixture, namespace, localB)
		if err != nil {
			b.Fatalf("new peer near cache: %v", err)
		}
	}
	if scenario == "LocalHit" {
		ctx, cancel := cacheProviderOperationContext()
		err = nearA.Set(ctx, key, value, cacheProviderTTL)
		cancel()
		if err != nil {
			b.Fatalf("seed near-cache local hit: %v", err)
		}
	}

	for range b.N {
		switch scenario {
		case "PublishDelete":
			ctx, cancel := cacheProviderOperationContext()
			err = localA.Set(ctx, key, value, cacheProviderTTL)
			cancel()
		case "PeerInvalidation":
			ctx, cancel := cacheProviderOperationContext()
			err = localB.Set(ctx, key, value, cacheProviderTTL)
			cancel()
		}
		if err != nil {
			b.Fatalf("prepare near-cache %s: %v", scenario, err)
		}

		opCtx, opCancel := cacheProviderOperationContext()
		observationCtx, observationCancel := context.WithTimeout(opCtx, cacheProviderObservationLimit)
		var got *benchmarkCacheRecord
		var observationErr error
		b.StartTimer()
		switch scenario {
		case "LocalHit", "LocalMiss":
			got, err = nearA.Get(opCtx, key)
		case "PublishSet":
			err = nearA.Set(opCtx, key, value, cacheProviderTTL)
		case "PublishDelete":
			err = nearA.Delete(opCtx, key)
		case "PeerInvalidation":
			err = nearA.Set(opCtx, key, value, cacheProviderTTL)
			if err == nil {
				err = observePeerInvalidation(observationCtx, func() bool {
					_, getErr := nearB.Get(opCtx, key)
					if errors.Is(getErr, btcache.ErrCacheMiss) {
						return true
					}
					if getErr != nil {
						observationErr = getErr
						return true
					}
					return false
				})
			}
		default:
			err = fmt.Errorf("unknown near-cache scenario %q", scenario)
		}
		b.StopTimer()
		observationCancel()
		opCancel()

		if scenario == "LocalMiss" {
			if !errors.Is(err, btcache.ErrCacheMiss) {
				b.Fatalf("near-cache local miss: %v", err)
			}
			err = nil
		}
		if joined := errors.Join(err, observationErr, errorsA.err()); joined != nil {
			b.Fatalf("near-cache %s: %v", scenario, joined)
		}
		if errorsB != nil {
			if err := errorsB.err(); err != nil {
				b.Fatalf("near-cache peer subscriber: %v", err)
			}
		}
		if scenario == "LocalHit" && got != value {
			b.Fatal("near-cache local hit did not preserve the decoded reference")
		}
		cacheProviderRecordSink = got
	}
}

func newBenchmarkNearCache(
	fixture *cacheProviderFixture,
	namespace string,
	local *btcache.Memory[string, *benchmarkCacheRecord],
) (*redisnear.NearCache[*benchmarkCacheRecord], *nearErrorCollector, error) {
	originID, err := newBenchmarkCacheID()
	if err != nil {
		return nil, nil, err
	}
	collector := &nearErrorCollector{}
	ctx, cancel := cacheProviderOperationContext()
	defer cancel()
	near, err := redisnear.NewPubSub(ctx, redisnear.Options[*benchmarkCacheRecord]{
		Client:    fixture.client,
		Namespace: namespace,
		OriginID:  originID,
		Local:     local,
		OnError:   collector.report,
	})
	if err != nil {
		return nil, nil, err
	}
	fixture.nearResources.add(near.Close, collector)
	return near, collector, nil
}

func TestBenchmarkPayloadSizes(t *testing.T) {
	for _, size := range []int{128, 4 << 10} {
		value := benchmarkCacheValue(size)
		if len(value.Payload) != size {
			t.Fatalf("size=%d got=%d", size, len(value.Payload))
		}
	}
}

func TestObservePeerInvalidationTimesOutAndDrains(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	err := observePeerInvalidation(ctx, func() bool { return false })
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("error=%v", err)
	}
}

func TestBenchmarkCacheIDIsLowercaseHex(t *testing.T) {
	value, err := newBenchmarkCacheID()
	if err != nil {
		t.Fatal(err)
	}
	if !regexp.MustCompile(`^[0-9a-f]{32}$`).MatchString(value) {
		t.Fatalf("benchmark cache ID = %q", value)
	}
}

func TestCountingSerializerTracksOnlySerializationBoundary(t *testing.T) {
	serializer := newCountingSerializer[*benchmarkCacheRecord](serialization.NewJSONSerializer[*benchmarkCacheRecord]())
	want := benchmarkCacheValue(128)
	encoded, err := serializer.Marshal(want)
	if err != nil {
		t.Fatal(err)
	}
	got, err := serializer.Unmarshal(encoded)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("round trip = %#v, want %#v", got, want)
	}
	if marshal, unmarshal := serializer.counts(); marshal != 1 || unmarshal != 1 {
		t.Fatalf("serializer counts = %d/%d, want 1/1", marshal, unmarshal)
	}
}

func TestCacheProviderCleanupRunsEveryStageOnceInOrder(t *testing.T) {
	var calls []string
	namespaceErr := errors.New("namespace")
	nearErr := errors.New("near")
	clientErr := errors.New("client")
	cleanup := &cacheProviderCleanup{
		deleteNamespace: func(context.Context) error {
			calls = append(calls, "namespace")
			return namespaceErr
		},
		closeNearCaches: func(context.Context) error {
			calls = append(calls, "near")
			return nearErr
		},
		closeClient: func(context.Context) error {
			calls = append(calls, "client")
			return clientErr
		},
	}

	err := cleanup.run(context.Background())
	secondErr := cleanup.run(context.Background())
	if !errors.Is(err, namespaceErr) || !errors.Is(err, nearErr) || !errors.Is(err, clientErr) {
		t.Fatalf("cleanup error = %v", err)
	}
	if !errors.Is(secondErr, namespaceErr) || !errors.Is(secondErr, nearErr) || !errors.Is(secondErr, clientErr) {
		t.Fatalf("second cleanup error = %v", secondErr)
	}
	if want := []string{"namespace", "near", "client"}; !reflect.DeepEqual(calls, want) {
		t.Fatalf("cleanup calls = %v, want %v", calls, want)
	}
}
