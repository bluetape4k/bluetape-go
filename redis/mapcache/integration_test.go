package redismap

import (
	"context"
	"strings"
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
}
