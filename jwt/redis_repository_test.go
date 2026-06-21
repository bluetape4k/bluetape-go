package jwt

import (
	"context"
	"errors"
	"fmt"
	"os"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/bluetape4k/bluetape-go/internal/testcleanup"
	concurrencytest "github.com/bluetape4k/bluetape-go/testing/concurrency"
	"github.com/redis/go-redis/v9"
	tcredis "github.com/testcontainers/testcontainers-go/modules/redis"
)

func TestRepositoryCurrentReturnsNewestNonExpiredKey(t *testing.T) {
	ctx := context.Background()
	client := jwtRedisClient(ctx, t)
	repo := newTestRedisRepository(t, client, "current", RedisRepositoryOptions{})
	now := time.Date(2026, 6, 12, 10, 0, 0, 0, time.UTC)
	oldKey := newTestHMACKey(t, "old", now.Add(-time.Hour), time.Hour)
	newKey := newTestHMACKey(t, "new", now, time.Hour)
	seedRedisKeyChain(ctx, t, repo, oldKey, false)
	seedRedisKeyChain(ctx, t, repo, newKey, true)

	current, err := repo.Current(ctx, now)
	if err != nil {
		t.Fatalf("Current() error = %v", err)
	}
	if current.KID() != "new" {
		t.Fatalf("Current() kid = %q, want new", current.KID())
	}
}

func TestRepositoryFindUsesKIDHashLookup(t *testing.T) {
	ctx := context.Background()
	client := jwtRedisClient(ctx, t)
	recorder := &redisCommandRecorder{}
	client.AddHook(recorder)
	repo := newTestRedisRepository(t, client, "find-hash", RedisRepositoryOptions{})
	now := time.Date(2026, 6, 12, 10, 0, 0, 0, time.UTC)
	key := newTestHMACKey(t, "target", now, time.Hour)
	seedRedisKeyChain(ctx, t, repo, key, true)
	recorder.reset()

	found, err := repo.Find(ctx, "target", now)
	if err != nil {
		t.Fatalf("Find() error = %v", err)
	}
	if found.KID() != "target" {
		t.Fatalf("Find() kid = %q, want target", found.KID())
	}
	assertRedisCommands(t, recorder.commands(), []string{"hget"})
	assertNoRedisScanCommands(t, recorder.commands())
}

func TestRepositoryFindRejectsMissingUnknownAndExpiredKID(t *testing.T) {
	ctx := context.Background()
	client := jwtRedisClient(ctx, t)
	repo := newTestRedisRepository(t, client, "find-reject", RedisRepositoryOptions{})
	now := time.Date(2026, 6, 12, 10, 0, 0, 0, time.UTC)
	expired := newTestHMACKey(t, "expired", now.Add(-2*time.Hour), time.Hour)
	seedRedisKeyChain(ctx, t, repo, expired, true)

	if _, err := repo.Find(ctx, "", now); !errors.Is(err, ErrKeyNotFound) {
		t.Fatalf("Find(empty) error = %v, want ErrKeyNotFound", err)
	}
	if _, err := repo.Find(ctx, strings.Repeat("a", maxKIDBytes+1), now); !errors.Is(err, ErrKeyNotFound) {
		t.Fatalf("Find(long kid) error = %v, want ErrKeyNotFound", err)
	}
	if _, err := repo.Find(ctx, "bad\nkid", now); !errors.Is(err, ErrKeyNotFound) {
		t.Fatalf("Find(control kid) error = %v, want ErrKeyNotFound", err)
	}
	if _, err := repo.Find(ctx, "missing", now); !errors.Is(err, ErrKeyNotFound) {
		t.Fatalf("Find(missing) error = %v, want ErrKeyNotFound", err)
	}
	if _, err := repo.Find(ctx, "expired", now); !errors.Is(err, ErrInvalidKey) {
		t.Fatalf("Find(expired) error = %v, want ErrInvalidKey", err)
	}
}

func TestRepositoryDeleteAllRemovesNamespacedState(t *testing.T) {
	ctx := context.Background()
	client := jwtRedisClient(ctx, t)
	repo := newTestRedisRepository(t, client, "delete", RedisRepositoryOptions{})
	now := time.Date(2026, 6, 12, 10, 0, 0, 0, time.UTC)
	seedRedisKeyChain(ctx, t, repo, newTestHMACKey(t, "delete-kid", now, time.Hour), true)

	if err := repo.DeleteAll(ctx); err != nil {
		t.Fatalf("DeleteAll() error = %v", err)
	}
	if exists := client.Exists(ctx, repo.opts.metaKey(), repo.opts.currentKey(), repo.opts.keysKey(), repo.opts.orderKey()).Val(); exists != 0 {
		t.Fatalf("remaining redis keys = %d, want 0", exists)
	}
}

func TestRepositoryNamespaceIsolation(t *testing.T) {
	ctx := context.Background()
	client := jwtRedisClient(ctx, t)
	repoA := newTestRedisRepository(t, client, "tenant-a", RedisRepositoryOptions{})
	repoB := newTestRedisRepository(t, client, "tenant-b", RedisRepositoryOptions{})
	now := time.Date(2026, 6, 12, 10, 0, 0, 0, time.UTC)
	seedRedisKeyChain(ctx, t, repoA, newTestHMACKey(t, "shared", now, time.Hour), true)

	if _, err := repoA.Find(ctx, "shared", now); err != nil {
		t.Fatalf("repoA Find() error = %v", err)
	}
	if _, err := repoB.Find(ctx, "shared", now); !errors.Is(err, ErrKeyNotFound) {
		t.Fatalf("repoB Find() error = %v, want ErrKeyNotFound", err)
	}
}

func TestRepositoryAlgorithmFamilyMismatchFails(t *testing.T) {
	ctx := context.Background()
	client := jwtRedisClient(ctx, t)
	repo := newTestRedisRepository(t, client, "algorithm", RedisRepositoryOptions{})
	dto := keyChainDTO{
		Version:   redisKeyVersion,
		KID:       "bad",
		Algorithm: string(RS256),
		Family:    redisKeyFamilyHMAC,
		CreatedAt: time.Now(),
		ExpiresAt: time.Now().Add(time.Hour),
		HMAC:      "not-secret",
	}
	client.HSet(ctx, repo.opts.keysKey(), "bad", string(marshalRedisDTO(t, dto)))

	if _, err := repo.Find(ctx, "bad", time.Now()); !errors.Is(err, ErrInvalidKey) {
		t.Fatalf("Find() error = %v, want ErrInvalidKey", err)
	}
}

func TestRepositoryContextCancellationPreserved(t *testing.T) {
	ctx := context.Background()
	client := jwtRedisClient(ctx, t)
	repo := newTestRedisRepository(t, client, "cancel", RedisRepositoryOptions{})
	tester := concurrencytest.NewAsyncJobTester(concurrencytest.Options{
		Workers:       2,
		RoundsPerTask: 5,
		Timeout:       2 * time.Second,
	})
	tester.RunT(t, func(ctx context.Context) error {
		canceled, cancel := context.WithCancel(ctx)
		cancel()
		_, err := repo.Find(canceled, "kid", time.Now())
		if !errors.Is(err, context.Canceled) {
			return fmt.Errorf("expected context.Canceled, got %w", err)
		}
		return nil
	})
}

func TestRepositoryDeadlinePreserved(t *testing.T) {
	ctx := context.Background()
	client := jwtRedisClient(ctx, t)
	repo := newTestRedisRepository(t, client, "deadline", RedisRepositoryOptions{})
	expired, cancel := context.WithDeadline(ctx, time.Now().Add(-time.Second))
	defer cancel()
	if _, err := repo.Current(expired, time.Now()); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Current(expired ctx) error = %v, want context.DeadlineExceeded", err)
	}
}

func TestRepositoryFindCommandBudget(t *testing.T) {
	ctx := context.Background()
	client := jwtRedisClient(ctx, t)
	recorder := &redisCommandRecorder{}
	client.AddHook(recorder)
	repo := newTestRedisRepository(t, client, "find-budget", RedisRepositoryOptions{})
	now := time.Date(2026, 6, 12, 10, 0, 0, 0, time.UTC)
	seedRedisKeyChain(ctx, t, repo, newTestHMACKey(t, "budget", now, time.Hour), true)
	recorder.reset()

	if _, err := repo.Find(ctx, "budget", now); err != nil {
		t.Fatalf("Find() error = %v", err)
	}
	assertRedisCommands(t, recorder.commands(), []string{"hget"})
	assertNoRedisScanCommands(t, recorder.commands())
}

func TestRepositoryCurrentCommandBudget(t *testing.T) {
	ctx := context.Background()
	client := jwtRedisClient(ctx, t)
	recorder := &redisCommandRecorder{}
	client.AddHook(recorder)
	repo := newTestRedisRepository(t, client, "current-budget", RedisRepositoryOptions{})
	now := time.Date(2026, 6, 12, 10, 0, 0, 0, time.UTC)
	seedRedisKeyChain(ctx, t, repo, newTestHMACKey(t, "current-budget", now, time.Hour), true)
	recorder.reset()

	if _, err := repo.Current(ctx, now); err != nil {
		t.Fatalf("Current() error = %v", err)
	}
	assertRedisCommands(t, recorder.commands(), []string{"get", "hget"})
	assertNoRedisScanCommands(t, recorder.commands())
}

func TestRepositoryRotateReturnsCurrentWithoutCallingCreate(t *testing.T) {
	ctx := context.Background()
	client := jwtRedisClient(ctx, t)
	repo := newTestRedisRepository(t, client, "rotate-current", RedisRepositoryOptions{})
	now := time.Date(2026, 6, 12, 10, 0, 0, 0, time.UTC)
	current := newTestHMACKey(t, "current", now, time.Hour)
	seedRedisKeyChain(ctx, t, repo, current, true)

	rotated, err := repo.Rotate(ctx, func() (*KeyChain, error) {
		t.Fatal("create must not run on current-hit rotate")
		return nil, nil
	}, now)
	if err != nil {
		t.Fatalf("Rotate() error = %v", err)
	}
	if rotated.KID() != "current" {
		t.Fatalf("Rotate() kid = %q, want current", rotated.KID())
	}
}

func TestRepositoryRotateStoresCandidateWhenNoCurrentKeyExists(t *testing.T) {
	ctx := context.Background()
	client := jwtRedisClient(ctx, t)
	repo := newTestRedisRepository(t, client, "rotate-empty", RedisRepositoryOptions{})
	now := time.Date(2026, 6, 12, 10, 0, 0, 0, time.UTC)

	rotated, err := repo.Rotate(ctx, func() (*KeyChain, error) {
		return newTestHMACKey(t, "created", now, time.Hour), nil
	}, now)
	if err != nil {
		t.Fatalf("Rotate() error = %v", err)
	}
	if rotated.KID() != "created" {
		t.Fatalf("Rotate() kid = %q, want created", rotated.KID())
	}
	current, err := repo.Current(ctx, now)
	if err != nil {
		t.Fatalf("Current() error = %v", err)
	}
	if current.KID() != "created" {
		t.Fatalf("Current() kid = %q, want created", current.KID())
	}
}

func TestRepositoryRotateCASReturnsConcurrentWinner(t *testing.T) {
	ctx := context.Background()
	client := jwtRedisClient(ctx, t)
	now := time.Date(2026, 6, 12, 10, 0, 0, 0, time.UTC)
	tester := concurrencytest.NewGoroutineStressTester(concurrencytest.Options{
		Workers:       1,
		RoundsPerTask: 3,
		Timeout:       3 * time.Second,
	})
	round := 0
	var mu sync.Mutex

	tester.RunT(t, func(context.Context) error {
		mu.Lock()
		round++
		namespace := fmt.Sprintf("rotate-cas-%d", round)
		mu.Unlock()

		repo := newTestRedisRepository(t, client, namespace, RedisRepositoryOptions{})
		ready := make(chan struct{}, 2)
		release := make(chan struct{})
		var releaseOnce sync.Once
		results := make(chan string, 2)
		errs := make(chan error, 2)
		startRotate := func(kid string) {
			defer func() {
				if recovered := recover(); recovered != nil {
					errs <- fmt.Errorf("panic: %v", recovered)
				}
			}()
			key, err := repo.Rotate(ctx, func() (*KeyChain, error) {
				ready <- struct{}{}
				<-release
				return newTestHMACKey(t, kid, now, time.Hour), nil
			}, now)
			if err != nil {
				errs <- err
				return
			}
			results <- key.KID()
		}

		go startRotate(namespace + "-a")
		go startRotate(namespace + "-b")

		for i := 0; i < 2; i++ {
			select {
			case <-ready:
			case <-time.After(200 * time.Millisecond):
				releaseOnce.Do(func() { close(release) })
			}
		}
		releaseOnce.Do(func() { close(release) })

		got := make([]string, 0, 2)
		for i := 0; i < 2; i++ {
			select {
			case err := <-errs:
				return err
			case kid := <-results:
				got = append(got, kid)
			case <-time.After(2 * time.Second):
				return fmt.Errorf("timed out waiting for rotate results")
			}
		}
		if got[0] != got[1] {
			return fmt.Errorf("concurrent rotate returned different winners: %v", got)
		}
		if count := repo.client.HLen(ctx, repo.opts.keysKey()).Val(); count != 1 {
			return fmt.Errorf("stored key count = %d, want 1", count)
		}
		return nil
	})
}

func TestRepositoryForcedRotateAlwaysStoresCandidate(t *testing.T) {
	ctx := context.Background()
	client := jwtRedisClient(ctx, t)
	repo := newTestRedisRepository(t, client, "forced-rotate", RedisRepositoryOptions{})
	now := time.Date(2026, 6, 12, 10, 0, 0, 0, time.UTC)
	seedRedisKeyChain(ctx, t, repo, newTestHMACKey(t, "old", now.Add(-time.Minute), time.Hour), true)

	rotated, err := repo.ForcedRotate(ctx, func() (*KeyChain, error) {
		return newTestHMACKey(t, "forced", now, time.Hour), nil
	}, now)
	if err != nil {
		t.Fatalf("ForcedRotate() error = %v", err)
	}
	if rotated.KID() != "forced" {
		t.Fatalf("ForcedRotate() kid = %q, want forced", rotated.KID())
	}
	current, err := repo.Current(ctx, now)
	if err != nil {
		t.Fatalf("Current() error = %v", err)
	}
	if current.KID() != "forced" {
		t.Fatalf("Current() kid = %q, want forced", current.KID())
	}
}

func TestRepositoryCapacityTrimPreservesNewestKeys(t *testing.T) {
	ctx := context.Background()
	client := jwtRedisClient(ctx, t)
	repo := newTestRedisRepository(t, client, "capacity", RedisRepositoryOptions{Capacity: 2})
	now := time.Date(2026, 6, 12, 10, 0, 0, 0, time.UTC)

	for i, kid := range []string{"old", "middle", "new"} {
		createdAt := now.Add(time.Duration(i) * time.Minute)
		if _, err := repo.ForcedRotate(ctx, func() (*KeyChain, error) {
			return newTestHMACKey(t, kid, createdAt, time.Hour), nil
		}, createdAt); err != nil {
			t.Fatalf("ForcedRotate(%s) error = %v", kid, err)
		}
	}
	if _, err := repo.Find(ctx, "old", now.Add(3*time.Minute)); !errors.Is(err, ErrKeyNotFound) {
		t.Fatalf("Find(old) error = %v, want ErrKeyNotFound", err)
	}
	for _, kid := range []string{"middle", "new"} {
		if _, err := repo.Find(ctx, kid, now.Add(3*time.Minute)); err != nil {
			t.Fatalf("Find(%s) error = %v", kid, err)
		}
	}
	if count := repo.client.HLen(ctx, repo.opts.keysKey()).Val(); count != 2 {
		t.Fatalf("stored key count = %d, want 2", count)
	}
}

func TestRepositoryCapacityTrimKeepsCandidateAndNewestRetainedKey(t *testing.T) {
	ctx := context.Background()
	client := jwtRedisClient(ctx, t)
	repo := newTestRedisRepository(t, client, "capacity-skew", RedisRepositoryOptions{Capacity: 2})
	now := time.Date(2026, 6, 12, 10, 0, 0, 0, time.UTC)

	for i, kid := range []string{"middle", "new"} {
		createdAt := now.Add(time.Duration(i+1) * time.Minute)
		if _, err := repo.ForcedRotate(ctx, func() (*KeyChain, error) {
			return newTestHMACKey(t, kid, createdAt, time.Hour), nil
		}, createdAt); err != nil {
			t.Fatalf("ForcedRotate(%s) error = %v", kid, err)
		}
	}
	if _, err := repo.ForcedRotate(ctx, func() (*KeyChain, error) {
		return newTestHMACKey(t, "skewed-candidate", now.Add(-time.Minute), time.Hour), nil
	}, now); err != nil {
		t.Fatalf("ForcedRotate(skewed-candidate) error = %v", err)
	}

	if count := repo.client.HLen(ctx, repo.opts.keysKey()).Val(); count != 2 {
		t.Fatalf("stored key count = %d, want 2", count)
	}
	if _, err := repo.Find(ctx, "skewed-candidate", now); err != nil {
		t.Fatalf("Find(skewed-candidate) error = %v", err)
	}
	if _, err := repo.Find(ctx, "new", now.Add(3*time.Minute)); err != nil {
		t.Fatalf("Find(new) error = %v", err)
	}
	if _, err := repo.Find(ctx, "middle", now.Add(3*time.Minute)); !errors.Is(err, ErrKeyNotFound) {
		t.Fatalf("Find(middle) error = %v, want ErrKeyNotFound", err)
	}
}

func TestRepositoryKeyTTLZeroLeavesKeysWithoutRedisExpiration(t *testing.T) {
	ctx := context.Background()
	client := jwtRedisClient(ctx, t)
	repo := newTestRedisRepository(t, client, "ttl-zero", RedisRepositoryOptions{})
	now := time.Date(2026, 6, 12, 10, 0, 0, 0, time.UTC)

	if _, err := repo.ForcedRotate(ctx, func() (*KeyChain, error) {
		return newTestHMACKey(t, "ttl-zero", now, time.Hour), nil
	}, now); err != nil {
		t.Fatalf("ForcedRotate() error = %v", err)
	}
	assertRedisNoExpiration(ctx, t, repo, repo.opts.metaKey(), repo.opts.currentKey(), repo.opts.keysKey(), repo.opts.orderKey())
}

func TestRepositoryConfiguredKeyTTLRetainsNonExpiredKeys(t *testing.T) {
	ctx := context.Background()
	client := jwtRedisClient(ctx, t)
	repo := newTestRedisRepository(t, client, "ttl-configured", RedisRepositoryOptions{
		KeyTTL:          2 * time.Hour,
		RetentionLeeway: 30 * time.Minute,
	})
	now := time.Date(2026, 6, 12, 10, 0, 0, 0, time.UTC)

	if _, err := repo.ForcedRotate(ctx, func() (*KeyChain, error) {
		return newTestHMACKey(t, "ttl-configured", now, time.Hour), nil
	}, now); err != nil {
		t.Fatalf("ForcedRotate() error = %v", err)
	}
	assertRedisHasExpiration(ctx, t, repo, repo.opts.metaKey(), repo.opts.currentKey(), repo.opts.keysKey(), repo.opts.orderKey())
	if _, err := repo.Find(ctx, "ttl-configured", now.Add(30*time.Minute)); err != nil {
		t.Fatalf("Find(non-expired) error = %v", err)
	}
}

func TestRepositoryConfiguredKeyTTLUsesKeyValidityNotStoreClock(t *testing.T) {
	ctx := context.Background()
	client := jwtRedisClient(ctx, t)
	repo := newTestRedisRepository(t, client, "ttl-store-clock", RedisRepositoryOptions{
		KeyTTL: time.Hour,
	})
	storeNow := time.Date(2026, 6, 12, 10, 0, 0, 0, time.UTC)
	keyCreatedAt := storeNow.Add(5 * time.Second)

	if _, err := repo.ForcedRotate(ctx, func() (*KeyChain, error) {
		return newTestHMACKey(t, "ttl-store-clock", keyCreatedAt, time.Hour), nil
	}, storeNow); err != nil {
		t.Fatalf("ForcedRotate() error = %v", err)
	}
}

func TestRepositoryRejectsKeyTTLShorterThanRetainedKeyValidityAndRetentionLeeway(t *testing.T) {
	ctx := context.Background()
	client := jwtRedisClient(ctx, t)
	repo := newTestRedisRepository(t, client, "ttl-short", RedisRepositoryOptions{
		KeyTTL:          30 * time.Minute,
		RetentionLeeway: 10 * time.Minute,
	})
	now := time.Date(2026, 6, 12, 10, 0, 0, 0, time.UTC)

	_, err := repo.ForcedRotate(ctx, func() (*KeyChain, error) {
		return newTestHMACKey(t, "ttl-short", now, time.Hour), nil
	}, now)
	if !errors.Is(err, ErrInvalidOptions) {
		t.Fatalf("ForcedRotate() error = %v, want ErrInvalidOptions", err)
	}
	if count := repo.client.HLen(ctx, repo.opts.keysKey()).Val(); count != 0 {
		t.Fatalf("stored key count = %d, want 0", count)
	}
}

func TestRepositoryRotateCanceledAfterCreateDoesNotPersistCandidate(t *testing.T) {
	ctx := context.Background()
	client := jwtRedisClient(ctx, t)
	repo := newTestRedisRepository(t, client, "rotate-cancel", RedisRepositoryOptions{})
	now := time.Date(2026, 6, 12, 10, 0, 0, 0, time.UTC)
	canceled, cancel := context.WithCancel(ctx)

	_, err := repo.Rotate(canceled, func() (*KeyChain, error) {
		cancel()
		return newTestHMACKey(t, "canceled", now, time.Hour), nil
	}, now)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Rotate() error = %v, want context.Canceled", err)
	}
	assertRedisEmptyRepository(ctx, t, repo)
}

func TestRepositoryForcedRotateCanceledAfterCreateDoesNotPersistCandidate(t *testing.T) {
	ctx := context.Background()
	client := jwtRedisClient(ctx, t)
	repo := newTestRedisRepository(t, client, "forced-cancel", RedisRepositoryOptions{})
	now := time.Date(2026, 6, 12, 10, 0, 0, 0, time.UTC)
	canceled, cancel := context.WithCancel(ctx)

	_, err := repo.ForcedRotate(canceled, func() (*KeyChain, error) {
		cancel()
		return newTestHMACKey(t, "canceled", now, time.Hour), nil
	}, now)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("ForcedRotate() error = %v, want context.Canceled", err)
	}
	assertRedisEmptyRepository(ctx, t, repo)
}

func TestRepositoryRotateCurrentHitCommandBudget(t *testing.T) {
	ctx := context.Background()
	client := jwtRedisClient(ctx, t)
	recorder := &redisCommandRecorder{}
	client.AddHook(recorder)
	repo := newTestRedisRepository(t, client, "rotate-budget", RedisRepositoryOptions{})
	now := time.Date(2026, 6, 12, 10, 0, 0, 0, time.UTC)
	seedRedisKeyChain(ctx, t, repo, newTestHMACKey(t, "budget", now, time.Hour), true)
	recorder.reset()

	if _, err := repo.Rotate(ctx, func() (*KeyChain, error) {
		t.Fatal("create must not run on current-hit rotate")
		return nil, nil
	}, now); err != nil {
		t.Fatalf("Rotate() error = %v", err)
	}
	assertRedisCommands(t, recorder.commands(), []string{"eval"})
	assertNoRedisScanCommands(t, recorder.commands())
}

func jwtRedisClient(ctx context.Context, t *testing.T) *redis.Client {
	t.Helper()
	addr, err := jwtRedisAddr(ctx)
	if err != nil {
		t.Fatalf("start redis container: %v", err)
	}
	client := redis.NewClient(&redis.Options{Addr: addr})
	t.Cleanup(func() { _ = client.Close() })
	waitForJWTRedis(t, client)
	return client
}

var jwtRedisFixture struct {
	once      sync.Once
	container *tcredis.RedisContainer
	addr      string
	err       error
}

func jwtRedisAddr(ctx context.Context) (string, error) {
	jwtRedisFixture.once.Do(func() {
		container, err := tcredis.Run(ctx, "redis:7.4-alpine")
		if err != nil {
			jwtRedisFixture.err = err
			return
		}
		addr, err := container.PortEndpoint(ctx, "6379/tcp", "")
		if err != nil {
			jwtRedisFixture.err = err
			_ = testcleanup.Terminate(ctx, 0, container)
			return
		}
		jwtRedisFixture.container = container
		jwtRedisFixture.addr = addr
	})
	return jwtRedisFixture.addr, jwtRedisFixture.err
}

func TestMain(m *testing.M) {
	code := m.Run()
	if jwtRedisFixture.container != nil {
		_ = testcleanup.Terminate(context.Background(), 0, jwtRedisFixture.container)
	}
	os.Exit(code)
}

func waitForJWTRedis(t *testing.T, client *redis.Client) {
	t.Helper()
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
	t.Fatalf("redis did not become ready: %v", lastErr)
}

func newTestRedisRepository(t *testing.T, client redis.Cmdable, namespace string, options RedisRepositoryOptions) *RedisRepository {
	t.Helper()
	if options.Client == nil {
		options.Client = client
	}
	if options.Namespace == "" {
		options.Namespace = testRedisRepositoryNamespace(t, namespace)
	}
	repo, err := NewRedisRepository(options)
	if err != nil {
		t.Fatalf("NewRedisRepository() error = %v", err)
	}
	return repo
}

func testRedisRepositoryNamespace(t *testing.T, namespace string) string {
	t.Helper()
	name := strings.NewReplacer("/", "-", " ", "-", "_", "-").Replace(t.Name())
	return namespace + "-" + name
}

func newTestHMACKey(t *testing.T, kid string, createdAt time.Time, ttl time.Duration) *KeyChain {
	t.Helper()
	key, err := newHMACKeyChain(kid, HS256, bytesOf('r', 32), createdAt, ttl)
	if err != nil {
		t.Fatalf("newHMACKeyChain() error = %v", err)
	}
	return key
}

func seedRedisKeyChain(ctx context.Context, t *testing.T, repo *RedisRepository, key *KeyChain, current bool) {
	t.Helper()
	payload, err := encodeRedisKeyChain(key)
	if err != nil {
		t.Fatalf("encodeRedisKeyChain() error = %v", err)
	}
	if err := repo.client.HSet(ctx, repo.opts.keysKey(), key.KID(), string(payload)).Err(); err != nil {
		t.Fatalf("HSet() error = %v", err)
	}
	if err := repo.client.ZAdd(ctx, repo.opts.orderKey(), redis.Z{Score: float64(key.CreatedAt().UnixNano()), Member: key.KID()}).Err(); err != nil {
		t.Fatalf("ZAdd() error = %v", err)
	}
	if err := repo.client.HSet(ctx, repo.opts.metaKey(), "version", redisKeyVersion, "family", redisKeyFamilyHMAC).Err(); err != nil {
		t.Fatalf("HSet(meta) error = %v", err)
	}
	if current {
		if err := repo.client.Set(ctx, repo.opts.currentKey(), key.KID(), 0).Err(); err != nil {
			t.Fatalf("Set(current) error = %v", err)
		}
	}
}

type redisCommandRecorder struct {
	mu       sync.Mutex
	recorded []string
}

func (r *redisCommandRecorder) DialHook(next redis.DialHook) redis.DialHook {
	return next
}

func (r *redisCommandRecorder) ProcessHook(next redis.ProcessHook) redis.ProcessHook {
	return func(ctx context.Context, cmd redis.Cmder) error {
		r.record(cmd.Name())
		return next(ctx, cmd)
	}
}

func (r *redisCommandRecorder) ProcessPipelineHook(next redis.ProcessPipelineHook) redis.ProcessPipelineHook {
	return func(ctx context.Context, cmds []redis.Cmder) error {
		for _, cmd := range cmds {
			r.record(cmd.Name())
		}
		return next(ctx, cmds)
	}
}

func (r *redisCommandRecorder) record(command string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.recorded = append(r.recorded, strings.ToLower(command))
}

func (r *redisCommandRecorder) reset() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.recorded = nil
}

func (r *redisCommandRecorder) commands() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string(nil), r.recorded...)
}

func assertRedisCommands(t *testing.T, got []string, want []string) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("redis commands = %v, want %v", got, want)
	}
}

func assertNoRedisScanCommands(t *testing.T, commands []string) {
	t.Helper()
	for _, command := range commands {
		switch strings.ToLower(command) {
		case "scan", "keys", "lrange", "zrange", "hgetall":
			t.Fatalf("redis command log contains forbidden command %q: %v", command, commands)
		}
	}
}

func assertRedisNoExpiration(ctx context.Context, t *testing.T, repo *RedisRepository, keys ...string) {
	t.Helper()
	for _, key := range keys {
		if ttl := repo.client.TTL(ctx, key).Val(); ttl != -1 {
			t.Fatalf("TTL(%s) = %v, want no expiration", key, ttl)
		}
	}
}

func assertRedisHasExpiration(ctx context.Context, t *testing.T, repo *RedisRepository, keys ...string) {
	t.Helper()
	for _, key := range keys {
		if ttl := repo.client.TTL(ctx, key).Val(); ttl <= 0 {
			t.Fatalf("TTL(%s) = %v, want positive expiration", key, ttl)
		}
	}
}

func assertRedisEmptyRepository(ctx context.Context, t *testing.T, repo *RedisRepository) {
	t.Helper()
	if current := repo.client.Exists(ctx, repo.opts.currentKey()).Val(); current != 0 {
		t.Fatalf("current key exists = %d, want 0", current)
	}
	if keys := repo.client.HLen(ctx, repo.opts.keysKey()).Val(); keys != 0 {
		t.Fatalf("hash key count = %d, want 0", keys)
	}
	if order := repo.client.ZCard(ctx, repo.opts.orderKey()).Val(); order != 0 {
		t.Fatalf("order key count = %d, want 0", order)
	}
}
