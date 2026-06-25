package redisbloom

import (
	"context"
	"errors"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/bluetape4k/bluetape-go/internal/testcleanup"
	"github.com/bluetape4k/bluetape-go/probabilistic"
	"github.com/redis/go-redis/v9"
	tcredis "github.com/testcontainers/testcontainers-go/modules/redis"
)

var redisFixture struct {
	once      sync.Once
	container *tcredis.RedisContainer
	addr      string
	err       error
}

func redisAddr(ctx context.Context) (string, error) {
	redisFixture.once.Do(func() {
		container, err := tcredis.Run(ctx, "redis:7.4-alpine")
		if err != nil {
			redisFixture.err = err
			return
		}
		addr, err := container.PortEndpoint(ctx, "6379/tcp", "")
		if err != nil {
			redisFixture.err = err
			_ = testcleanup.Terminate(ctx, 0, container)
			return
		}
		redisFixture.container = container
		redisFixture.addr = addr
	})
	return redisFixture.addr, redisFixture.err
}

func newRedisClient(tb testing.TB) *redis.Client {
	tb.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	tb.Cleanup(cancel)
	addr, err := redisAddr(ctx)
	if err != nil {
		tb.Fatalf("start redis container: %v", err)
	}
	client := redis.NewClient(&redis.Options{Addr: addr})
	tb.Cleanup(func() {
		if err := client.Close(); err != nil {
			tb.Fatalf("close redis client: %v", err)
		}
	})
	waitForRedis(tb, client)
	if err := client.FlushDB(ctx).Err(); err != nil {
		tb.Fatalf("flush redis db: %v", err)
	}
	return client
}

func waitForRedis(tb testing.TB, client *redis.Client) {
	tb.Helper()
	deadline := time.Now().Add(10 * time.Second)
	var lastErr error
	for time.Now().Before(deadline) {
		err := client.Ping(context.Background()).Err()
		if err == nil {
			return
		}
		lastErr = err
		time.Sleep(25 * time.Millisecond)
	}
	tb.Fatalf("redis did not become ready: %v", lastErr)
}

func TestMain(m *testing.M) {
	code := m.Run()
	if redisFixture.container != nil {
		ctx, cancel := context.WithTimeout(context.Background(), testcleanup.DefaultTerminateTimeout)
		_ = testcleanup.Terminate(ctx, 0, redisFixture.container)
		cancel()
	}
	os.Exit(code)
}

func testConfig(tb testing.TB, expected uint64) probabilistic.Config {
	tb.Helper()
	cfg, err := probabilistic.NewConfig(expected, 0.01)
	if err != nil {
		tb.Fatalf("NewConfig failed: %v", err)
	}
	return cfg
}

func testNamespace(tb testing.TB) string {
	tb.Helper()
	return "test:" + strings.ReplaceAll(tb.Name(), "/", ":")
}

func cleanupNamespace(tb testing.TB, client redis.Cmdable, namespace string) {
	tb.Helper()
	keys, err := buildKeys(namespace)
	if err != nil {
		tb.Fatalf("buildKeys failed: %v", err)
	}
	tb.Cleanup(func() {
		if err := client.Del(context.Background(), keys.bits, keys.config).Err(); err != nil {
			tb.Fatalf("cleanup redis keys: %v", err)
		}
	})
}

func TestNewBloomFilterInitializesAndReusesConfig(t *testing.T) {
	ctx := context.Background()
	client := newRedisClient(t)
	namespace := testNamespace(t)
	cleanupNamespace(t, client, namespace)

	filter, err := NewStringBloomFilter(ctx, client, namespace, testConfig(t, 1000))
	if err != nil {
		t.Fatalf("NewStringBloomFilter failed: %v", err)
	}
	if filter.ExpectedInsertions() != 1000 {
		t.Fatalf("ExpectedInsertions = %d, want 1000", filter.ExpectedInsertions())
	}

	again, err := NewStringBloomFilter(ctx, client, namespace, testConfig(t, 1000))
	if err != nil {
		t.Fatalf("NewStringBloomFilter reused config failed: %v", err)
	}
	if filter.BitSize() != again.BitSize() {
		t.Fatalf("BitSize mismatch: %d != %d", filter.BitSize(), again.BitSize())
	}
}

func TestNewBloomFilterRejectsChangedConfig(t *testing.T) {
	ctx := context.Background()
	client := newRedisClient(t)
	namespace := testNamespace(t)
	cleanupNamespace(t, client, namespace)

	if _, err := NewStringBloomFilter(ctx, client, namespace, testConfig(t, 1000)); err != nil {
		t.Fatalf("NewStringBloomFilter failed: %v", err)
	}

	_, err := NewStringBloomFilter(ctx, client, namespace, testConfig(t, 2000))
	if !errors.Is(err, ErrConfigMismatch) {
		t.Fatalf("expected ErrConfigMismatch, got %v", err)
	}
}

func TestNewBloomFilterRejectsCorruptMetadataWithBitmap(t *testing.T) {
	ctx := context.Background()
	client := newRedisClient(t)
	namespace := testNamespace(t)
	cleanupNamespace(t, client, namespace)

	if _, err := NewStringBloomFilter(ctx, client, namespace, testConfig(t, 1000)); err != nil {
		t.Fatalf("NewStringBloomFilter failed: %v", err)
	}
	keys, err := buildKeys(namespace)
	if err != nil {
		t.Fatalf("buildKeys failed: %v", err)
	}
	if err := client.SetBit(ctx, keys.bits, 0, 1).Err(); err != nil {
		t.Fatalf("set bitmap bit: %v", err)
	}
	if err := client.HDel(ctx, keys.config, "fingerprint").Err(); err != nil {
		t.Fatalf("delete fingerprint: %v", err)
	}

	_, err = NewStringBloomFilter(ctx, client, namespace, testConfig(t, 1000))
	if !errors.Is(err, ErrConfigCorrupt) {
		t.Fatalf("expected ErrConfigCorrupt, got %v", err)
	}
}

func TestNewBloomFilterRejectsMissingConfigWithBitmap(t *testing.T) {
	ctx := context.Background()
	client := newRedisClient(t)
	namespace := testNamespace(t)
	cleanupNamespace(t, client, namespace)

	if _, err := NewStringBloomFilter(ctx, client, namespace, testConfig(t, 1000)); err != nil {
		t.Fatalf("NewStringBloomFilter failed: %v", err)
	}
	keys, err := buildKeys(namespace)
	if err != nil {
		t.Fatalf("buildKeys failed: %v", err)
	}
	if err := client.SetBit(ctx, keys.bits, 0, 1).Err(); err != nil {
		t.Fatalf("set bitmap bit: %v", err)
	}
	if err := client.Del(ctx, keys.config).Err(); err != nil {
		t.Fatalf("delete config: %v", err)
	}

	_, err = NewStringBloomFilter(ctx, client, namespace, testConfig(t, 1000))
	if !errors.Is(err, ErrConfigCorrupt) {
		t.Fatalf("expected ErrConfigCorrupt, got %v", err)
	}
}

func TestNewBloomFilterRejectsPartialMetadataEvenWithFingerprint(t *testing.T) {
	ctx := context.Background()
	client := newRedisClient(t)
	namespace := testNamespace(t)
	cleanupNamespace(t, client, namespace)

	filter, err := NewStringBloomFilter(ctx, client, namespace, testConfig(t, 1000))
	if err != nil {
		t.Fatalf("NewStringBloomFilter failed: %v", err)
	}

	keys, err := buildKeys(namespace)
	if err != nil {
		t.Fatalf("buildKeys failed: %v", err)
	}
	fingerprint := client.HGet(ctx, keys.config, "fingerprint").Val()
	if err := client.Del(ctx, keys.config).Err(); err != nil {
		t.Fatalf("delete config: %v", err)
	}
	if err := client.HSet(ctx, keys.config, "fingerprint", fingerprint, "expected_insertions", "1000").Err(); err != nil {
		t.Fatalf("write partial config: %v", err)
	}

	_, err = NewStringBloomFilter(ctx, client, namespace, testConfig(t, 1000))
	if !errors.Is(err, ErrConfigCorrupt) {
		t.Fatalf("expected ErrConfigCorrupt, got %v", err)
	}
	if filter.HasherKey() != "probabilistic:string:v1" {
		t.Fatalf("HasherKey = %q", filter.HasherKey())
	}
}

func TestConcurrentIncompatibleConstructorsLeaveOneConfig(t *testing.T) {
	ctx := context.Background()
	client := newRedisClient(t)
	namespace := testNamespace(t)
	cleanupNamespace(t, client, namespace)

	var wg sync.WaitGroup
	errs := make(chan error, 2)
	for _, expected := range []uint64{1000, 2000} {
		wg.Add(1)
		go func(expected uint64) {
			defer wg.Done()
			_, err := NewStringBloomFilter(ctx, client, namespace, testConfig(t, expected))
			errs <- err
		}(expected)
	}
	wg.Wait()
	close(errs)

	var success, mismatch int
	for err := range errs {
		if err == nil {
			success++
			continue
		}
		if errors.Is(err, ErrConfigMismatch) {
			mismatch++
		}
	}
	if success != 1 || mismatch != 1 {
		t.Fatalf("success=%d mismatch=%d, want 1/1", success, mismatch)
	}

	keys, err := buildKeys(namespace)
	if err != nil {
		t.Fatalf("buildKeys failed: %v", err)
	}
	if exists := client.Exists(ctx, keys.bits).Val(); exists != 0 {
		t.Fatalf("bitmap key exists = %d, want 0", exists)
	}
	metadata := client.HGetAll(ctx, keys.config).Val()
	if len(metadata) != 8 {
		t.Fatalf("metadata len = %d, want 8: %#v", len(metadata), metadata)
	}
	if metadata["version"] != "1" {
		t.Fatalf("version = %q, want 1", metadata["version"])
	}
	if metadata["family"] != "redis-bloom" {
		t.Fatalf("family = %q, want redis-bloom", metadata["family"])
	}
	if metadata["expected_insertions"] != "1000" && metadata["expected_insertions"] != "2000" {
		t.Fatalf("expected_insertions = %q", metadata["expected_insertions"])
	}
	for _, field := range []string{"false_positive_probability", "bit_size", "hash_function_count", "fingerprint"} {
		if metadata[field] == "" {
			t.Fatalf("metadata field %q is empty: %#v", field, metadata)
		}
	}
	if metadata["hasher_key"] != "probabilistic:string:v1" {
		t.Fatalf("hasher_key = %q", metadata["hasher_key"])
	}
}
