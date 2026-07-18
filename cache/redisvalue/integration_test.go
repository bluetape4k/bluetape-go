package redisvalue

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/bluetape4k/bluetape-go/cache"
	"github.com/bluetape4k/bluetape-go/serialization"
	redistestcontainer "github.com/bluetape4k/bluetape-go/testcontainers/redis"
	bttesting "github.com/bluetape4k/bluetape-go/testing"
	"github.com/redis/go-redis/v9"
)

type integrationRecord struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

func TestRedisValueIntegration(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	t.Cleanup(cancel)
	addr := redistestcontainer.Start(ctx, t)
	client := redis.NewClient(&redis.Options{Addr: addr})
	t.Cleanup(func() {
		if err := client.Close(); err != nil {
			t.Errorf("close Redis client: %v", err)
		}
	})
	bttesting.Eventually(t, 5*time.Second, func() bool { return client.Ping(ctx).Err() == nil })

	t.Run("lifecycle-ttl-and-bounds", func(t *testing.T) {
		values := integrationValueCache(t, client, "lifecycle", serialization.NewJSONSerializer[integrationRecord](), ValueConfig{
			RemoteTTL: time.Hour, MaxValueBytes: 64, ClearBatchSize: 2,
		})
		want := integrationRecord{Name: "stored", Count: 7}
		if _, err := values.Get(ctx, "missing"); !errors.Is(err, cache.ErrCacheMiss) {
			t.Fatalf("missing Get() = %v", err)
		}
		if err := values.Set(ctx, "item", want, time.Minute); err != nil {
			t.Fatal(err)
		}
		if got, err := values.Get(ctx, "item"); err != nil || got != want {
			t.Fatalf("Get() = %+v/%v", got, err)
		}
		if err := values.Delete(ctx, "item"); err != nil {
			t.Fatal(err)
		}
		if _, err := values.Get(ctx, "item"); !errors.Is(err, cache.ErrCacheMiss) {
			t.Fatalf("deleted Get() = %v", err)
		}

		if err := values.Set(ctx, "finite", want, 25*time.Millisecond); err != nil {
			t.Fatal(err)
		}
		bttesting.Eventually(t, 3*time.Second, func() bool {
			_, err := values.Get(ctx, "finite")
			return errors.Is(err, cache.ErrCacheMiss)
		})
		if err := values.Set(ctx, "persistent", want, 0); err != nil {
			t.Fatal(err)
		}
		persistentKey := integrationPhysicalKey(t, values, "persistent")
		if ttl, err := client.PTTL(ctx, persistentKey).Result(); err != nil || ttl != -1 {
			t.Fatalf("persistent PTTL = %s/%v", ttl, err)
		}

		if err := values.Set(ctx, "submillisecond", want, time.Microsecond); err != nil {
			t.Fatal(err)
		}
		bttesting.Eventually(t, 3*time.Second, func() bool {
			_, err := values.Get(ctx, "submillisecond")
			return errors.Is(err, cache.ErrCacheMiss)
		})

		maxValue := integrationRecord{Name: strings.Repeat("x", 40)}
		encoded, err := values.serializer.Marshal(maxValue)
		if err != nil {
			t.Fatal(err)
		}
		bounded := integrationValueCache(t, client, "bounds", serialization.NewJSONSerializer[integrationRecord](), ValueConfig{
			RemoteTTL: time.Hour, MaxValueBytes: len(encoded), ClearBatchSize: 2,
		})
		if err := bounded.Set(ctx, "maximum", maxValue, time.Minute); err != nil {
			t.Fatalf("maximum Set() = %v", err)
		}
		if err := bounded.Set(ctx, "oversize", integrationRecord{Name: maxValue.Name + "x"}, time.Minute); !hasReason(err, ReasonPayloadTooLarge) {
			t.Fatalf("oversize Set() = %v", err)
		}
	})

	t.Run("empty-payload-is-not-a-miss", func(t *testing.T) {
		values := integrationValueCache(t, client, "empty", serialization.StringSerializer{}, ValueConfig{
			RemoteTTL: time.Hour, MaxValueBytes: 8, ClearBatchSize: 2,
		})
		if err := values.Set(ctx, "empty", "", time.Minute); err != nil {
			t.Fatal(err)
		}
		if got, err := values.Get(ctx, "empty"); err != nil || got != "" {
			t.Fatalf("empty Get() = %q/%v", got, err)
		}
		if _, err := values.Get(ctx, "missing"); !errors.Is(err, cache.ErrCacheMiss) {
			t.Fatalf("missing Get() = %v", err)
		}
	})

	t.Run("namespace-clear-is-paged-and-isolated", func(t *testing.T) {
		first := integrationValueCache(t, client, "clear-first", serialization.StringSerializer{}, ValueConfig{
			RemoteTTL: time.Hour, MaxValueBytes: 32, ClearBatchSize: 2,
		})
		second := integrationValueCache(t, client, "clear-second", serialization.StringSerializer{}, ValueConfig{
			RemoteTTL: time.Hour, MaxValueBytes: 32, ClearBatchSize: 2,
		})
		for i := 0; i < 9; i++ {
			key := string(rune('a' + i))
			if err := first.Set(ctx, key, key, time.Minute); err != nil {
				t.Fatal(err)
			}
		}
		if err := second.Set(ctx, "foreign", "kept", time.Minute); err != nil {
			t.Fatal(err)
		}
		if err := first.Clear(ctx); err != nil {
			t.Fatal(err)
		}
		for i := 0; i < 9; i++ {
			if _, err := first.Get(ctx, string(rune('a'+i))); !errors.Is(err, cache.ErrCacheMiss) {
				t.Fatalf("cleared key %d = %v", i, err)
			}
		}
		if got, err := second.Get(ctx, "foreign"); err != nil || got != "kept" {
			t.Fatalf("foreign Get() = %q/%v", got, err)
		}
	})

	t.Run("pointer-isolation", func(t *testing.T) {
		remote := integrationValueCache(t, client, "pointers", serialization.NewJSONSerializer[*integrationRecord](), ValueConfig{
			RemoteTTL: time.Hour, MaxValueBytes: 128, ClearBatchSize: 2,
		})
		first := mustTieredCache(t, cache.NewMemory[string, *integrationRecord](), remote, nil)
		second := mustTieredCache(t, cache.NewMemory[string, *integrationRecord](), remote, nil)
		written := &integrationRecord{Name: "shared", Count: 1}
		if err := remote.Set(ctx, "item", written, time.Minute); err != nil {
			t.Fatal(err)
		}
		firstValue, err := first.Get(ctx, "item")
		if err != nil {
			t.Fatal(err)
		}
		secondValue, err := second.Get(ctx, "item")
		if err != nil {
			t.Fatal(err)
		}
		if firstValue == written || secondValue == written || firstValue == secondValue || *firstValue != *secondValue {
			t.Fatalf("pointer identities = %p/%p/%p", written, firstValue, secondValue)
		}
	})

	t.Run("known-write-local-ttl-does-not-outlive-l2", func(t *testing.T) {
		remote := integrationValueCache(t, client, "tiered-ttl", serialization.StringSerializer{}, ValueConfig{
			RemoteTTL: time.Hour, MaxValueBytes: 32, ClearBatchSize: 2,
		})
		tiered := mustTieredCache(t, cache.NewMemory[string, string](), remote, nil)
		if err := tiered.Set(ctx, "short", "value", 25*time.Millisecond); err != nil {
			t.Fatal(err)
		}
		bttesting.Eventually(t, 3*time.Second, func() bool {
			_, err := tiered.Get(ctx, "short")
			return errors.Is(err, cache.ErrCacheMiss)
		})
	})

	t.Run("mixed-version-matrix", func(t *testing.T) {
		jsonSerializer := serialization.NewJSONSerializer[integrationRecord]()
		version1, err := serialization.NewVersionedSerializer[integrationRecord](jsonSerializer, 1)
		if err != nil {
			t.Fatal(err)
		}
		version2, err := serialization.NewVersionedSerializer[integrationRecord](jsonSerializer, 2)
		if err != nil {
			t.Fatal(err)
		}
		config := ValueConfig{RemoteTTL: time.Hour, MaxValueBytes: 256, ClearBatchSize: 2}
		oldReader := integrationValueCache(t, client, "versions", version1, config)
		newReader := integrationValueCache(t, client, "versions", version2, config)
		v1 := integrationRecord{Name: "v1"}
		if err := oldReader.Set(ctx, "item", v1, time.Minute); err != nil {
			t.Fatal(err)
		}
		if got, err := newReader.Get(ctx, "item"); err != nil || got != v1 {
			t.Fatalf("v2 reads v1 = %+v/%v", got, err)
		}
		if err := newReader.Set(ctx, "item", integrationRecord{Name: "v2"}, time.Minute); err != nil {
			t.Fatal(err)
		}
		if _, err := oldReader.Get(ctx, "item"); !errors.Is(err, serialization.ErrUnsupportedVersion) || !hasReason(err, ReasonInvalidPayload) {
			t.Fatalf("v1 reads v2 = %v", err)
		}
	})

	t.Run("least-privilege-identities", func(t *testing.T) {
		testRedisValueACLs(t, ctx, addr, client)
	})
}

func integrationValueCache[V any](
	t *testing.T,
	client *redis.Client,
	namespace string,
	serializer serialization.Serializer[V],
	config ValueConfig,
) *ValueCache[V] {
	t.Helper()
	values, err := NewValueCache(ValueOptions[V]{Client: client, Namespace: namespace, Serializer: serializer, Config: &config})
	if err != nil {
		t.Fatal(err)
	}
	return values
}

func integrationPhysicalKey[V any](t *testing.T, values *ValueCache[V], logical string) string {
	t.Helper()
	key, err := values.key(logical)
	if err != nil {
		t.Fatal(err)
	}
	return key.Value
}

func testRedisValueACLs(t *testing.T, ctx context.Context, addr string, admin *redis.Client) {
	t.Helper()
	const ordinaryUser = "redisvalue-ordinary"
	const ordinaryPassword = "ordinary-password"
	const clearUser = "redisvalue-clear"
	const clearPassword = "clear-password"
	const ownPattern = "~bluetape:cache:value:acl-owned:*"
	for _, command := range [][]any{
		{"ACL", "SETUSER", ordinaryUser, "reset", "on", ">" + ordinaryPassword, ownPattern, "+getrange", "+exists", "+set", "+del"},
		{"ACL", "SETUSER", clearUser, "reset", "on", ">" + clearPassword, ownPattern, "+scan", "+unlink"},
	} {
		if err := admin.Do(ctx, command...).Err(); err != nil {
			t.Fatalf("configure ACL: %v", err)
		}
	}
	t.Cleanup(func() {
		if err := admin.Do(context.Background(), "ACL", "DELUSER", ordinaryUser, clearUser).Err(); err != nil {
			t.Errorf("delete ACL users: %v", err)
		}
	})

	ordinary := redis.NewClient(&redis.Options{Addr: addr, Username: ordinaryUser, Password: ordinaryPassword})
	clearAdmin := redis.NewClient(&redis.Options{Addr: addr, Username: clearUser, Password: clearPassword})
	t.Cleanup(func() { _ = ordinary.Close() })
	t.Cleanup(func() { _ = clearAdmin.Close() })
	config := ValueConfig{RemoteTTL: time.Hour, MaxValueBytes: 32, ClearBatchSize: 2}
	values := integrationValueCache(t, ordinary, "acl-owned", serialization.StringSerializer{}, config)
	if err := values.Set(ctx, "item", "value", time.Minute); err != nil {
		t.Fatalf("ordinary Set() = %v", err)
	}
	if got, err := values.Get(ctx, "item"); err != nil || got != "value" {
		t.Fatalf("ordinary Get() = %q/%v", got, err)
	}
	if err := values.Delete(ctx, "item"); err != nil {
		t.Fatalf("ordinary Delete() = %v", err)
	}
	if err := values.Clear(ctx); !hasReason(err, ReasonPartialClear) {
		t.Fatalf("ordinary Clear() = %v", err)
	}

	if err := admin.Set(ctx, "bluetape:cache:value:acl-owned:a", "a", 0).Err(); err != nil {
		t.Fatal(err)
	}
	if err := admin.Set(ctx, "foreign:secret", "secret", 0).Err(); err != nil {
		t.Fatal(err)
	}
	keys, _, err := clearAdmin.Scan(ctx, 0, "*", 100).Result()
	if err != nil || !slices.Contains(keys, "foreign:secret") {
		t.Fatalf("clear-admin SCAN = %v/%v", keys, err)
	}
	for name, command := range map[string]*redis.Cmd{
		"foreign get":    clearAdmin.Do(ctx, "GET", "foreign:secret"),
		"foreign unlink": clearAdmin.Do(ctx, "UNLINK", "foreign:secret"),
		"flushdb":        clearAdmin.Do(ctx, "FLUSHDB"),
		"flushall":       clearAdmin.Do(ctx, "FLUSHALL"),
	} {
		if err := command.Err(); err == nil || !strings.Contains(err.Error(), "NOPERM") {
			t.Fatalf("%s = %v", name, err)
		}
	}
	clearValues := integrationValueCache(t, clearAdmin, "acl-owned", serialization.StringSerializer{}, config)
	if err := clearValues.Clear(ctx); err != nil {
		t.Fatalf("clear-admin Clear() = %v", err)
	}
	if exists, err := admin.Exists(ctx, "bluetape:cache:value:acl-owned:a").Result(); err != nil || exists != 0 {
		t.Fatalf("owned key exists = %d/%v", exists, err)
	}
	if got, err := admin.Get(ctx, "foreign:secret").Result(); err != nil || got != "secret" {
		t.Fatalf("foreign key = %q/%v", got, err)
	}
}
