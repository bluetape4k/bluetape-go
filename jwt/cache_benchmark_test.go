package jwt

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"testing"
	"time"
)

func BenchmarkProviderParseHMAC(b *testing.B) {
	now := time.Date(2026, 6, 14, 1, 0, 0, 0, time.UTC)
	provider := newBenchmarkFixedHMACProvider(b, now)
	token := newBenchmarkToken(b, provider)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := provider.Parse(token); err != nil {
			b.Fatalf("Parse() error = %v", err)
		}
	}
}

func BenchmarkCachedProviderParseHMACColdMiss(b *testing.B) {
	now := time.Date(2026, 6, 14, 1, 0, 0, 0, time.UTC)
	provider := newBenchmarkFixedHMACProvider(b, now)
	token := newBenchmarkToken(b, provider)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cache := newSpyReaderCache(func() time.Time { return now })
		cached, err := NewCachedProvider(provider, cache, WithCacheClock(func() time.Time { return now }))
		if err != nil {
			b.Fatalf("NewCachedProvider() error = %v", err)
		}
		if _, err := cached.Parse(token); err != nil {
			b.Fatalf("Parse() error = %v", err)
		}
	}
}

func BenchmarkCachedProviderParseHMACWarmHit(b *testing.B) {
	now := time.Date(2026, 6, 14, 1, 0, 0, 0, time.UTC)
	provider := newBenchmarkFixedHMACProvider(b, now)
	token := newBenchmarkToken(b, provider)
	cache := newSpyReaderCache(func() time.Time { return now })
	cached, err := NewCachedProvider(provider, cache, WithCacheClock(func() time.Time { return now }))
	if err != nil {
		b.Fatalf("NewCachedProvider() error = %v", err)
	}
	if _, err := cached.Parse(token); err != nil {
		b.Fatalf("warmup Parse() error = %v", err)
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := cached.Parse(token); err != nil {
			b.Fatalf("Parse() error = %v", err)
		}
	}
}

func BenchmarkCachedProviderParseRSAWarmHit(b *testing.B) {
	now := time.Date(2026, 6, 14, 1, 0, 0, 0, time.UTC)
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		b.Fatalf("GenerateKey() error = %v", err)
	}
	provider, err := NewFixedRSAProvider(RS256, privateKey,
		WithClock(func() time.Time { return now }),
		WithKeyIDGenerator(staticKID("bench-rsa")),
	)
	if err != nil {
		b.Fatalf("NewFixedRSAProvider() error = %v", err)
	}
	token := newBenchmarkToken(b, provider)
	cache := newSpyReaderCache(func() time.Time { return now })
	cached, err := NewCachedProvider(provider, cache, WithCacheClock(func() time.Time { return now }))
	if err != nil {
		b.Fatalf("NewCachedProvider() error = %v", err)
	}
	if _, err := cached.Parse(token); err != nil {
		b.Fatalf("warmup Parse() error = %v", err)
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := cached.Parse(token); err != nil {
			b.Fatalf("Parse() error = %v", err)
		}
	}
}

func BenchmarkCachedDistributedProviderParseHMACWarmHit(b *testing.B) {
	ctx := context.Background()
	now := time.Date(2026, 6, 14, 1, 0, 0, 0, time.UTC)
	repo := &fakeDistributedRepository{}
	provider, err := NewDistributedHMACProvider(ctx, repo, HS256,
		WithClock(func() time.Time { return now }),
		WithKeyIDGenerator(staticKID("bench-distributed")),
		WithEntropy(repeatingReader('d')),
	)
	if err != nil {
		b.Fatalf("NewDistributedHMACProvider() error = %v", err)
	}
	token, err := provider.ComposeContext(ctx, WithSubject("subject"), WithExpiresAfter(time.Hour))
	if err != nil {
		b.Fatalf("ComposeContext() error = %v", err)
	}
	cache := newSpyReaderCache(func() time.Time { return now })
	cached, err := NewCachedDistributedProvider(provider, cache, WithCacheClock(func() time.Time { return now }))
	if err != nil {
		b.Fatalf("NewCachedDistributedProvider() error = %v", err)
	}
	if _, err := cached.ParseContext(ctx, token, WithExpectedSubject("subject")); err != nil {
		b.Fatalf("warmup ParseContext() error = %v", err)
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := cached.ParseContext(ctx, token, WithExpectedSubject("subject")); err != nil {
			b.Fatalf("ParseContext() error = %v", err)
		}
	}
}

func BenchmarkCacheKeyProfile(b *testing.B) {
	now := time.Date(2026, 6, 14, 1, 0, 0, 0, time.UTC)
	parse, err := normalizeParseConfig(func() time.Time { return now }, []ParseOption{
		WithExpectedIssuer("issuer"),
		WithExpectedAudience("api", "worker"),
		WithExpectedSubject("subject"),
		WithExpirationRequired(),
	})
	if err != nil {
		b.Fatalf("normalizeParseConfig() error = %v", err)
	}
	cfg := cacheConfig{
		maxTTL:     defaultProviderCacheMaxTTL,
		keyPrefix:  defaultProviderCacheKeyPrefix,
		trustScope: "bench-scope",
		now:        time.Now,
	}

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if profile := buildCacheProfile(HS256, cfg, parse, "header.payload.signature"); !profile.cacheable {
			b.Fatalf("profile should be cacheable")
		}
	}
}

func newBenchmarkFixedHMACProvider(b *testing.B, now time.Time) *Provider {
	b.Helper()
	provider, err := NewFixedHMACProvider(HS256, bytesOf('b', 32),
		WithClock(func() time.Time { return now }),
		WithKeyIDGenerator(staticKID("bench-kid")),
	)
	if err != nil {
		b.Fatalf("NewFixedHMACProvider() error = %v", err)
	}
	return provider
}

func newBenchmarkToken(b *testing.B, provider *Provider) string {
	b.Helper()
	token, err := provider.Compose(WithSubject("subject"), WithExpiresAfter(time.Hour))
	if err != nil {
		b.Fatalf("Compose() error = %v", err)
	}
	return token
}
