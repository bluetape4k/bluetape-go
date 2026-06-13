package jwt

import (
	"context"
	"errors"
	"testing"
	"time"

	golangjwt "github.com/golang-jwt/jwt/v5"
)

func TestCachedDistributedProviderCachesWarmHitWithKeyRevalidation(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 6, 14, 1, 0, 0, 0, time.UTC)
	repo := &fakeDistributedRepository{}
	provider, err := NewDistributedHMACProvider(ctx, repo, HS256,
		WithClock(func() time.Time { return now }),
		WithKeyIDGenerator(staticKID("distributed-cache-1")),
		WithEntropy(repeatingReader('d')),
	)
	if err != nil {
		t.Fatalf("NewDistributedHMACProvider() error = %v", err)
	}
	cache := newSpyReaderCache(func() time.Time { return now })
	cached, err := NewCachedDistributedProvider(provider, cache,
		WithCacheClock(func() time.Time { return now }),
		WithCacheTrustScope("distributed-scope"),
	)
	if err != nil {
		t.Fatalf("NewCachedDistributedProvider() error = %v", err)
	}
	token, err := provider.ComposeContext(ctx, WithSubject("subject"), WithExpiresAfter(time.Hour))
	if err != nil {
		t.Fatalf("ComposeContext() error = %v", err)
	}

	first, err := cached.ParseContext(ctx, token, WithExpectedSubject("subject"))
	if err != nil {
		t.Fatalf("first ParseContext() error = %v", err)
	}
	_, _, _, _, _ = repo.snapshot()
	second, err := cached.ParseContext(ctx, token, WithExpectedSubject("subject"))
	if err != nil {
		t.Fatalf("second ParseContext() error = %v", err)
	}
	if first != second {
		t.Fatalf("warm hit should return cached Reader pointer")
	}
	gets, sets, deletes, _ := cache.snapshot()
	if gets < 2 || sets != 1 || deletes != 0 {
		t.Fatalf("cache operations gets=%d sets=%d deletes=%d, want warm hit", gets, sets, deletes)
	}
	repo.mu.Lock()
	findHits := repo.findHits
	repo.mu.Unlock()
	if findHits < 3 {
		t.Fatalf("distributed warm hit should revalidate key, findHits=%d", findHits)
	}
}

func TestNewCachedDistributedProviderValidation(t *testing.T) {
	ctx := context.Background()
	repo := &fakeDistributedRepository{}
	provider, err := NewDistributedHMACProvider(ctx, repo, HS256,
		WithKeyIDGenerator(staticKID("distributed-validation")),
		WithEntropy(repeatingReader('v')),
	)
	if err != nil {
		t.Fatalf("NewDistributedHMACProvider() error = %v", err)
	}
	cache := newSpyReaderCache(time.Now)

	if _, err := NewCachedDistributedProvider(nil, cache); !errors.Is(err, ErrInvalidOptions) {
		t.Fatalf("nil provider error = %v, want ErrInvalidOptions", err)
	}
	var zero DistributedProvider
	if _, err := NewCachedDistributedProvider(&zero, cache); !errors.Is(err, ErrInvalidOptions) {
		t.Fatalf("zero provider error = %v, want ErrInvalidOptions", err)
	}
	if _, err := NewCachedDistributedProvider(provider, nil); !errors.Is(err, ErrInvalidOptions) {
		t.Fatalf("nil cache error = %v, want ErrInvalidOptions", err)
	}
	var typedNil *spyReaderCache
	if _, err := NewCachedDistributedProvider(provider, typedNil); !errors.Is(err, ErrInvalidOptions) {
		t.Fatalf("typed nil cache error = %v, want ErrInvalidOptions", err)
	}
	if _, err := NewCachedDistributedProvider(provider, cache, nil); !errors.Is(err, ErrInvalidOptions) {
		t.Fatalf("nil option error = %v, want ErrInvalidOptions", err)
	}
	if _, err := NewCachedDistributedProvider(provider, cache, WithCacheMaxTTL(0)); !errors.Is(err, ErrInvalidOptions) {
		t.Fatalf("invalid ttl error = %v, want ErrInvalidOptions", err)
	}
}

func TestCachedDistributedProviderRejectsDoneContextBeforeCacheAccess(t *testing.T) {
	ctx := context.Background()
	repo := &fakeDistributedRepository{}
	provider, err := NewDistributedHMACProvider(ctx, repo, HS256,
		WithKeyIDGenerator(staticKID("distributed-cache-context")),
		WithEntropy(repeatingReader('c')),
	)
	if err != nil {
		t.Fatalf("NewDistributedHMACProvider() error = %v", err)
	}
	cache := newSpyReaderCache(time.Now)
	cached, err := NewCachedDistributedProvider(provider, cache)
	if err != nil {
		t.Fatalf("NewCachedDistributedProvider() error = %v", err)
	}
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := cached.ParseContext(canceled, "token"); !errors.Is(err, context.Canceled) {
		t.Fatalf("ParseContext() error = %v, want context.Canceled", err)
	}
	gets, sets, deletes, clears := cache.snapshot()
	if gets != 0 || sets != 0 || deletes != 0 || clears != 0 {
		t.Fatalf("done context should not touch cache, got %d/%d/%d/%d", gets, sets, deletes, clears)
	}
}

func TestCachedDistributedProviderRejectsEmptyTokenBeforeCacheAccess(t *testing.T) {
	ctx := context.Background()
	repo := &fakeDistributedRepository{}
	provider, err := NewDistributedHMACProvider(ctx, repo, HS256,
		WithKeyIDGenerator(staticKID("distributed-cache-empty")),
		WithEntropy(repeatingReader('e')),
	)
	if err != nil {
		t.Fatalf("NewDistributedHMACProvider() error = %v", err)
	}
	cache := newSpyReaderCache(time.Now)
	cached, err := NewCachedDistributedProvider(provider, cache)
	if err != nil {
		t.Fatalf("NewCachedDistributedProvider() error = %v", err)
	}
	if _, err := cached.ParseContext(ctx, ""); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("ParseContext(empty) error = %v, want ErrInvalidToken", err)
	}
	gets, sets, deletes, clears := cache.snapshot()
	if gets != 0 || sets != 0 || deletes != 0 || clears != 0 {
		t.Fatalf("empty token should not touch cache, got %d/%d/%d/%d", gets, sets, deletes, clears)
	}
}

func TestCachedDistributedProviderParseFailureDoesNotCache(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 6, 14, 1, 0, 0, 0, time.UTC)
	repo := &fakeDistributedRepository{}
	provider, err := NewDistributedHMACProvider(ctx, repo, HS256,
		WithClock(func() time.Time { return now }),
		WithKeyIDGenerator(staticKID("distributed-cache-failure")),
		WithEntropy(repeatingReader('f')),
	)
	if err != nil {
		t.Fatalf("NewDistributedHMACProvider() error = %v", err)
	}
	wrongKey := golangjwt.NewWithClaims(golangjwt.SigningMethodHS256, golangjwt.MapClaims{"iat": golangjwt.NewNumericDate(now)})
	wrongKey.Header["kid"] = "distributed-cache-failure"
	wrongKeyToken, err := wrongKey.SignedString(bytesOf('x', 32))
	if err != nil {
		t.Fatalf("wrongKey SignedString() error = %v", err)
	}
	wrongAlg := golangjwt.NewWithClaims(golangjwt.SigningMethodHS384, golangjwt.MapClaims{"iat": golangjwt.NewNumericDate(now)})
	wrongAlg.Header["kid"] = "distributed-cache-failure"
	wrongAlgToken, err := wrongAlg.SignedString(bytesOf('f', 48))
	if err != nil {
		t.Fatalf("wrongAlg SignedString() error = %v", err)
	}

	tests := []struct {
		name  string
		token string
	}{
		{name: "malformed", token: "malformed.token.value"},
		{name: "wrong key", token: wrongKeyToken},
		{name: "wrong algorithm", token: wrongAlgToken},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cache := newSpyReaderCache(func() time.Time { return now })
			cached, err := NewCachedDistributedProvider(provider, cache, WithCacheClock(func() time.Time { return now }))
			if err != nil {
				t.Fatalf("NewCachedDistributedProvider() error = %v", err)
			}
			if _, err := cached.ParseContext(ctx, tt.token); !errors.Is(err, ErrInvalidToken) {
				t.Fatalf("ParseContext(%s) error = %v, want ErrInvalidToken", tt.name, err)
			}
			_, sets, _, _ := cache.snapshot()
			if sets != 0 {
				t.Fatalf("parse failure must not cache, sets=%d", sets)
			}
		})
	}
}

func TestCachedDistributedProviderTTLClippingAndNonPositiveSkip(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 6, 14, 1, 0, 0, 0, time.UTC)

	tests := []struct {
		name       string
		keyTTL     time.Duration
		tokenTTL   time.Duration
		maxTTL     time.Duration
		wantSet    bool
		wantSetTTL time.Duration
	}{
		{name: "max ttl clips", keyTTL: time.Hour, tokenTTL: time.Hour, maxTTL: 10 * time.Minute, wantSet: true, wantSetTTL: 10 * time.Minute},
		{name: "token ttl clips", keyTTL: time.Hour, tokenTTL: 3 * time.Minute, maxTTL: 10 * time.Minute, wantSet: true, wantSetTTL: 3 * time.Minute},
		{name: "key ttl clips", keyTTL: 4 * time.Minute, tokenTTL: time.Hour, maxTTL: 10 * time.Minute, wantSet: true, wantSetTTL: 4 * time.Minute},
		{name: "expired token skips set", keyTTL: time.Hour, tokenTTL: -time.Minute, maxTTL: 10 * time.Minute, wantSet: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &fakeDistributedRepository{}
			provider, err := NewDistributedHMACProvider(ctx, repo, HS256,
				WithClock(func() time.Time { return now }),
				WithKeyIDGenerator(staticKID("distributed-ttl")),
				WithEntropy(repeatingReader('t')),
				WithKeyTTL(tt.keyTTL),
			)
			if err != nil {
				t.Fatalf("NewDistributedHMACProvider() error = %v", err)
			}
			cache := newSpyReaderCache(func() time.Time { return now })
			cached, err := NewCachedDistributedProvider(provider, cache,
				WithCacheClock(func() time.Time { return now }),
				WithCacheMaxTTL(tt.maxTTL),
			)
			if err != nil {
				t.Fatalf("NewCachedDistributedProvider() error = %v", err)
			}
			token, err := provider.ComposeContext(ctx, WithExpiresAt(now.Add(tt.tokenTTL)))
			if err != nil {
				t.Fatalf("ComposeContext() error = %v", err)
			}
			_, err = cached.ParseContext(ctx, token)
			if tt.wantSet && err != nil {
				t.Fatalf("ParseContext() error = %v", err)
			}
			if !tt.wantSet && !errors.Is(err, ErrExpiredToken) {
				t.Fatalf("ParseContext() error = %v, want ErrExpiredToken", err)
			}
			ttls := cache.ttls()
			if !tt.wantSet {
				if len(ttls) != 0 {
					t.Fatalf("expired token must not cache, ttls=%v", ttls)
				}
				return
			}
			if len(ttls) != 1 || ttls[0] != tt.wantSetTTL {
				t.Fatalf("set TTLs = %v, want [%s]", ttls, tt.wantSetTTL)
			}
		})
	}
}

func TestCachedDistributedProviderStaleHitsDeleteAndReparse(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 6, 14, 1, 0, 0, 0, time.UTC)

	tests := []struct {
		name   string
		reader *Reader
	}{
		{name: "nil reader", reader: nil},
		{name: "wrong algorithm", reader: &Reader{kid: "distributed-stale", algorithm: RS256}},
		{name: "unknown kid", reader: &Reader{kid: "unknown", algorithm: HS256}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &fakeDistributedRepository{}
			provider, err := NewDistributedHMACProvider(ctx, repo, HS256,
				WithClock(func() time.Time { return now }),
				WithKeyIDGenerator(staticKID("distributed-stale")),
				WithEntropy(repeatingReader('s')),
			)
			if err != nil {
				t.Fatalf("NewDistributedHMACProvider() error = %v", err)
			}
			cache := newSpyReaderCache(func() time.Time { return now })
			cached, err := NewCachedDistributedProvider(provider, cache,
				WithCacheClock(func() time.Time { return now }),
				WithCacheTrustScope("distributed-stale-branch-scope"),
			)
			if err != nil {
				t.Fatalf("NewCachedDistributedProvider() error = %v", err)
			}
			token, err := provider.ComposeContext(ctx, WithExpiresAfter(time.Hour))
			if err != nil {
				t.Fatalf("ComposeContext() error = %v", err)
			}
			parse, err := normalizeParseConfig(provider.provider.now, nil)
			if err != nil {
				t.Fatalf("normalizeParseConfig() error = %v", err)
			}
			key := buildCacheProfile(provider.provider.algorithm, cached.cfg, parse, token).key
			cache.values[key] = tt.reader

			reader, err := cached.ParseContext(ctx, token)
			if err != nil {
				t.Fatalf("ParseContext() error = %v", err)
			}
			if reader == nil || reader.Kid() != "distributed-stale" {
				t.Fatalf("reader kid = %q, want distributed-stale", reader.Kid())
			}
			_, sets, deletes, _ := cache.snapshot()
			if deletes != 1 {
				t.Fatalf("stale hit deletes = %d, want 1", deletes)
			}
			if sets != 1 {
				t.Fatalf("sets = %d, want 1", sets)
			}
		})
	}
}

func TestCachedDistributedProviderStaleDeleteFailureIsVisible(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 6, 14, 1, 0, 0, 0, time.UTC)
	repo := &fakeDistributedRepository{}
	provider, err := NewDistributedHMACProvider(ctx, repo, HS256,
		WithClock(func() time.Time { return now }),
		WithKeyIDGenerator(staticKID("distributed-stale-delete")),
		WithEntropy(repeatingReader('s')),
	)
	if err != nil {
		t.Fatalf("NewDistributedHMACProvider() error = %v", err)
	}
	token, err := provider.ComposeContext(ctx, WithExpiresAfter(time.Hour))
	if err != nil {
		t.Fatalf("ComposeContext() error = %v", err)
	}
	cache := newSpyReaderCache(func() time.Time { return now })
	cached, err := NewCachedDistributedProvider(provider, cache,
		WithCacheClock(func() time.Time { return now }),
		WithCacheTrustScope("distributed-stale-scope"),
	)
	if err != nil {
		t.Fatalf("NewCachedDistributedProvider() error = %v", err)
	}
	parse, err := normalizeParseConfig(provider.provider.now, nil)
	if err != nil {
		t.Fatalf("normalizeParseConfig() error = %v", err)
	}
	key := buildCacheProfile(provider.provider.algorithm, cached.cfg, parse, token).key
	cache.values[key] = &Reader{kid: "distributed-stale-delete", algorithm: RS256}
	cause := errorsNew("delete failed")
	cache.deleteErr = cause
	reader, err := cached.ParseContext(ctx, token)
	if !errors.Is(err, cause) {
		t.Fatalf("ParseContext() error = %v, want delete cause", err)
	}
	if reader != nil {
		t.Fatalf("delete failure must not return stale reader")
	}
}

func TestCachedDistributedProviderClearFailuresAreVisible(t *testing.T) {
	ctx := context.Background()
	repo := &fakeDistributedRepository{}
	provider, err := NewDistributedHMACProvider(ctx, repo, HS256,
		WithKeyIDGenerator(sequenceKID("distributed-clear")),
		WithEntropy(repeatingReader('q')),
	)
	if err != nil {
		t.Fatalf("NewDistributedHMACProvider() error = %v", err)
	}
	cache := newSpyReaderCache(time.Now)
	cause := errorsNew("clear failed")
	cache.clearErr = cause
	cached, err := NewCachedDistributedProvider(provider, cache)
	if err != nil {
		t.Fatalf("NewCachedDistributedProvider() error = %v", err)
	}
	if _, err := cached.ForcedRotateContext(ctx); !errors.Is(err, cause) {
		t.Fatalf("ForcedRotateContext() error = %v, want clear cause", err)
	}
	if err := cached.DeleteKeyChainsContext(ctx); !errors.Is(err, cause) {
		t.Fatalf("DeleteKeyChainsContext() error = %v, want clear cause", err)
	}
}
