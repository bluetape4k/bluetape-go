package btredis

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	redistestcontainer "github.com/bluetape4k/bluetape-go/testcontainers/redis"
	redis "github.com/redis/go-redis/v9"
)

func TestCompareAndDeleteRemovesOnlyMatchingOwner(t *testing.T) {
	ctx, client, key := redisIntegrationClient(t)
	lease := redisLeaseForKey(t, key)
	setLeaseValue(ctx, t, client, lease, time.Minute)

	ok, err := CompareAndDelete(ctx, client, lease, "redis test")
	if err != nil {
		t.Fatalf("CompareAndDelete() error = %v", err)
	}
	if !ok {
		t.Fatal("CompareAndDelete() = false, want true")
	}
	if exists := client.Exists(ctx, key).Val(); exists != 0 {
		t.Fatalf("key exists = %d, want 0", exists)
	}
}

func TestCompareAndDeleteDoesNotRemoveLaterOwner(t *testing.T) {
	ctx, client, key := redisIntegrationClient(t)
	stale := redisLeaseForKey(t, key)
	later := redisLeaseForKey(t, key)
	setLeaseValue(ctx, t, client, later, time.Minute)

	ok, err := CompareAndDelete(ctx, client, stale, "redis test")
	if err != nil {
		t.Fatalf("CompareAndDelete() error = %v", err)
	}
	if ok {
		t.Fatal("CompareAndDelete() = true, want false")
	}
	if got := client.Get(ctx, key).Val(); got != later.Token().RedisValue() {
		t.Fatal("CompareAndDelete() removed or changed later owner")
	}
}

func TestCompareAndExtendUpdatesOnlyMatchingOwner(t *testing.T) {
	ctx, client, key := redisIntegrationClient(t)
	lease := redisLeaseForKey(t, key)
	setLeaseValue(ctx, t, client, lease, 500*time.Millisecond)

	ok, err := CompareAndExtend(ctx, client, lease, 5*time.Second, "redis test")
	if err != nil {
		t.Fatalf("CompareAndExtend() error = %v", err)
	}
	if !ok {
		t.Fatal("CompareAndExtend() = false, want true")
	}
	if ttl := client.PTTL(ctx, key).Val(); ttl < 3*time.Second {
		t.Fatalf("PTTL() = %s, want extended TTL", ttl)
	}
}

func TestCompareAndExtendDoesNotExtendLaterOwner(t *testing.T) {
	ctx, client, key := redisIntegrationClient(t)
	stale := redisLeaseForKey(t, key)
	later := redisLeaseForKey(t, key)
	setLeaseValue(ctx, t, client, later, time.Second)

	before := client.PTTL(ctx, key).Val()
	ok, err := CompareAndExtend(ctx, client, stale, 5*time.Second, "redis test")
	if err != nil {
		t.Fatalf("CompareAndExtend() error = %v", err)
	}
	if ok {
		t.Fatal("CompareAndExtend() = true, want false")
	}
	if got := client.Get(ctx, key).Val(); got != later.Token().RedisValue() {
		t.Fatal("CompareAndExtend() changed later owner")
	}
	if ttl := client.PTTL(ctx, key).Val(); ttl > before+500*time.Millisecond {
		t.Fatalf("PTTL() = %s, before = %s; stale owner extended TTL", ttl, before)
	}
}

func TestCompareAndDeleteCanceledContextReturnsContextError(t *testing.T) {
	_, client, key := redisIntegrationClient(t)
	lease := redisLeaseForKey(t, key)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	ok, err := CompareAndDelete(ctx, client, lease, "redis test")
	if ok {
		t.Fatal("CompareAndDelete() = true, want false")
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("CompareAndDelete() error = %v, want context.Canceled", err)
	}
}

func TestCompareAndExtendDeadlineReturnsContextError(t *testing.T) {
	_, client, key := redisIntegrationClient(t)
	lease := redisLeaseForKey(t, key)
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer cancel()

	ok, err := CompareAndExtend(ctx, client, lease, time.Second, "redis test")
	if ok {
		t.Fatal("CompareAndExtend() = true, want false")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("CompareAndExtend() error = %v, want context.DeadlineExceeded", err)
	}
}

func TestCompareAndDeleteFirstRunUsesGoRedisScriptFallback(t *testing.T) {
	ctx, client, key := redisIntegrationClient(t)
	lease := redisLeaseForKey(t, key)
	setLeaseValue(ctx, t, client, lease, time.Minute)

	ok, err := CompareAndDelete(ctx, client, lease, "redis test")
	if err != nil {
		t.Fatalf("CompareAndDelete() error = %v", err)
	}
	if !ok {
		t.Fatal("CompareAndDelete() = false, want true")
	}
}

func TestCompareAndDeleteInterleavedOwnersStress(t *testing.T) {
	ctx, client, key := redisIntegrationClient(t)
	const workers = 8
	const iterations = 32

	for i := range iterations {
		later := redisLeaseForKey(t, key)
		setLeaseValue(ctx, t, client, later, time.Minute)

		var wg sync.WaitGroup
		for worker := range workers {
			wg.Add(1)
			go func(worker int) {
				defer wg.Done()
				stale := redisLeaseForKey(t, key)
				ok, err := CompareAndDelete(ctx, client, stale, "redis test")
				if err != nil {
					t.Errorf("worker %d iteration %d: CompareAndDelete error = %v", worker, i, err)
				}
				if ok {
					t.Errorf("worker %d iteration %d: stale owner deleted key", worker, i)
				}
			}(worker)
		}
		wg.Wait()

		if got := client.Get(ctx, key).Val(); got != later.Token().RedisValue() {
			t.Fatalf("iteration %d: later owner changed", i)
		}
	}
}

func TestCompareAndExtendInterleavedOwnersStress(t *testing.T) {
	ctx, client, key := redisIntegrationClient(t)
	const workers = 8
	const iterations = 32

	for i := range iterations {
		later := redisLeaseForKey(t, key)
		setLeaseValue(ctx, t, client, later, time.Second)
		before := client.PTTL(ctx, key).Val()

		var wg sync.WaitGroup
		for worker := range workers {
			wg.Add(1)
			go func(worker int) {
				defer wg.Done()
				stale := redisLeaseForKey(t, key)
				ok, err := CompareAndExtend(ctx, client, stale, 5*time.Second, "redis test")
				if err != nil {
					t.Errorf("worker %d iteration %d: CompareAndExtend error = %v", worker, i, err)
				}
				if ok {
					t.Errorf("worker %d iteration %d: stale owner extended key", worker, i)
				}
			}(worker)
		}
		wg.Wait()

		if got := client.Get(ctx, key).Val(); got != later.Token().RedisValue() {
			t.Fatalf("iteration %d: later owner changed", i)
		}
		if ttl := client.PTTL(ctx, key).Val(); ttl > before+500*time.Millisecond {
			t.Fatalf("iteration %d: stale owner increased TTL from %s to %s", i, before, ttl)
		}
	}
}

func redisIntegrationClient(tb testing.TB) (context.Context, *redis.Client, string) {
	tb.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	tb.Cleanup(cancel)

	client := redis.NewClient(&redis.Options{Addr: redistestcontainer.Start(ctx, tb)})
	key := "bluetape:redis:test:" + strings.ReplaceAll(tb.Name(), "/", ":")
	tb.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanupCancel()
		_ = client.Del(cleanupCtx, key).Err()
		_ = client.Close()
	})
	return ctx, client, key
}

func redisLeaseForKey(tb testing.TB, key string) Lease {
	tb.Helper()
	token, err := NewOwnerToken()
	if err != nil {
		tb.Fatalf("NewOwnerToken() error = %v", err)
	}
	lease, err := NewLease(key, token)
	if err != nil {
		tb.Fatalf("NewLease(%s) error = %v", RedactedKeyID(key), err)
	}
	return lease
}

func setLeaseValue(ctx context.Context, tb testing.TB, client redis.Cmdable, lease Lease, ttl time.Duration) {
	tb.Helper()
	if err := client.Set(ctx, lease.Key(), lease.Token().RedisValue(), ttl).Err(); err != nil {
		tb.Fatalf("Set(%s) error = %v", lease.RedactedKeyID(), err)
	}
}

func TestCompareAndDeleteIntegrationNames(t *testing.T) {
	if strings.Contains(fmt.Sprint(RedactedKeyID(t.Name())), t.Name()) {
		t.Fatal("redacted key id leaked test name")
	}
}
