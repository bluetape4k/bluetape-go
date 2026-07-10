package redislock_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	redislock "github.com/bluetape4k/bluetape-go/lock/redis"
	btredis "github.com/bluetape4k/bluetape-go/redis"
	redistestcontainer "github.com/bluetape4k/bluetape-go/testcontainers/redis"
	bttesting "github.com/bluetape4k/bluetape-go/testing"
	concurrencytest "github.com/bluetape4k/bluetape-go/testing/concurrency"
	"github.com/redis/go-redis/v9"
)

func TestNewRejectsInvalidOptions(t *testing.T) {
	client := redis.NewClient(&redis.Options{Addr: "127.0.0.1:0"})
	t.Cleanup(func() {
		_ = client.Close()
	})

	tests := []struct {
		name   string
		client redis.Cmdable
		opts   redislock.Options
	}{
		{name: "missing client", client: nil, opts: redislock.Options{Key: "k", TTL: time.Second}},
		{name: "missing key", client: client, opts: redislock.Options{TTL: time.Second}},
		{name: "blank key", client: client, opts: redislock.Options{Key: "  ", TTL: time.Second}},
		{name: "zero ttl", client: client, opts: redislock.Options{Key: "k"}},
		{name: "negative ttl", client: client, opts: redislock.Options{Key: "k", TTL: -time.Second}},
		{name: "blank token", client: client, opts: redislock.Options{Key: "k", TTL: time.Second, Token: "  "}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := redislock.New(tt.client, tt.opts); err == nil {
				t.Fatal("invalid options should fail")
			}
		})
	}
}

func TestNewPreservesRedisKeyVerbatim(t *testing.T) {
	client := redis.NewClient(&redis.Options{Addr: "127.0.0.1:0"})
	t.Cleanup(func() {
		_ = client.Close()
	})

	mutex, err := redislock.New(client, redislock.Options{
		Key: " locks:billing-rollup ",
		TTL: time.Second,
	})
	if err != nil {
		t.Fatalf("new mutex: %v", err)
	}
	if mutex.Key() != " locks:billing-rollup " {
		t.Fatalf("key should be preserved verbatim, got %q", mutex.Key())
	}
}

func TestMutexAcquiresAndUnlocksOwner(t *testing.T) {
	ctx := context.Background()
	client := redisClient(ctx, t)
	key := testLockKey(t)

	mutex, err := redislock.New(client, redislock.Options{
		Key:   key,
		TTL:   2 * time.Second,
		Token: "owner-a",
	})
	if err != nil {
		t.Fatalf("new mutex: %v", err)
	}

	lease, err := mutex.TryLock(ctx)
	if err != nil {
		t.Fatalf("try lock: %v", err)
	}
	if lease.Key() != key || lease.Token() != "owner-a" {
		t.Fatalf("lease identity mismatch: key=%q token=%q", lease.Key(), lease.Token())
	}
	value, err := client.Get(ctx, key).Result()
	if err != nil {
		t.Fatalf("read lock key: %v", err)
	}
	if value != lease.Token() {
		t.Fatalf("redis token = %q, want %q", value, lease.Token())
	}
	ttl, err := client.PTTL(ctx, key).Result()
	if err != nil {
		t.Fatalf("read lock ttl: %v", err)
	}
	if ttl <= 0 {
		t.Fatalf("ttl should be positive, got %s", ttl)
	}

	released, err := lease.Unlock(ctx)
	if err != nil {
		t.Fatalf("unlock: %v", err)
	}
	if !released {
		t.Fatal("owner unlock should release key")
	}
	assertRedisKeyMissing(t, client, key)
}

func TestMutexGeneratedTokenUsesSharedOwnerToken(t *testing.T) {
	ctx := context.Background()
	client := redisClient(ctx, t)
	mutex := newMutex(t, client, testLockKey(t), "", time.Second)

	lease, err := mutex.TryLock(ctx)
	if err != nil {
		t.Fatalf("try lock: %v", err)
	}
	if _, err := btredis.ParseOwnerToken(lease.Token()); err != nil {
		t.Fatalf("generated token should be a shared owner token: %v", err)
	}

	released, err := lease.Unlock(ctx)
	if err != nil || !released {
		t.Fatalf("unlock generated lease: released=%t err=%v", released, err)
	}
}

func TestMutexCustomTokenPreservesLegacyNormalization(t *testing.T) {
	ctx := context.Background()
	client := redisClient(ctx, t)
	key := testLockKey(t)
	mutex := newMutex(t, client, key, " owner-a ", time.Second)

	lease, err := mutex.TryLock(ctx)
	if err != nil {
		t.Fatalf("try lock: %v", err)
	}
	if lease.Token() != "owner-a" {
		t.Fatalf("custom token = %q, want owner-a", lease.Token())
	}
	value, err := client.Get(ctx, key).Result()
	if err != nil {
		t.Fatalf("read lock token: %v", err)
	}
	if value != lease.Token() {
		t.Fatalf("redis custom token = %q, want %q", value, lease.Token())
	}

	released, err := lease.Unlock(ctx)
	if err != nil || !released {
		t.Fatalf("unlock custom lease: released=%t err=%v", released, err)
	}
}

func TestMutexRedactsRedisProviderErrors(t *testing.T) {
	ctx := context.Background()
	key := "secret-lock-key"
	token := "secret-custom-token"

	closed := redis.NewClient(&redis.Options{Addr: "127.0.0.1:0"})
	if err := closed.Close(); err != nil {
		t.Fatalf("close acquire client: %v", err)
	}
	mutex := newMutex(t, closed, key, token, time.Second)
	if _, err := mutex.TryLock(ctx); err == nil {
		t.Fatal("closed client acquire should fail")
	} else {
		assertRedactedRedisOpError(t, err, key, token)
	}

	client := redisClient(ctx, t)
	lease, err := newMutex(t, client, key, token, time.Second).TryLock(ctx)
	if err != nil {
		t.Fatalf("acquire lease for unlock failure: %v", err)
	}
	if err := client.Close(); err != nil {
		t.Fatalf("close unlock client: %v", err)
	}
	if _, err := lease.Unlock(ctx); err == nil {
		t.Fatal("closed client unlock should fail")
	} else {
		assertRedactedRedisOpError(t, err, key, token)
	}
}

func TestMutexRejectsContention(t *testing.T) {
	ctx := context.Background()
	client := redisClient(ctx, t)
	key := testLockKey(t)

	first := newMutex(t, client, key, "owner-a", time.Second)
	second := newMutex(t, client, key, "owner-b", time.Second)

	lease, err := first.TryLock(ctx)
	if err != nil {
		t.Fatalf("first lock: %v", err)
	}
	t.Cleanup(func() {
		_, _ = lease.Unlock(context.Background())
	})

	if _, err := second.TryLock(ctx); !errors.Is(err, redislock.ErrNotAcquired) {
		t.Fatalf("second lock should fail with ErrNotAcquired, got %v", err)
	}
}

func TestLeaseUnlockDoesNotDeleteDifferentOwner(t *testing.T) {
	ctx := context.Background()
	client := redisClient(ctx, t)
	key := testLockKey(t)

	first := newMutex(t, client, key, "owner-a", 50*time.Millisecond)
	stale, err := first.TryLock(ctx)
	if err != nil {
		t.Fatalf("first lock: %v", err)
	}

	bttesting.Eventually(t, time.Second, func() bool {
		return client.Exists(ctx, key).Val() == 0
	})

	second := newMutex(t, client, key, "owner-b", time.Second)
	fresh, err := second.TryLock(ctx)
	if err != nil {
		t.Fatalf("second lock after expiry: %v", err)
	}
	t.Cleanup(func() {
		_, _ = fresh.Unlock(context.Background())
	})

	released, err := stale.Unlock(ctx)
	if err != nil {
		t.Fatalf("stale unlock: %v", err)
	}
	if released {
		t.Fatal("stale owner should not release current owner")
	}

	value, err := client.Get(ctx, key).Result()
	if err != nil {
		t.Fatalf("read current owner: %v", err)
	}
	if value != "owner-b" {
		t.Fatalf("current owner should remain owner-b, got %q", value)
	}
}

func TestExpiredLockCanBeAcquiredByAnotherOwner(t *testing.T) {
	ctx := context.Background()
	client := redisClient(ctx, t)
	key := testLockKey(t)

	first := newMutex(t, client, key, "owner-a", 50*time.Millisecond)
	if _, err := first.TryLock(ctx); err != nil {
		t.Fatalf("first lock: %v", err)
	}

	second := newMutex(t, client, key, "owner-b", time.Second)
	var acquired *redislock.Lease
	bttesting.Eventually(t, time.Second, func() bool {
		lease, err := second.TryLock(ctx)
		if err != nil {
			return false
		}
		acquired = lease
		return true
	})
	t.Cleanup(func() {
		if acquired != nil {
			_, _ = acquired.Unlock(context.Background())
		}
	})
}

func TestMutexPreservesCanceledContext(t *testing.T) {
	ctx := context.Background()
	client := redisClient(ctx, t)
	key := testLockKey(t)
	mutex := newMutex(t, client, key, "owner-a", time.Second)

	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := mutex.TryLock(cancelled); !errors.Is(err, context.Canceled) {
		t.Fatalf("try lock should preserve context.Canceled, got %v", err)
	}

	lease, err := mutex.TryLock(ctx)
	if err != nil {
		t.Fatalf("try lock for unlock cancellation: %v", err)
	}
	t.Cleanup(func() {
		_, _ = lease.Unlock(context.Background())
	})
	if released, err := lease.Unlock(cancelled); !errors.Is(err, context.Canceled) || released {
		t.Fatalf("unlock should preserve context.Canceled without release, released=%t err=%v", released, err)
	}
}

func TestMutexSameKeyContentionStress(t *testing.T) {
	ctx := context.Background()
	client := redisClient(ctx, t)
	key := testLockKey(t)
	var sequence int64
	var active int32
	var winners int32

	task := func(ctx context.Context) error {
		id := atomic.AddInt64(&sequence, 1)
		mutex, err := redislock.New(client, redislock.Options{
			Key:   key,
			TTL:   500 * time.Millisecond,
			Token: fmt.Sprintf("owner-%d", id),
		})
		if err != nil {
			return err
		}
		lease, err := mutex.TryLock(ctx)
		if errors.Is(err, redislock.ErrNotAcquired) {
			return nil
		}
		if err != nil {
			return err
		}
		atomic.AddInt32(&winners, 1)
		if current := atomic.AddInt32(&active, 1); current != 1 {
			atomic.AddInt32(&active, -1)
			_, _ = lease.Unlock(context.Background())
			return fmt.Errorf("multiple active lock owners: %d", current)
		}
		time.Sleep(2 * time.Millisecond)
		atomic.AddInt32(&active, -1)
		released, unlockErr := lease.Unlock(context.Background())
		if unlockErr != nil {
			return unlockErr
		}
		if !released {
			return fmt.Errorf("owner lost before unlock")
		}
		return nil
	}

	tester := concurrencytest.NewGoroutineStressTester(concurrencytest.Options{
		Workers:       16,
		RoundsPerTask: 64,
		Timeout:       10 * time.Second,
	})
	report := tester.RunT(t, task)
	if report.Completed != 64 {
		t.Fatalf("stress should complete all rounds, got %+v", report)
	}
	if atomic.LoadInt32(&winners) == 0 {
		t.Fatal("stress should acquire the lock at least once")
	}
}

func TestMutexAsyncCancellationDoesNotLeakKey(t *testing.T) {
	ctx := context.Background()
	client := redisClient(ctx, t)
	key := testLockKey(t)
	var sequence int64

	task := func(context.Context) error {
		id := atomic.AddInt64(&sequence, 1)
		mutex, err := redislock.New(client, redislock.Options{
			Key:   key,
			TTL:   time.Second,
			Token: fmt.Sprintf("owner-%d", id),
		})
		if err != nil {
			return err
		}
		cancelled, cancel := context.WithCancel(context.Background())
		cancel()
		_, err = mutex.TryLock(cancelled)
		if !errors.Is(err, context.Canceled) {
			return fmt.Errorf("expected context.Canceled, got %w", err)
		}
		return nil
	}

	tester := concurrencytest.NewAsyncJobTester(concurrencytest.Options{
		Workers:       4,
		RoundsPerTask: 16,
		Timeout:       5 * time.Second,
	})
	report := tester.RunT(t, task)
	if report.Completed != 16 {
		t.Fatalf("async tester should complete every round, got %+v", report)
	}
	assertRedisKeyMissing(t, client, key)
}

func redisClient(ctx context.Context, t *testing.T) *redis.Client {
	t.Helper()

	client := redis.NewClient(&redis.Options{Addr: redistestcontainer.Start(ctx, t)})
	t.Cleanup(func() {
		_ = client.Close()
	})
	bttesting.Eventually(t, 2*time.Second, func() bool {
		return client.Ping(ctx).Err() == nil
	})
	return client
}

func newMutex(t *testing.T, client redis.Cmdable, key string, token string, ttl time.Duration) *redislock.Mutex {
	t.Helper()

	mutex, err := redislock.New(client, redislock.Options{
		Key:   key,
		TTL:   ttl,
		Token: token,
	})
	if err != nil {
		t.Fatalf("new mutex: %v", err)
	}
	return mutex
}

func testLockKey(t *testing.T) string {
	t.Helper()
	name := strings.NewReplacer("/", ":", " ", "-", "_", "-").Replace(t.Name())
	return "bluetape:test:lock:" + name
}

func assertRedisKeyMissing(t *testing.T, client *redis.Client, key string) {
	t.Helper()

	bttesting.Eventually(t, time.Second, func() bool {
		return client.Exists(context.Background(), key).Val() == 0
	})
}

func assertRedactedRedisOpError(t *testing.T, err error, key string, token string) {
	t.Helper()

	if !errors.Is(err, redis.ErrClosed) {
		t.Fatalf("error should preserve redis.ErrClosed, got %v", err)
	}
	var opErr *btredis.OpError
	if !errors.As(err, &opErr) {
		t.Fatalf("error should be a redacted redis operation error, got %T", err)
	}
	if strings.Contains(err.Error(), key) || strings.Contains(err.Error(), token) {
		t.Fatalf("error leaked key or token: %v", err)
	}
}
