package redisfory

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/apache/fory/go/fory"
	"github.com/bluetape4k/bluetape-go/cache"
	redistestcontainer "github.com/bluetape4k/bluetape-go/testcontainers/redis"
	bttesting "github.com/bluetape4k/bluetape-go/testing"
	"github.com/redis/go-redis/v9"
)

type integrationValue struct {
	Name  string
	Count int
}

func registerIntegrationValue(runtime *fory.Fory) error {
	return runtime.RegisterStructByName(integrationValue{}, "redisfory.integrationValue")
}

func TestValueCacheRedisIntegration(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	t.Cleanup(cancel)
	addr := redistestcontainer.Start(ctx, t)
	client := redis.NewClient(&redis.Options{Addr: addr})
	t.Cleanup(func() {
		if err := client.Close(); err != nil {
			t.Errorf("close caller-owned Redis client: %v", err)
		}
	})
	bttesting.Eventually(t, 5*time.Second, func() bool {
		return client.Ping(ctx).Err() == nil
	})

	t.Run("profiles-store-btfv", func(t *testing.T) {
		constructors := []struct {
			name string
			new  func(Options) (*ValueCache[integrationValue], error)
		}{
			{name: "native-fast", new: NewNativeFast[integrationValue]},
			{name: "native-compatible", new: NewNativeCompatible[integrationValue]},
		}
		for _, tc := range constructors {
			t.Run(tc.name, func(t *testing.T) {
				c, err := tc.new(integrationOptions(client, "profile."+tc.name, 1))
				if err != nil {
					t.Fatalf("construct cache: %v", err)
				}
				want := integrationValue{Name: tc.name, Count: 7}
				if err := c.Set(ctx, "item:42", want, time.Minute); err != nil {
					t.Fatalf("set value: %v", err)
				}
				key, err := c.key("item:42")
				if err != nil {
					t.Fatalf("build physical key: %v", err)
				}
				raw, err := client.Get(ctx, key.Value).Bytes()
				if err != nil {
					t.Fatalf("read raw Redis value: %v", err)
				}
				if len(raw) < envelopeHeaderSize || string(raw[:4]) != "BTFV" || raw[0] == '{' || strings.HasPrefix(string(raw), "eyJ") {
					t.Fatalf("raw Redis value is not a BTFV binary envelope: %x", raw)
				}
				got, err := c.Get(ctx, "item:42")
				if err != nil {
					t.Fatalf("get value: %v", err)
				}
				if got != want {
					t.Fatalf("round trip = %+v, want %+v", got, want)
				}
			})
		}
	})

	t.Run("documented-minimum-acl-supports-cache-lifecycle", func(t *testing.T) {
		const username = "redisfory-integration"
		const password = "redisfory-integration-password"
		if err := client.Do(ctx,
			"ACL", "SETUSER", username, "reset", "on", ">"+password,
			"~bluetape:cache:fory:acl:g1:*", "+getrange", "+exists", "+set", "+del",
		).Err(); err != nil {
			t.Fatalf("configure minimum ACL: %v", err)
		}
		t.Cleanup(func() {
			if err := client.Do(context.Background(), "ACL", "DELUSER", username).Err(); err != nil {
				t.Errorf("delete ACL user: %v", err)
			}
		})

		aclClient := redis.NewClient(&redis.Options{Addr: addr, Username: username, Password: password})
		t.Cleanup(func() {
			if err := aclClient.Close(); err != nil {
				t.Errorf("close ACL Redis client: %v", err)
			}
		})
		c := mustIntegrationCache(t, aclClient, "acl", 1)
		want := integrationValue{Name: "least-privilege", Count: 1}
		if err := c.Set(ctx, "item", want, time.Minute); err != nil {
			t.Fatalf("ACL set: %v", err)
		}
		got, err := c.Get(ctx, "item")
		if err != nil || got != want {
			t.Fatalf("ACL get = %+v, %v", got, err)
		}
		if _, err := c.Get(ctx, "missing"); !errors.Is(err, cache.ErrCacheMiss) {
			t.Fatalf("ACL miss = %v", err)
		}
		if err := c.Delete(ctx, "item"); err != nil {
			t.Fatalf("ACL delete: %v", err)
		}
	})

	t.Run("ttl-miss-and-idempotent-delete", func(t *testing.T) {
		c := mustIntegrationCache(t, client, "lifecycle", 1)
		if _, err := c.Get(ctx, "missing"); !errors.Is(err, cache.ErrCacheMiss) {
			t.Fatalf("explicit miss = %v", err)
		}
		if err := c.Delete(ctx, "missing"); err != nil {
			t.Fatalf("idempotent delete: %v", err)
		}
		if err := c.Set(ctx, "short", integrationValue{Name: "ttl"}, 20*time.Millisecond); err != nil {
			t.Fatalf("set expiring value: %v", err)
		}
		deadline := time.Now().Add(3 * time.Second)
		for {
			_, err := c.Get(ctx, "short")
			if errors.Is(err, cache.ErrCacheMiss) {
				break
			}
			if err != nil {
				t.Fatalf("wait for TTL expiry: %v", err)
			}
			if time.Now().After(deadline) {
				t.Fatal("wait for TTL expiry: value remained present")
			}
			time.Sleep(10 * time.Millisecond)
		}
	})

	t.Run("oversized-value-is-bounded-before-decode", func(t *testing.T) {
		options := integrationOptions(client, "oversized", 1)
		options.MaxPayloadBytes = 16
		c, err := NewNativeFast[integrationValue](options)
		if err != nil {
			t.Fatalf("construct bounded cache: %v", err)
		}
		key, err := c.key("large")
		if err != nil {
			t.Fatalf("build physical key: %v", err)
		}
		if err := client.Set(ctx, key.Value, []byte{}, time.Minute).Err(); err != nil {
			t.Fatalf("seed empty corrupt value: %v", err)
		}
		_, err = c.Get(ctx, "large")
		assertCacheReason(t, err, ReasonInvalidMagic)
		if err := client.Set(ctx, key.Value, make([]byte, envelopeHeaderSize+17), time.Minute).Err(); err != nil {
			t.Fatalf("seed oversized value: %v", err)
		}
		_, err = c.Get(ctx, "large")
		assertCacheReason(t, err, ReasonPayloadTooLarge)
	})

	t.Run("schema-generations-are-key-isolated", func(t *testing.T) {
		generation1 := mustIntegrationCache(t, client, "generation", 1)
		generation2 := mustIntegrationCache(t, client, "generation", 2)
		if err := generation1.Set(ctx, "shared", integrationValue{Name: "v1"}, time.Minute); err != nil {
			t.Fatalf("set generation 1: %v", err)
		}
		if _, err := generation2.Get(ctx, "shared"); !errors.Is(err, cache.ErrCacheMiss) {
			t.Fatalf("generation 2 isolation = %v", err)
		}
		key1, _ := generation1.key("shared")
		key2, _ := generation2.key("shared")
		if key1.Value == key2.Value || !strings.Contains(key1.Value, ":g1:") || !strings.Contains(key2.Value, ":g2:") {
			t.Fatalf("generation keys = %q / %q", key1.Value, key2.Value)
		}
	})

	t.Run("command-failure-is-redacted", func(t *testing.T) {
		failedClient := redis.NewClient(&redis.Options{Addr: addr})
		c := mustIntegrationCache(t, failedClient, "failed-client", 1)
		if err := failedClient.Close(); err != nil {
			t.Fatalf("close failure client: %v", err)
		}
		_, err := c.Get(ctx, "private-logical-key")
		if err == nil || !errors.Is(err, errProviderFailed) {
			t.Fatalf("closed client error = %v", err)
		}
		assertErrorRedacted(t, err, "redis: client is closed", "private-logical-key", addr)
	})

	t.Run("canceled-after-serialization-leaves-no-write", func(t *testing.T) {
		writeCtx, cancelWrite := context.WithCancel(ctx)
		codec := fakeValueCodec[integrationValue]{
			serialize: func(integrationValue) ([]byte, error) {
				cancelWrite()
				return []byte("encoded"), nil
			},
			deserialize: func([]byte) (integrationValue, error) { return integrationValue{}, nil },
		}
		c := unitCache[integrationValue](client, codec)
		key, err := c.key("late-write")
		if err != nil {
			t.Fatalf("build physical key: %v", err)
		}
		if err := client.Del(ctx, key.Value).Err(); err != nil {
			t.Fatalf("remove preexisting key: %v", err)
		}
		err = c.Set(writeCtx, "late-write", integrationValue{Name: "cancel"}, time.Minute)
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("set after cancellation = %v", err)
		}
		exists, err := client.Exists(ctx, key.Value).Result()
		if err != nil {
			t.Fatalf("check canceled write: %v", err)
		}
		if exists != 0 {
			t.Fatalf("canceled write created %d key", exists)
		}
	})

	t.Run("concurrent-round-trips", func(t *testing.T) {
		c := mustIntegrationCache(t, client, "concurrent", 1)
		const workers = 16
		const rounds = 100
		var operations atomic.Int64
		var misses atomic.Int64
		var failures atomic.Int64
		var wg sync.WaitGroup
		wg.Add(workers)
		for worker := 0; worker < workers; worker++ {
			go func(worker int) {
				defer wg.Done()
				for round := 0; round < rounds; round++ {
					key := fmt.Sprintf("worker:%d:round:%d", worker, round)
					want := integrationValue{Name: key, Count: round}
					if err := c.Set(ctx, key, want, time.Minute); err != nil {
						failures.Add(1)
						continue
					}
					operations.Add(1)
					got, err := c.Get(ctx, key)
					if err != nil || got != want {
						failures.Add(1)
						continue
					}
					operations.Add(1)
					if err := c.Delete(ctx, key); err != nil {
						failures.Add(1)
						continue
					}
					operations.Add(1)
					if _, err := c.Get(ctx, key); !errors.Is(err, cache.ErrCacheMiss) {
						failures.Add(1)
						continue
					}
					operations.Add(1)
					misses.Add(1)
				}
			}(worker)
		}
		wg.Wait()
		if failures.Load() != 0 || operations.Load() != workers*rounds*4 || misses.Load() != workers*rounds {
			t.Fatalf("operations/misses/failures = %d/%d/%d", operations.Load(), misses.Load(), failures.Load())
		}
	})

	if err := client.Ping(ctx).Err(); err != nil {
		t.Fatalf("caller-owned client was closed by cache: %v", err)
	}
}

func integrationOptions(client redis.Cmdable, namespace string, generation uint32) Options {
	return Options{
		Client:           client,
		Namespace:        namespace,
		SchemaGeneration: generation,
		Register:         registerIntegrationValue,
	}
}

func mustIntegrationCache(t *testing.T, client redis.Cmdable, namespace string, generation uint32) *ValueCache[integrationValue] {
	t.Helper()
	c, err := NewNativeFast[integrationValue](integrationOptions(client, namespace, generation))
	if err != nil {
		t.Fatalf("construct integration cache: %v", err)
	}
	return c
}
