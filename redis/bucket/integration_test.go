package redisbucket

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/bluetape4k/bluetape-go/serialization"
	redistestcontainer "github.com/bluetape4k/bluetape-go/testcontainers/redis"
	"github.com/redis/go-redis/v9"
)

func TestBucketRedisIntegrationExpiryAndAtomicOperations(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	t.Cleanup(cancel)
	client := redis.NewClient(&redis.Options{Addr: redistestcontainer.Start(ctx, t)})
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanupCancel()
		_ = client.FlushDB(cleanupCtx).Err()
		_ = client.Close()
	})
	bucket, err := New(client, Options[string]{Namespace: "integration-bucket", Serializer: serialization.StringSerializer{}})
	if err != nil {
		t.Fatal(err)
	}
	key := strings.ReplaceAll(t.Name(), "/", ":")
	if err := bucket.Set(ctx, key, "value", 40*time.Millisecond); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		_, hit, err := bucket.Get(ctx, key)
		if err != nil {
			t.Fatal(err)
		}
		if !hit {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if _, hit, err := bucket.Get(ctx, key); err != nil || hit {
		t.Fatalf("expired Get() = hit=%v err=%v", hit, err)
	}
	if ok, err := bucket.SetIfAbsent(ctx, key, "first", 0); err != nil || !ok {
		t.Fatalf("SetIfAbsent() = %v, %v", ok, err)
	}
	if ok, err := bucket.SetIfAbsent(ctx, key, "second", 0); err != nil || ok {
		t.Fatalf("contended SetIfAbsent() = %v, %v", ok, err)
	}
	if ok, err := bucket.CompareAndSet(ctx, key, "first", "second", 0); err != nil || !ok {
		t.Fatalf("CompareAndSet() = %v, %v", ok, err)
	}
	got, hit, err := bucket.GetAndDelete(ctx, key)
	if err != nil || !hit || got != "second" {
		t.Fatalf("GetAndDelete() = %q, %v, %v", got, hit, err)
	}
}
