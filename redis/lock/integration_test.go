package redislock

import (
	"context"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	btredis "github.com/bluetape4k/bluetape-go/redis"
	redistestcontainer "github.com/bluetape4k/bluetape-go/testcontainers/redis"
	"github.com/redis/go-redis/v9"
)

func TestFencedLockAcquireReleaseAndMonotonicFence(t *testing.T) {
	ctx, client := redisLockIntegrationClient(t)
	key := "bluetape:test:redis:fenced-lock:" + strings.ReplaceAll(t.Name(), "/", ":")
	firstLock, err := New(client, Options{Key: key, TTL: 80 * time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	first, err := firstLock.TryAcquire(ctx)
	if err != nil {
		t.Fatalf("first acquire: %v", err)
	}
	if first.FencingToken() == 0 {
		t.Fatal("first fencing token must be positive")
	}

	secondLock, err := New(client, Options{Key: key, TTL: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := secondLock.TryAcquire(ctx); !errors.Is(err, ErrNotAcquired) {
		t.Fatalf("contended acquire = %v, want ErrNotAcquired", err)
	}

	fresh := acquireAfterExpiry(t, secondLock, time.Second)
	if fresh.FencingToken() <= first.FencingToken() {
		t.Fatalf("fencing token did not increase: first=%d fresh=%d", first.FencingToken(), fresh.FencingToken())
	}

	if released, err := first.Release(ctx); err != nil || released {
		t.Fatalf("stale release = %t, %v", released, err)
	}
	if released, err := fresh.Release(ctx); err != nil || !released {
		t.Fatalf("fresh release = %t, %v", released, err)
	}
	if released, err := fresh.Release(ctx); err != nil || released {
		t.Fatalf("idempotent release = %t, %v", released, err)
	}
}

func TestFencedLockAcquireWaitsForContextAndRelease(t *testing.T) {
	ctx, client := redisLockIntegrationClient(t)
	key := "bluetape:test:redis:fenced-lock:wait:" + strings.ReplaceAll(t.Name(), "/", ":")
	firstLock, err := New(client, Options{Key: key, TTL: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	first, err := firstLock.TryAcquire(ctx)
	if err != nil {
		t.Fatal(err)
	}
	secondLock, err := New(client, Options{Key: key, TTL: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	waitCtx, cancel := context.WithTimeout(ctx, 50*time.Millisecond)
	defer cancel()
	if _, err := secondLock.Acquire(waitCtx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Acquire() error = %v, want deadline", err)
	}
	if released, err := first.Release(ctx); err != nil || !released {
		t.Fatalf("release held lease = %t, %v", released, err)
	}
	fresh, err := secondLock.Acquire(ctx)
	if err != nil {
		t.Fatalf("Acquire() after release: %v", err)
	}
	if released, err := fresh.Release(ctx); err != nil || !released {
		t.Fatalf("release fresh lease = %t, %v", released, err)
	}
}

func TestFencedLockClosedClientRedactsMutationErrors(t *testing.T) {
	client := redis.NewClient(&redis.Options{Addr: "127.0.0.1:0"})
	if err := client.Close(); err != nil {
		t.Fatal(err)
	}
	logicalKey := "secret fenced lock logical key"
	lock, err := New(client, Options{Key: logicalKey, TTL: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := lock.TryAcquire(context.Background()); err == nil {
		t.Fatal("closed client acquire should fail")
	} else {
		if !errors.Is(err, redis.ErrClosed) {
			t.Fatalf("closed client acquire = %v, want redis.ErrClosed", err)
		}
		if !errors.Is(err, btredis.ErrCommitUnknown) {
			t.Fatalf("closed client acquire = %v, want ErrCommitUnknown", err)
		}
		if strings.Contains(err.Error(), logicalKey) {
			t.Fatalf("logical key leaked in error: %v", err)
		}
	}

	owner, err := btredis.NewOwnerToken()
	if err != nil {
		t.Fatal(err)
	}
	lease, err := lock.newLease(owner, 1)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := lease.Release(context.Background()); err == nil {
		t.Fatal("closed client release should fail")
	} else {
		if !errors.Is(err, redis.ErrClosed) || !errors.Is(err, btredis.ErrCommitUnknown) {
			t.Fatalf("closed client release = %v, want provider and ambiguity errors", err)
		}
		if strings.Contains(err.Error(), owner.RedisValue()) || strings.Contains(err.Error(), logicalKey) {
			t.Fatalf("secret value leaked in error: %v", err)
		}
	}
}

func TestFencedLockConcurrentTryAcquireMaintainsSingleOwner(t *testing.T) {
	ctx, client := redisLockIntegrationClient(t)
	key := "bluetape:test:redis:fenced-lock:stress:" + strings.ReplaceAll(t.Name(), "/", ":")
	lock, err := New(client, Options{Key: key, TTL: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	const workers = 8
	const rounds = 24
	var active atomic.Int32
	var maxActive atomic.Int32
	var wg sync.WaitGroup
	errorsCh := make(chan error, workers)
	for worker := 0; worker < workers; worker++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for round := 0; round < rounds; round++ {
				lease, err := lock.Acquire(ctx)
				if err != nil {
					errorsCh <- err
					return
				}
				current := active.Add(1)
				for {
					seen := maxActive.Load()
					if current <= seen || maxActive.CompareAndSwap(seen, current) {
						break
					}
				}
				time.Sleep(time.Millisecond)
				active.Add(-1)
				if _, err := lease.Release(ctx); err != nil {
					errorsCh <- err
					return
				}
			}
		}()
	}
	wg.Wait()
	close(errorsCh)
	for err := range errorsCh {
		t.Errorf("concurrent worker: %v", err)
	}
	if got := maxActive.Load(); got > 1 {
		t.Fatalf("max active owners = %d, want <= 1", got)
	}
}

func redisLockIntegrationClient(tb testing.TB) (context.Context, *redis.Client) {
	tb.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	tb.Cleanup(cancel)
	client := redis.NewClient(&redis.Options{Addr: redistestcontainer.Start(ctx, tb)})
	tb.Cleanup(func() {
		_ = client.Close()
	})
	return ctx, client
}

func acquireAfterExpiry(t *testing.T, lock *FencedLock, timeout time.Duration) *Lease {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		lease, err := lock.TryAcquire(context.Background())
		if err == nil {
			return lease
		}
		if !errors.Is(err, ErrNotAcquired) {
			t.Fatalf("acquire after expiry: %v", err)
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("lock did not become available before timeout")
	return nil
}
