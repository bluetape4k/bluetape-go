package redissem

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

func TestSemaphorePermitAccountingAndIdempotentRelease(t *testing.T) {
	ctx, client := redisSemaphoreIntegrationClient(t)
	key := "bluetape:test:redis:semaphore:" + strings.ReplaceAll(t.Name(), "/", ":")
	semaphore, err := New(client, Options{Key: key, Permits: 2, TTL: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	first, err := semaphore.TryAcquire(ctx)
	if err != nil {
		t.Fatalf("first acquire: %v", err)
	}
	second, err := semaphore.TryAcquire(ctx)
	if err != nil {
		t.Fatalf("second acquire: %v", err)
	}
	if _, err := semaphore.TryAcquire(ctx); !errors.Is(err, ErrNotAcquired) {
		t.Fatalf("third acquire = %v, want ErrNotAcquired", err)
	}
	if released, err := first.Release(ctx); err != nil || !released {
		t.Fatalf("first release = %t, %v", released, err)
	}
	if released, err := first.Release(ctx); err != nil || released {
		t.Fatalf("idempotent first release = %t, %v", released, err)
	}
	third, err := semaphore.TryAcquire(ctx)
	if err != nil {
		t.Fatalf("third acquire after release: %v", err)
	}
	if released, err := second.Release(ctx); err != nil || !released {
		t.Fatalf("second release = %t, %v", released, err)
	}
	if released, err := third.Release(ctx); err != nil || !released {
		t.Fatalf("third release = %t, %v", released, err)
	}
}

func TestSemaphoreExpiredPermitIsCleanedByNextAcquire(t *testing.T) {
	ctx, client := redisSemaphoreIntegrationClient(t)
	key := "bluetape:test:redis:semaphore:expiry:" + strings.ReplaceAll(t.Name(), "/", ":")
	semaphore, err := New(client, Options{Key: key, Permits: 1, TTL: 50 * time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	stale, err := semaphore.TryAcquire(ctx)
	if err != nil {
		t.Fatal(err)
	}
	fresh := acquireSemaphoreAfterExpiry(t, semaphore, time.Second)
	if released, err := stale.Release(ctx); err != nil || released {
		t.Fatalf("stale release = %t, %v", released, err)
	}
	if released, err := fresh.Release(ctx); err != nil || !released {
		t.Fatalf("fresh release = %t, %v", released, err)
	}
}

func TestSemaphoreAcquirePreservesDeadlineWithoutLeakingPermit(t *testing.T) {
	ctx, client := redisSemaphoreIntegrationClient(t)
	key := "bluetape:test:redis:semaphore:wait:" + strings.ReplaceAll(t.Name(), "/", ":")
	semaphore, err := New(client, Options{Key: key, Permits: 1, TTL: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	held, err := semaphore.TryAcquire(ctx)
	if err != nil {
		t.Fatal(err)
	}
	waitCtx, cancel := context.WithTimeout(ctx, 45*time.Millisecond)
	defer cancel()
	if _, err := semaphore.Acquire(waitCtx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Acquire() error = %v, want deadline", err)
	}
	if _, err := semaphore.TryAcquire(ctx); !errors.Is(err, ErrNotAcquired) {
		t.Fatalf("canceled waiter changed permit count: %v", err)
	}
	if released, err := held.Release(ctx); err != nil || !released {
		t.Fatalf("release held permit = %t, %v", released, err)
	}
	fresh, err := semaphore.TryAcquire(ctx)
	if err != nil {
		t.Fatalf("acquire after cleanup: %v", err)
	}
	if _, err := fresh.Release(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestSemaphoreClosedClientRedactsMutationErrors(t *testing.T) {
	client := redis.NewClient(&redis.Options{Addr: "127.0.0.1:0"})
	if err := client.Close(); err != nil {
		t.Fatal(err)
	}
	logicalKey := "secret semaphore logical key"
	semaphore, err := New(client, Options{Key: logicalKey, Permits: 1, TTL: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := semaphore.TryAcquire(context.Background()); err == nil {
		t.Fatal("closed client acquire should fail")
	} else {
		if !errors.Is(err, redis.ErrClosed) || !errors.Is(err, btredis.ErrCommitUnknown) {
			t.Fatalf("closed client acquire = %v, want provider and ambiguity errors", err)
		}
		if strings.Contains(err.Error(), logicalKey) {
			t.Fatalf("logical key leaked in error: %v", err)
		}
	}

	owner, err := btredis.NewOwnerToken()
	if err != nil {
		t.Fatal(err)
	}
	lease := &Lease{semaphore: semaphore, key: logicalKey, owner: owner}
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

func TestSemaphoreConcurrentAcquireNeverExceedsCapacity(t *testing.T) {
	ctx, client := redisSemaphoreIntegrationClient(t)
	key := "bluetape:test:redis:semaphore:stress:" + strings.ReplaceAll(t.Name(), "/", ":")
	const permits = 3
	semaphore, err := New(client, Options{Key: key, Permits: permits, TTL: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	const workers = 16
	const rounds = 32
	var active atomic.Int32
	var maxActive atomic.Int32
	var wg sync.WaitGroup
	errorsCh := make(chan error, workers)
	for worker := 0; worker < workers; worker++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for round := 0; round < rounds; round++ {
				lease, err := semaphore.Acquire(ctx)
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
	if got := maxActive.Load(); got > permits {
		t.Fatalf("max active permits = %d, want <= %d", got, permits)
	}
}

func redisSemaphoreIntegrationClient(tb testing.TB) (context.Context, *redis.Client) {
	tb.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	tb.Cleanup(cancel)
	client := redis.NewClient(&redis.Options{Addr: redistestcontainer.Start(ctx, tb)})
	tb.Cleanup(func() {
		_ = client.Close()
	})
	return ctx, client
}

func acquireSemaphoreAfterExpiry(t *testing.T, semaphore *Semaphore, timeout time.Duration) *Lease {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		lease, err := semaphore.TryAcquire(context.Background())
		if err == nil {
			return lease
		}
		if !errors.Is(err, ErrNotAcquired) {
			t.Fatalf("acquire after expiry: %v", err)
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("semaphore permit did not expire before timeout")
	return nil
}
