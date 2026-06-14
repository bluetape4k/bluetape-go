package jwt

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/bluetape4k/bluetape-go/cache"
	golangjwt "github.com/golang-jwt/jwt/v5"
)

type spyReaderCache struct {
	mu         sync.Mutex
	values     map[string]*Reader
	expiresAt  map[string]time.Time
	gets       int
	sets       int
	setTTLs    []time.Duration
	deletes    int
	clears     int
	getErr     error
	setErr     error
	deleteErr  error
	clearErr   error
	setBlock   chan struct{}
	setRelease chan struct{}
	now        func() time.Time
}

func newSpyReaderCache(now func() time.Time) *spyReaderCache {
	if now == nil {
		now = time.Now
	}
	return &spyReaderCache{
		values:    make(map[string]*Reader),
		expiresAt: make(map[string]time.Time),
		now:       now,
	}
}

func (c *spyReaderCache) Get(ctx context.Context, key string) (*Reader, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.gets++
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if c.getErr != nil {
		return nil, c.getErr
	}
	value, ok := c.values[key]
	if !ok {
		return nil, cache.ErrCacheMiss
	}
	if expiresAt := c.expiresAt[key]; !expiresAt.IsZero() && !c.now().Before(expiresAt) {
		delete(c.values, key)
		delete(c.expiresAt, key)
		return nil, cache.ErrCacheMiss
	}
	return value, nil
}

func (c *spyReaderCache) Set(ctx context.Context, key string, value *Reader, ttl time.Duration) error {
	if c.setBlock != nil {
		select {
		case c.setBlock <- struct{}{}:
		default:
		}
		select {
		case <-c.setRelease:
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	c.sets++
	if err := ctx.Err(); err != nil {
		return err
	}
	if c.setErr != nil {
		return c.setErr
	}
	c.setTTLs = append(c.setTTLs, ttl)
	c.values[key] = value
	if ttl > 0 {
		c.expiresAt[key] = c.now().Add(ttl)
	} else {
		delete(c.expiresAt, key)
	}
	return nil
}

func (c *spyReaderCache) Delete(ctx context.Context, key string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.deletes++
	if err := ctx.Err(); err != nil {
		return err
	}
	if c.deleteErr != nil {
		return c.deleteErr
	}
	delete(c.values, key)
	delete(c.expiresAt, key)
	return nil
}

func (c *spyReaderCache) Clear(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.clears++
	if err := ctx.Err(); err != nil {
		return err
	}
	if c.clearErr != nil {
		return c.clearErr
	}
	clear(c.values)
	clear(c.expiresAt)
	return nil
}

func (c *spyReaderCache) snapshot() (gets int, sets int, deletes int, clears int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.gets, c.sets, c.deletes, c.clears
}

func (c *spyReaderCache) ttls() []time.Duration {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]time.Duration(nil), c.setTTLs...)
}

func TestNewCachedProviderValidation(t *testing.T) {
	provider := newTestFixedHMACProvider(t)
	cache := newSpyReaderCache(time.Now)

	if _, err := NewCachedProvider(nil, cache); !errors.Is(err, ErrInvalidOptions) {
		t.Fatalf("nil provider error = %v, want ErrInvalidOptions", err)
	}
	if _, err := NewCachedProvider(provider, nil); !errors.Is(err, ErrInvalidOptions) {
		t.Fatalf("nil cache error = %v, want ErrInvalidOptions", err)
	}
	if _, err := NewCachedProvider(provider, cache, nil); !errors.Is(err, ErrInvalidOptions) {
		t.Fatalf("nil option error = %v, want ErrInvalidOptions", err)
	}
	if _, err := NewCachedProvider(provider, cache, WithCacheMaxTTL(0)); !errors.Is(err, ErrInvalidOptions) {
		t.Fatalf("max ttl error = %v, want ErrInvalidOptions", err)
	}
	if _, err := NewCachedProvider(provider, cache, WithCacheKeyPrefix("")); !errors.Is(err, ErrInvalidOptions) {
		t.Fatalf("key prefix error = %v, want ErrInvalidOptions", err)
	}
	if _, err := NewCachedProvider(provider, cache, WithCacheTrustScope("bad\nscope")); !errors.Is(err, ErrInvalidOptions) {
		t.Fatalf("trust scope error = %v, want ErrInvalidOptions", err)
	}
	if _, err := NewCachedProvider(provider, cache, WithCacheClock(nil)); !errors.Is(err, ErrInvalidOptions) {
		t.Fatalf("clock error = %v, want ErrInvalidOptions", err)
	}
}

func TestCachedProviderCachesWarmHit(t *testing.T) {
	now := time.Date(2026, 6, 14, 1, 0, 0, 0, time.UTC)
	provider := newTestFixedHMACProviderAt(t, now)
	cache := newSpyReaderCache(func() time.Time { return now })
	cached, err := NewCachedProvider(provider, cache,
		WithCacheClock(func() time.Time { return now }),
		WithCacheTrustScope("test-scope"),
	)
	if err != nil {
		t.Fatalf("NewCachedProvider() error = %v", err)
	}
	token, err := provider.Compose(WithSubject("subject"), WithExpiresAfter(time.Hour))
	if err != nil {
		t.Fatalf("Compose() error = %v", err)
	}

	first, err := cached.Parse(token, WithExpectedSubject("subject"))
	if err != nil {
		t.Fatalf("first Parse() error = %v", err)
	}
	second, err := cached.Parse(token, WithExpectedSubject("subject"))
	if err != nil {
		t.Fatalf("second Parse() error = %v", err)
	}
	if first != second {
		t.Fatalf("warm hit should return cached Reader pointer")
	}
	gets, sets, deletes, _ := cache.snapshot()
	if gets < 2 || sets != 1 || deletes != 0 {
		t.Fatalf("cache operations gets=%d sets=%d deletes=%d, want warm hit", gets, sets, deletes)
	}
}

func TestCachedProviderDefaultCacheClockUsesProviderClock(t *testing.T) {
	now := time.Date(2000, 1, 1, 1, 0, 0, 0, time.UTC)
	provider := newTestFixedHMACProviderAt(t, now)
	cache := newSpyReaderCache(time.Now)
	cached, err := NewCachedProvider(provider, cache, WithCacheTrustScope("provider-clock-scope"))
	if err != nil {
		t.Fatalf("NewCachedProvider() error = %v", err)
	}
	token, err := provider.Compose(WithExpiresAfter(time.Hour))
	if err != nil {
		t.Fatalf("Compose() error = %v", err)
	}

	if _, err := cached.Parse(token); err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	_, sets, _, _ := cache.snapshot()
	if sets != 1 {
		t.Fatalf("default cache clock should follow provider clock, sets=%d", sets)
	}
}

func TestCachedProviderWithParseClockBypassesCache(t *testing.T) {
	now := time.Date(2026, 6, 14, 1, 0, 0, 0, time.UTC)
	provider := newTestFixedHMACProviderAt(t, now)
	cache := newSpyReaderCache(func() time.Time { return now })
	cached, err := NewCachedProvider(provider, cache, WithCacheClock(func() time.Time { return now }))
	if err != nil {
		t.Fatalf("NewCachedProvider() error = %v", err)
	}
	token, err := provider.Compose(WithExpiresAfter(time.Hour))
	if err != nil {
		t.Fatalf("Compose() error = %v", err)
	}
	if _, err := cached.Parse(token, WithParseClock(func() time.Time { return now })); err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	gets, sets, deletes, _ := cache.snapshot()
	if gets != 0 || sets != 0 || deletes != 0 {
		t.Fatalf("custom clock should bypass cache, got gets=%d sets=%d deletes=%d", gets, sets, deletes)
	}
}

func TestCachedProviderRejectsDoneContextBeforeCacheAccess(t *testing.T) {
	provider := newTestFixedHMACProvider(t)
	cache := newSpyReaderCache(time.Now)
	cached, err := NewCachedProvider(provider, cache)
	if err != nil {
		t.Fatalf("NewCachedProvider() error = %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := cached.ParseContext(ctx, "token"); !errors.Is(err, context.Canceled) {
		t.Fatalf("ParseContext() error = %v, want context.Canceled", err)
	}
	gets, sets, deletes, clears := cache.snapshot()
	if gets != 0 || sets != 0 || deletes != 0 || clears != 0 {
		t.Fatalf("done context should not touch cache, got %d/%d/%d/%d", gets, sets, deletes, clears)
	}
}

func TestCachedProviderRejectsEmptyTokenBeforeCacheAccess(t *testing.T) {
	provider := newTestFixedHMACProvider(t)
	cache := newSpyReaderCache(time.Now)
	cached, err := NewCachedProvider(provider, cache)
	if err != nil {
		t.Fatalf("NewCachedProvider() error = %v", err)
	}
	if _, err := cached.Parse(""); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("Parse(empty) error = %v, want ErrInvalidToken", err)
	}
	gets, sets, deletes, clears := cache.snapshot()
	if gets != 0 || sets != 0 || deletes != 0 || clears != 0 {
		t.Fatalf("empty token should not touch cache, got %d/%d/%d/%d", gets, sets, deletes, clears)
	}
}

func TestCachedProviderCacheGetFailureBlocksParse(t *testing.T) {
	cause := errorsNew("cache unavailable")
	provider := newTestFixedHMACProvider(t)
	cache := newSpyReaderCache(time.Now)
	cache.getErr = cause
	cached, err := NewCachedProvider(provider, cache)
	if err != nil {
		t.Fatalf("NewCachedProvider() error = %v", err)
	}
	token, err := provider.Compose(WithExpiresAfter(time.Hour))
	if err != nil {
		t.Fatalf("Compose() error = %v", err)
	}
	if _, err := cached.Parse(token); !errors.Is(err, cause) {
		t.Fatalf("Parse() error = %v, want cache cause", err)
	}
	_, sets, _, _ := cache.snapshot()
	if sets != 0 {
		t.Fatalf("get failure should block provider parse and set, sets=%d", sets)
	}
}

func TestCachedProviderSetFailureIsVisible(t *testing.T) {
	cause := errorsNew("set failed")
	provider := newTestFixedHMACProvider(t)
	cache := newSpyReaderCache(time.Now)
	cache.setErr = cause
	cached, err := NewCachedProvider(provider, cache)
	if err != nil {
		t.Fatalf("NewCachedProvider() error = %v", err)
	}
	token, err := provider.Compose(WithExpiresAfter(time.Hour))
	if err != nil {
		t.Fatalf("Compose() error = %v", err)
	}
	if _, err := cached.Parse(token); !errors.Is(err, cause) {
		t.Fatalf("Parse() error = %v, want set cause", err)
	}
}

func TestCachedProviderParseFailureDoesNotCache(t *testing.T) {
	now := time.Date(2026, 6, 14, 1, 0, 0, 0, time.UTC)
	provider := newTestFixedHMACProviderAt(t, now)
	wrongKeyProvider, err := NewFixedHMACProvider(HS256, bytesOf('b', 32),
		WithClock(func() time.Time { return now }),
		WithKeyIDGenerator(staticKID("kid-1")),
	)
	if err != nil {
		t.Fatalf("wrong key provider error = %v", err)
	}
	valid, err := provider.Compose(WithExpiresAfter(time.Hour))
	if err != nil {
		t.Fatalf("Compose(valid) error = %v", err)
	}
	wrongAlg := golangjwt.NewWithClaims(golangjwt.SigningMethodHS384, golangjwt.MapClaims{"iat": golangjwt.NewNumericDate(now)})
	wrongAlg.Header["kid"] = "kid-1"
	wrongAlgToken, err := wrongAlg.SignedString(bytesOf('a', 48))
	if err != nil {
		t.Fatalf("wrongAlg SignedString() error = %v", err)
	}

	tests := []struct {
		name     string
		provider *Provider
		token    string
	}{
		{name: "malformed", provider: provider, token: "malformed.token.value"},
		{name: "wrong key", provider: wrongKeyProvider, token: valid},
		{name: "wrong algorithm", provider: provider, token: wrongAlgToken},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cache := newSpyReaderCache(func() time.Time { return now })
			cached, err := NewCachedProvider(tt.provider, cache, WithCacheClock(func() time.Time { return now }))
			if err != nil {
				t.Fatalf("NewCachedProvider() error = %v", err)
			}
			if _, err := cached.Parse(tt.token); !errors.Is(err, ErrInvalidToken) {
				t.Fatalf("Parse(%s) error = %v, want ErrInvalidToken", tt.name, err)
			}
			_, sets, _, _ := cache.snapshot()
			if sets != 0 {
				t.Fatalf("parse failure must not cache, sets=%d", sets)
			}
		})
	}
}

func TestCachedProviderTTLClippingAndNonPositiveSkip(t *testing.T) {
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
			provider, err := NewFixedHMACProvider(HS256, bytesOf('a', 32),
				WithClock(func() time.Time { return now }),
				WithKeyIDGenerator(staticKID("kid-1")),
				WithKeyTTL(tt.keyTTL),
			)
			if err != nil {
				t.Fatalf("NewFixedHMACProvider() error = %v", err)
			}
			cache := newSpyReaderCache(func() time.Time { return now })
			cached, err := NewCachedProvider(provider, cache,
				WithCacheClock(func() time.Time { return now }),
				WithCacheMaxTTL(tt.maxTTL),
			)
			if err != nil {
				t.Fatalf("NewCachedProvider() error = %v", err)
			}
			token, err := provider.Compose(WithExpiresAt(now.Add(tt.tokenTTL)))
			if err != nil {
				t.Fatalf("Compose() error = %v", err)
			}
			_, err = cached.Parse(token)
			if tt.wantSet && err != nil {
				t.Fatalf("Parse() error = %v", err)
			}
			if !tt.wantSet && !errors.Is(err, ErrExpiredToken) {
				t.Fatalf("Parse() error = %v, want ErrExpiredToken", err)
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

func TestCachedProviderStaleHitsDeleteAndReparse(t *testing.T) {
	now := time.Date(2026, 6, 14, 1, 0, 0, 0, time.UTC)

	tests := []struct {
		name     string
		reader   *Reader
		cacheNow time.Time
		wantSets int
	}{
		{name: "nil reader", reader: nil, cacheNow: now, wantSets: 1},
		{name: "wrong algorithm", reader: &Reader{kid: "kid-1", algorithm: RS256}, cacheNow: now, wantSets: 1},
		{name: "unknown kid", reader: &Reader{kid: "unknown", algorithm: HS256}, cacheNow: now, wantSets: 1},
		{name: "expired key skips recache", reader: &Reader{kid: "kid-1", algorithm: HS256}, cacheNow: now.Add(2 * time.Minute), wantSets: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider, err := NewFixedHMACProvider(HS256, bytesOf('a', 32),
				WithClock(func() time.Time { return now }),
				WithKeyIDGenerator(staticKID("kid-1")),
				WithKeyTTL(time.Minute),
			)
			if err != nil {
				t.Fatalf("NewFixedHMACProvider() error = %v", err)
			}
			cache := newSpyReaderCache(func() time.Time { return tt.cacheNow })
			cached, err := NewCachedProvider(provider, cache,
				WithCacheClock(func() time.Time { return tt.cacheNow }),
				WithCacheTrustScope("stale-branch-scope"),
			)
			if err != nil {
				t.Fatalf("NewCachedProvider() error = %v", err)
			}
			token, err := provider.Compose(WithExpiresAfter(time.Hour))
			if err != nil {
				t.Fatalf("Compose() error = %v", err)
			}
			parse, err := normalizeParseConfig(provider.now, nil)
			if err != nil {
				t.Fatalf("normalizeParseConfig() error = %v", err)
			}
			key := buildCacheProfile(provider.algorithm, cached.cfg, parse, token).key
			cache.values[key] = tt.reader

			reader, err := cached.Parse(token)
			if err != nil {
				t.Fatalf("Parse() error = %v", err)
			}
			if reader == nil || reader.Kid() != "kid-1" {
				t.Fatalf("reader kid = %q, want kid-1", reader.Kid())
			}
			_, sets, deletes, _ := cache.snapshot()
			if deletes != 1 {
				t.Fatalf("stale hit deletes = %d, want 1", deletes)
			}
			if sets != tt.wantSets {
				t.Fatalf("sets = %d, want %d", sets, tt.wantSets)
			}
		})
	}
}

func TestCachedProviderStaleDeleteFailureIsVisible(t *testing.T) {
	now := time.Date(2026, 6, 14, 1, 0, 0, 0, time.UTC)
	provider := newTestFixedHMACProviderAt(t, now)
	cache := newSpyReaderCache(func() time.Time { return now })
	cached, err := NewCachedProvider(provider, cache,
		WithCacheClock(func() time.Time { return now }),
		WithCacheTrustScope("stale-scope"),
	)
	if err != nil {
		t.Fatalf("NewCachedProvider() error = %v", err)
	}
	token, err := provider.Compose(WithExpiresAfter(time.Hour))
	if err != nil {
		t.Fatalf("Compose() error = %v", err)
	}
	parse, err := normalizeParseConfig(provider.now, nil)
	if err != nil {
		t.Fatalf("normalizeParseConfig() error = %v", err)
	}
	key := buildCacheProfile(provider.algorithm, cached.cfg, parse, token).key
	cache.values[key] = &Reader{kid: "kid-1", algorithm: RS256}
	cause := errorsNew("delete failed")
	cache.deleteErr = cause

	reader, err := cached.Parse(token)
	if !errors.Is(err, cause) {
		t.Fatalf("Parse() error = %v, want delete cause", err)
	}
	if reader != nil {
		t.Fatalf("delete failure must not return stale reader")
	}
}

func TestCachedProviderLiveWaiterRetriesAfterCanceledOwner(t *testing.T) {
	provider := newTestFixedHMACProvider(t)
	cache := newSpyReaderCache(time.Now)
	block := make(chan struct{}, 1)
	cache.setBlock = block
	cache.setRelease = make(chan struct{})
	cached, err := NewCachedProvider(provider, cache)
	if err != nil {
		t.Fatalf("NewCachedProvider() error = %v", err)
	}
	token, err := provider.Compose(WithExpiresAfter(time.Hour), WithSubject("subject"))
	if err != nil {
		t.Fatalf("Compose() error = %v", err)
	}
	ownerCtx, ownerCancel := context.WithCancel(context.Background())
	ownerErr := make(chan error, 1)
	go func() {
		_, err := cached.ParseContext(ownerCtx, token, WithExpectedSubject("subject"))
		ownerErr <- err
	}()
	<-block
	liveErr := make(chan error, 1)
	go func() {
		reader, err := cached.ParseContext(context.Background(), token, WithExpectedSubject("subject"))
		if err != nil {
			liveErr <- err
			return
		}
		if reader.Subject() != "subject" {
			liveErr <- errorsNew("subject mismatch")
			return
		}
		liveErr <- nil
	}()
	ownerCancel()
	if err := <-ownerErr; !errors.Is(err, context.Canceled) {
		t.Fatalf("owner error = %v, want context.Canceled", err)
	}
	close(cache.setRelease)
	if err := <-liveErr; err != nil {
		t.Fatalf("live waiter retry ParseContext() error = %v", err)
	}
}

func TestCachedProviderClearPreventsInFlightSet(t *testing.T) {
	provider := newTestFixedHMACProvider(t)
	cache := newSpyReaderCache(time.Now)
	cached, err := NewCachedProvider(provider, cache)
	if err != nil {
		t.Fatalf("NewCachedProvider() error = %v", err)
	}
	token, err := provider.Compose(WithExpiresAfter(time.Hour))
	if err != nil {
		t.Fatalf("Compose() error = %v", err)
	}
	cache.setBlock = make(chan struct{}, 1)
	cache.setRelease = make(chan struct{})
	parseDone := make(chan error, 1)
	go func() {
		_, err := cached.Parse(token)
		parseDone <- err
	}()
	<-cache.setBlock
	clearDone := make(chan error, 1)
	go func() {
		clearDone <- cached.ClearCache(context.Background())
	}()
	close(cache.setRelease)
	if err := <-parseDone; err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if err := <-clearDone; err != nil {
		t.Fatalf("ClearCache() error = %v", err)
	}
	if len(cache.values) != 0 {
		t.Fatalf("clear racing with in-flight set should leave cache empty, entries=%d", len(cache.values))
	}
}

func TestCachedProviderForcedRotateClearFailureIsVisible(t *testing.T) {
	provider, err := NewHMACProvider(HS256,
		WithKeyIDGenerator(sequenceKID("rotate-cache")),
		WithEntropy(repeatingReader('r')),
	)
	if err != nil {
		t.Fatalf("NewHMACProvider() error = %v", err)
	}
	cache := newSpyReaderCache(time.Now)
	cause := errorsNew("clear failed")
	cache.clearErr = cause
	cached, err := NewCachedProvider(provider, cache)
	if err != nil {
		t.Fatalf("NewCachedProvider() error = %v", err)
	}
	if _, err := cached.ForcedRotate(); !errors.Is(err, cause) {
		t.Fatalf("ForcedRotate() error = %v, want clear cause", err)
	}
}

func newTestFixedHMACProvider(t *testing.T) *Provider {
	t.Helper()
	return newTestFixedHMACProviderAt(t, time.Date(2026, 6, 14, 1, 0, 0, 0, time.UTC))
}

func newTestFixedHMACProviderAt(t *testing.T, now time.Time) *Provider {
	t.Helper()
	provider, err := NewFixedHMACProvider(HS256, bytesOf('a', 32),
		WithClock(func() time.Time { return now }),
		WithKeyIDGenerator(staticKID("kid-1")),
	)
	if err != nil {
		t.Fatalf("NewFixedHMACProvider() error = %v", err)
	}
	return provider
}
