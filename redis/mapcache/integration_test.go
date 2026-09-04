package redismap

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/bluetape4k/bluetape-go/serialization"
	redistestcontainer "github.com/bluetape4k/bluetape-go/testcontainers/redis"
	"github.com/redis/go-redis/v9"
)

func TestMapCacheRedisIntegrationIndependentEntryExpiry(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	t.Cleanup(cancel)
	client := redis.NewClient(&redis.Options{Addr: redistestcontainer.Start(ctx, t)})
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanupCancel()
		_ = client.FlushDB(cleanupCtx).Err()
		_ = client.Close()
	})
	cache, err := New(client, Options[string]{Namespace: "integration-map", HashTag: "integration", Serializer: serialization.StringSerializer{}})
	if err != nil {
		t.Fatal(err)
	}
	prefix := strings.ReplaceAll(t.Name(), "/", ":")
	shortKey, longKey := prefix+":short", prefix+":long"
	if err := cache.Set(ctx, shortKey, "short", 40*time.Millisecond); err != nil {
		t.Fatal(err)
	}
	if err := cache.Set(ctx, longKey, "long", time.Second); err != nil {
		t.Fatal(err)
	}
	time.Sleep(100 * time.Millisecond)
	if _, hit, err := cache.Get(ctx, shortKey); err != nil || hit {
		t.Fatalf("short entry after expiry = hit=%v err=%v", hit, err)
	}
	if got, hit, err := cache.Get(ctx, longKey); err != nil || !hit || got != "long" {
		t.Fatalf("long entry after short expiry = %q hit=%v err=%v", got, hit, err)
	}
	if ok, err := cache.CompareAndSet(ctx, longKey, "long", "updated", 0); err != nil || !ok {
		t.Fatalf("CompareAndSet() = %v, %v", ok, err)
	}
	if got, hit, err := cache.GetAndDelete(ctx, longKey); err != nil || !hit || got != "updated" {
		t.Fatalf("GetAndDelete() = %q, %v, %v", got, hit, err)
	}

	bounded, err := New(client, Options[string]{
		Namespace:       "integration-map-bounded",
		HashTag:         "integration",
		Serializer:      serialization.StringSerializer{},
		MaxPayloadBytes: 4,
	})
	if err != nil {
		t.Fatal(err)
	}
	overSizedKey := "legacy-oversized"
	physicalKey := "integration-map-bounded:map:{integration}:" + overSizedKey
	if err := client.Set(ctx, physicalKey, "12345", 0).Err(); err != nil {
		t.Fatal(err)
	}
	if _, _, err := bounded.Get(ctx, overSizedKey); !errors.Is(err, ErrPayloadTooLarge) {
		t.Fatalf("oversized Get() = %v, want ErrPayloadTooLarge", err)
	}
	if ok, err := bounded.CompareAndSet(ctx, overSizedKey, "1234", "next", 0); !errors.Is(err, ErrPayloadTooLarge) || ok {
		t.Fatalf("oversized CompareAndSet() = %v, %v", ok, err)
	}
	if _, _, err := bounded.GetAndDelete(ctx, overSizedKey); !errors.Is(err, ErrPayloadTooLarge) {
		t.Fatalf("oversized GetAndDelete() = %v, want ErrPayloadTooLarge", err)
	}
	if exists := client.Exists(ctx, physicalKey).Val(); exists != 1 {
		t.Fatalf("oversized key existence = %d, want retained", exists)
	}
	if err := bounded.Set(ctx, "empty", "", 0); err != nil {
		t.Fatal(err)
	}
	if got, hit, err := bounded.Get(ctx, "empty"); err != nil || !hit || got != "" {
		t.Fatalf("empty Get() = %q, %v, %v", got, hit, err)
	}

	concurrentKey := "concurrent-cas"
	if err := cache.Set(ctx, concurrentKey, "expected", 0); err != nil {
		t.Fatal(err)
	}
	const workers = 16
	results := make(chan bool, workers)
	var wg sync.WaitGroup
	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ok, err := cache.CompareAndSet(ctx, concurrentKey, "expected", "winner", 0)
			if err != nil {
				t.Errorf("concurrent CompareAndSet() = %v", err)
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
		t.Fatalf("Redis CAS winners = %d, want 1", winners)
	}
}
