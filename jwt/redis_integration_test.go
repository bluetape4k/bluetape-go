package jwt

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	concurrencytest "github.com/bluetape4k/bluetape-go/testing/concurrency"
	"github.com/redis/go-redis/v9"
)

func TestRedisDistributedProvidersShareHMACKeysAcrossInstances(t *testing.T) {
	ctx := context.Background()
	client := jwtRedisClient(ctx, t)
	now := time.Date(2026, 6, 12, 10, 0, 0, 0, time.UTC)
	repoA := newTestRedisRepository(t, client, "hmac-share", RedisRepositoryOptions{})
	repoB := newTestRedisRepository(t, client, "hmac-share", RedisRepositoryOptions{})

	signer, err := NewDistributedHMACProvider(ctx, repoA, HS256,
		WithClock(func() time.Time { return now }),
		WithKeyIDGenerator(staticKID("hmac-shared")),
		WithEntropy(repeatingReader('h')),
	)
	if err != nil {
		t.Fatalf("signer constructor error = %v", err)
	}
	parser, err := NewDistributedHMACProvider(ctx, repoB, HS256,
		WithClock(func() time.Time { return now }),
		WithKeyIDGenerator(staticKID("unused")),
		WithEntropy(repeatingReader('u')),
	)
	if err != nil {
		t.Fatalf("parser constructor error = %v", err)
	}

	token, err := signer.ComposeContext(ctx, WithSubject("account-redis"), WithExpiresAfter(time.Hour))
	if err != nil {
		t.Fatalf("ComposeContext() error = %v", err)
	}
	reader, err := parser.ParseContext(ctx, token, WithExpectedSubject("account-redis"), WithParseClock(func() time.Time { return now }))
	if err != nil {
		t.Fatalf("ParseContext() error = %v", err)
	}
	if reader.Kid() != "hmac-shared" || reader.Algorithm() != HS256 {
		t.Fatalf("reader = %q/%q, want hmac-shared/%q", reader.Kid(), reader.Algorithm(), HS256)
	}
}

func TestRedisDistributedProvidersShareRSAKeysAcrossInstances(t *testing.T) {
	ctx := context.Background()
	client := jwtRedisClient(ctx, t)
	now := time.Date(2026, 6, 12, 10, 0, 0, 0, time.UTC)
	repoA := newTestRedisRepository(t, client, "rsa-share", RedisRepositoryOptions{})
	repoB := newTestRedisRepository(t, client, "rsa-share", RedisRepositoryOptions{})

	signer, err := NewDistributedRSAProvider(ctx, repoA, RS256,
		WithClock(func() time.Time { return now }),
		WithKeyIDGenerator(staticKID("rsa-shared")),
	)
	if err != nil {
		t.Fatalf("signer constructor error = %v", err)
	}
	parser, err := NewDistributedRSAProvider(ctx, repoB, RS256,
		WithClock(func() time.Time { return now }),
		WithKeyIDGenerator(staticKID("unused")),
	)
	if err != nil {
		t.Fatalf("parser constructor error = %v", err)
	}

	token, err := signer.ComposeContext(ctx, WithClaim("scope", "rsa"), WithExpiresAfter(time.Hour))
	if err != nil {
		t.Fatalf("ComposeContext() error = %v", err)
	}
	reader, err := parser.ParseContext(ctx, token, WithParseClock(func() time.Time { return now }))
	if err != nil {
		t.Fatalf("ParseContext() error = %v", err)
	}
	if reader.Kid() != "rsa-shared" || reader.Algorithm() != RS256 {
		t.Fatalf("reader = %q/%q, want rsa-shared/%q", reader.Kid(), reader.Algorithm(), RS256)
	}
}

func TestRedisDistributedProviderParsesAfterForcedRotationByKID(t *testing.T) {
	ctx := context.Background()
	client := jwtRedisClient(ctx, t)
	now := time.Date(2026, 6, 12, 10, 0, 0, 0, time.UTC)
	repo := newTestRedisRepository(t, client, "retained", RedisRepositoryOptions{Capacity: 2})
	provider, err := NewDistributedHMACProvider(ctx, repo, HS256,
		WithClock(func() time.Time { return now }),
		WithKeyIDGenerator(sequenceKID("retained")),
		WithEntropy(repeatingReader('r')),
	)
	if err != nil {
		t.Fatalf("constructor error = %v", err)
	}
	oldToken, err := provider.ComposeContext(ctx, WithClaim("phase", "old"), WithExpiresAfter(time.Hour))
	if err != nil {
		t.Fatalf("ComposeContext(old) error = %v", err)
	}
	oldKey, err := provider.CurrentKeyChainContext(ctx)
	if err != nil {
		t.Fatalf("CurrentKeyChainContext(old) error = %v", err)
	}
	if _, err := provider.ForcedRotateContext(ctx); err != nil {
		t.Fatalf("ForcedRotateContext() error = %v", err)
	}

	reader, err := provider.ParseContext(ctx, oldToken, WithParseClock(func() time.Time { return now }))
	if err != nil {
		t.Fatalf("ParseContext(old retained) error = %v", err)
	}
	if reader.Kid() != oldKey.KID() {
		t.Fatalf("reader kid = %q, want %q", reader.Kid(), oldKey.KID())
	}
}

func TestRedisDistributedProviderRejectsEvictedKID(t *testing.T) {
	ctx := context.Background()
	client := jwtRedisClient(ctx, t)
	now := time.Date(2026, 6, 12, 10, 0, 0, 0, time.UTC)
	repo := newTestRedisRepository(t, client, "evicted", RedisRepositoryOptions{Capacity: 2})
	provider, err := NewDistributedHMACProvider(ctx, repo, HS256,
		WithClock(func() time.Time { return now }),
		WithKeyIDGenerator(sequenceKID("evicted")),
		WithEntropy(repeatingReader('e')),
	)
	if err != nil {
		t.Fatalf("constructor error = %v", err)
	}
	oldToken, err := provider.ComposeContext(ctx, WithExpiresAfter(time.Hour))
	if err != nil {
		t.Fatalf("ComposeContext(old) error = %v", err)
	}
	if _, err := provider.ForcedRotateContext(ctx); err != nil {
		t.Fatalf("ForcedRotateContext(1) error = %v", err)
	}
	if _, err := provider.ForcedRotateContext(ctx); err != nil {
		t.Fatalf("ForcedRotateContext(2) error = %v", err)
	}

	if _, err := provider.ParseContext(ctx, oldToken, WithParseClock(func() time.Time { return now })); !errors.Is(err, ErrKeyNotFound) {
		t.Fatalf("ParseContext(evicted) error = %v, want ErrKeyNotFound", err)
	}
}

func TestRedisDistributedProviderRepositoryFailurePropagates(t *testing.T) {
	ctx := context.Background()
	client := jwtRedisClient(ctx, t)
	repo := newTestRedisRepository(t, client, "failure", RedisRepositoryOptions{})
	provider, err := NewDistributedHMACProvider(ctx, repo, HS256,
		WithKeyIDGenerator(staticKID("failure")),
		WithEntropy(repeatingReader('f')),
	)
	if err != nil {
		t.Fatalf("constructor error = %v", err)
	}
	if err := client.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	if _, err := provider.ComposeContext(ctx, WithExpiresAfter(time.Hour)); err == nil {
		t.Fatalf("ComposeContext() error = nil, want redis client failure")
	}
}

func TestRedisDistributedProviderConstructorCanceledAfterCreateDoesNotPersistCandidate(t *testing.T) {
	ctx := context.Background()
	client := jwtRedisClient(ctx, t)
	repo := newTestRedisRepository(t, client, "ctor-cancel", RedisRepositoryOptions{})
	canceled, cancel := context.WithCancel(ctx)

	_, err := NewDistributedHMACProvider(canceled, repo, HS256,
		WithKeyIDGenerator(func() (string, error) {
			cancel()
			return "ctor-cancel", nil
		}),
		WithEntropy(repeatingReader('c')),
	)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("constructor error = %v, want context.Canceled", err)
	}
	assertRedisEmptyRepository(ctx, t, repo)
}

func TestRedisDistributedProviderConstructorDeadlineAfterCreateDoesNotPersistCandidate(t *testing.T) {
	ctx := context.Background()
	client := jwtRedisClient(ctx, t)
	repo := newTestRedisRepository(t, client, "ctor-deadline", RedisRepositoryOptions{})
	deadline, cancel := context.WithTimeout(ctx, time.Millisecond)
	defer cancel()

	_, err := NewDistributedHMACProvider(deadline, repo, HS256,
		WithKeyIDGenerator(func() (string, error) {
			<-deadline.Done()
			return "ctor-deadline", nil
		}),
		WithEntropy(repeatingReader('d')),
	)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("constructor error = %v, want context.DeadlineExceeded", err)
	}
	assertRedisEmptyRepository(ctx, t, repo)
}

func TestRedisDistributedProviderConcurrentRotateSignParseStress(t *testing.T) {
	ctx := context.Background()
	client := jwtRedisClient(ctx, t)
	now := time.Date(2026, 6, 12, 10, 0, 0, 0, time.UTC)
	repo := newTestRedisRepository(t, client, "provider-stress", RedisRepositoryOptions{Capacity: 256})
	provider, err := NewDistributedHMACProvider(ctx, repo, HS256,
		WithClock(func() time.Time { return now }),
		WithKeyIDGenerator(sequenceKID("provider-stress")),
		WithEntropy(repeatingReader('p')),
	)
	if err != nil {
		t.Fatalf("constructor error = %v", err)
	}
	tester := concurrencytest.NewGoroutineStressTester(concurrencytest.Options{
		Workers:       8,
		RoundsPerTask: 20,
		Timeout:       10 * time.Second,
	})
	tester.RunT(t,
		func(ctx context.Context) error {
			token, err := provider.ComposeContext(ctx, WithClaim("stress", "ok"), WithExpiresAfter(time.Hour))
			if err != nil {
				return err
			}
			reader, err := provider.ParseContext(ctx, token, WithParseClock(func() time.Time { return now }))
			if err != nil {
				return err
			}
			if value, ok := reader.ClaimString("stress"); !ok || value != "ok" {
				return errorsNew("stress claim mismatch")
			}
			return nil
		},
		func(ctx context.Context) error {
			_, err := provider.ForcedRotateContext(ctx)
			return err
		},
	)
}

func TestRedisRepositoryConcurrentEmptyRotateConvergesOnOneCurrentWinner(t *testing.T) {
	ctx := context.Background()
	client := jwtRedisClient(ctx, t)
	repo := newTestRedisRepository(t, client, "empty-rotate-stress", RedisRepositoryOptions{})
	now := time.Date(2026, 6, 12, 10, 0, 0, 0, time.UTC)
	var mu sync.Mutex
	next := 0
	winners := map[string]struct{}{}
	tester := concurrencytest.NewGoroutineStressTester(concurrencytest.Options{
		Workers:       8,
		RoundsPerTask: 20,
		Timeout:       10 * time.Second,
	})

	tester.RunT(t, func(ctx context.Context) error {
		mu.Lock()
		next++
		kid := fmt.Sprintf("winner-%d", next)
		mu.Unlock()
		key, err := repo.Rotate(ctx, func() (*KeyChain, error) {
			return newTestHMACKey(t, kid, now, time.Hour), nil
		}, now)
		if err != nil {
			return err
		}
		mu.Lock()
		winners[key.KID()] = struct{}{}
		mu.Unlock()
		return nil
	})

	if len(winners) != 1 {
		t.Fatalf("winners = %v, want exactly one", winners)
	}
	if count := repo.client.HLen(ctx, repo.opts.keysKey()).Val(); count != 1 {
		t.Fatalf("stored key count = %d, want 1", count)
	}
}

func TestRedisDistributedProviderContextCancellationStress(t *testing.T) {
	ctx := context.Background()
	client := jwtRedisClient(ctx, t)
	repo := newTestRedisRepository(t, client, "provider-cancel", RedisRepositoryOptions{})
	provider, err := NewDistributedHMACProvider(ctx, repo, HS256,
		WithKeyIDGenerator(staticKID("provider-cancel")),
		WithEntropy(repeatingReader('c')),
	)
	if err != nil {
		t.Fatalf("constructor error = %v", err)
	}
	tester := concurrencytest.NewAsyncJobTester(concurrencytest.Options{
		Workers:       2,
		RoundsPerTask: 5,
		Timeout:       2 * time.Second,
	})
	tester.RunT(t, func(ctx context.Context) error {
		canceled, cancel := context.WithCancel(ctx)
		cancel()
		_, err := provider.ComposeContext(canceled, WithExpiresAfter(time.Hour))
		if !errors.Is(err, context.Canceled) {
			return fmt.Errorf("expected context.Canceled, got %w", err)
		}
		return nil
	})
}

func TestRedisDistributedProviderDeadlineStress(t *testing.T) {
	ctx := context.Background()
	client := jwtRedisClient(ctx, t)
	repo := newTestRedisRepository(t, client, "provider-deadline", RedisRepositoryOptions{})
	provider, err := NewDistributedHMACProvider(ctx, repo, HS256,
		WithKeyIDGenerator(staticKID("provider-deadline")),
		WithEntropy(repeatingReader('d')),
	)
	if err != nil {
		t.Fatalf("constructor error = %v", err)
	}
	tester := concurrencytest.NewAsyncJobTester(concurrencytest.Options{
		Workers:       2,
		RoundsPerTask: 5,
		Timeout:       2 * time.Second,
	})
	tester.RunT(t, func(ctx context.Context) error {
		expired, cancel := context.WithDeadline(ctx, time.Now().Add(-time.Second))
		defer cancel()
		_, err := provider.CurrentKeyChainContext(expired)
		if !errors.Is(err, context.DeadlineExceeded) {
			return fmt.Errorf("expected context.DeadlineExceeded, got %w", err)
		}
		return nil
	})
}

var _ redis.Cmdable = (*redis.Client)(nil)
