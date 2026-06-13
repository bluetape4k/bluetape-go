package jwt

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	concurrencytest "github.com/bluetape4k/bluetape-go/testing/concurrency"
)

func TestCachedProviderColdBurstUsesSingleflight(t *testing.T) {
	now := time.Date(2026, 6, 14, 1, 0, 0, 0, time.UTC)
	provider := newTestFixedHMACProviderAt(t, now)
	cache := newSpyReaderCache(func() time.Time { return now })
	cached, err := NewCachedProvider(provider, cache,
		WithCacheClock(func() time.Time { return now }),
		WithCacheTrustScope("singleflight-scope"),
	)
	if err != nil {
		t.Fatalf("NewCachedProvider() error = %v", err)
	}
	token, err := provider.Compose(WithSubject("subject"), WithExpiresAfter(time.Hour))
	if err != nil {
		t.Fatalf("Compose() error = %v", err)
	}

	tester := concurrencytest.NewGoroutineStressTester(concurrencytest.Options{
		Workers:       16,
		RoundsPerTask: 16,
		Timeout:       5 * time.Second,
	})
	tester.RunT(t, func(context.Context) error {
		reader, err := cached.Parse(token, WithExpectedSubject("subject"))
		if err != nil {
			return err
		}
		if reader.Subject() != "subject" {
			return fmt.Errorf("subject = %q", reader.Subject())
		}
		return nil
	})

	_, sets, deletes, _ := cache.snapshot()
	if sets != 1 || deletes != 0 {
		t.Fatalf("cold burst should cache once without stale delete, sets=%d deletes=%d", sets, deletes)
	}
}

func TestCachedProviderStressParseAndForcedRotate(t *testing.T) {
	provider, err := NewHMACProvider(HS256,
		WithRepositoryCapacity(256),
		WithKeyIDGenerator(sequenceKID("cached-stress")),
		WithEntropy(repeatingReader('s')),
	)
	if err != nil {
		t.Fatalf("NewHMACProvider() error = %v", err)
	}
	cache := newSpyReaderCache(time.Now)
	cached, err := NewCachedProvider(provider, cache, WithCacheTrustScope("stress-scope"))
	if err != nil {
		t.Fatalf("NewCachedProvider() error = %v", err)
	}
	var counter atomic.Int64

	tester := concurrencytest.NewGoroutineStressTester(concurrencytest.Options{
		Workers:       8,
		RoundsPerTask: 20,
		Timeout:       5 * time.Second,
	})
	tester.RunT(t,
		func(context.Context) error {
			n := counter.Add(1)
			token, err := cached.Compose(WithClaim("n", fmt.Sprintf("%d", n)), WithExpiresAfter(time.Hour))
			if err != nil {
				return err
			}
			reader, err := cached.Parse(token)
			if err != nil {
				return err
			}
			if _, ok := reader.ClaimString("n"); !ok {
				return errorsNew("missing n claim")
			}
			return nil
		},
		func(context.Context) error {
			_, err := cached.ForcedRotate()
			return err
		},
	)
}

func TestCachedDistributedProviderColdBurstUsesSingleflight(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 6, 14, 1, 0, 0, 0, time.UTC)
	repo := &fakeDistributedRepository{}
	provider, err := NewDistributedHMACProvider(ctx, repo, HS256,
		WithClock(func() time.Time { return now }),
		WithKeyIDGenerator(staticKID("distributed-cold-burst")),
		WithEntropy(repeatingReader('d')),
	)
	if err != nil {
		t.Fatalf("NewDistributedHMACProvider() error = %v", err)
	}
	cache := newSpyReaderCache(func() time.Time { return now })
	cached, err := NewCachedDistributedProvider(provider, cache,
		WithCacheClock(func() time.Time { return now }),
		WithCacheTrustScope("distributed-singleflight-scope"),
	)
	if err != nil {
		t.Fatalf("NewCachedDistributedProvider() error = %v", err)
	}
	token, err := provider.ComposeContext(ctx, WithSubject("subject"), WithExpiresAfter(time.Hour))
	if err != nil {
		t.Fatalf("ComposeContext() error = %v", err)
	}

	tester := concurrencytest.NewGoroutineStressTester(concurrencytest.Options{
		Workers:       16,
		RoundsPerTask: 16,
		Timeout:       5 * time.Second,
	})
	tester.RunT(t, func(ctx context.Context) error {
		reader, err := cached.ParseContext(ctx, token, WithExpectedSubject("subject"))
		if err != nil {
			return err
		}
		if reader.Subject() != "subject" {
			return fmt.Errorf("subject = %q", reader.Subject())
		}
		return nil
	})

	_, sets, deletes, _ := cache.snapshot()
	if sets != 1 || deletes != 0 {
		t.Fatalf("distributed cold burst should cache once without stale delete, sets=%d deletes=%d", sets, deletes)
	}
}

func TestCachedDistributedProviderStressParseRotateAndDelete(t *testing.T) {
	ctx := context.Background()
	repo := &fakeDistributedRepository{capacity: 256}
	provider, err := NewDistributedHMACProvider(ctx, repo, HS256,
		WithRepositoryCapacity(256),
		WithKeyIDGenerator(sequenceKID("distributed-stress")),
		WithEntropy(repeatingReader('s')),
	)
	if err != nil {
		t.Fatalf("NewDistributedHMACProvider() error = %v", err)
	}
	cache := newSpyReaderCache(time.Now)
	cached, err := NewCachedDistributedProvider(provider, cache, WithCacheTrustScope("distributed-stress-scope"))
	if err != nil {
		t.Fatalf("NewCachedDistributedProvider() error = %v", err)
	}
	var counter atomic.Int64

	tester := concurrencytest.NewGoroutineStressTester(concurrencytest.Options{
		Workers:       8,
		RoundsPerTask: 20,
		Timeout:       5 * time.Second,
	})
	tester.RunT(t,
		func(ctx context.Context) error {
			n := counter.Add(1)
			token, err := cached.ComposeContext(ctx, WithClaim("n", fmt.Sprintf("%d", n)), WithExpiresAfter(time.Hour))
			if err != nil {
				if errors.Is(err, ErrKeyNotFound) {
					return nil
				}
				return err
			}
			reader, err := cached.ParseContext(ctx, token)
			if err != nil {
				if errors.Is(err, ErrInvalidToken) || errors.Is(err, ErrKeyNotFound) {
					return nil
				}
				return err
			}
			if _, ok := reader.ClaimString("n"); !ok {
				return errorsNew("missing n claim")
			}
			return nil
		},
		func(ctx context.Context) error {
			_, err := cached.ForcedRotateContext(ctx)
			return err
		},
		func(ctx context.Context) error {
			if err := cached.DeleteKeyChainsContext(ctx); err != nil {
				return err
			}
			_, err := cached.RotateContext(ctx)
			return err
		},
	)
}
