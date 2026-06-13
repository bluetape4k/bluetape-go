package jwt

import (
	"strings"
	"testing"
	"time"
)

func TestCacheProfileBypassesCustomParseClock(t *testing.T) {
	now := time.Date(2026, 6, 14, 1, 0, 0, 0, time.UTC)
	parse, err := normalizeParseConfig(func() time.Time { return now }, []ParseOption{
		WithParseClock(func() time.Time { return now }),
	})
	if err != nil {
		t.Fatalf("normalizeParseConfig() error = %v", err)
	}
	cfg := cacheConfig{maxTTL: time.Minute, keyPrefix: "jwt:test", trustScope: "scope", now: time.Now}
	profile := buildCacheProfile(HS256, cfg, parse, "token")
	if profile.cacheable {
		t.Fatalf("custom parse clock should bypass cache")
	}
}

func TestCacheProfileKeySeparatesParseOptionsAndDoesNotExposeRawToken(t *testing.T) {
	now := time.Date(2026, 6, 14, 1, 0, 0, 0, time.UTC)
	cfg := cacheConfig{maxTTL: time.Minute, keyPrefix: "jwt:test", trustScope: "scope", now: time.Now}
	token := "header.payload.signature"

	first, err := normalizeParseConfig(func() time.Time { return now }, []ParseOption{
		WithExpectedAudience("api", "worker"),
		WithExpectedIssuer("issuer"),
		WithExpectedSubject("subject"),
		WithLeeway(time.Second),
		WithExpirationRequired(),
	})
	if err != nil {
		t.Fatalf("normalize first profile error = %v", err)
	}
	second, err := normalizeParseConfig(func() time.Time { return now }, []ParseOption{
		WithExpectedAudience("worker", "api"),
		WithExpectedIssuer("issuer"),
		WithExpectedSubject("subject"),
		WithLeeway(time.Second),
		WithExpirationRequired(),
	})
	if err != nil {
		t.Fatalf("normalize second profile error = %v", err)
	}

	firstKey := buildCacheProfile(HS256, cfg, first, token)
	secondKey := buildCacheProfile(HS256, cfg, second, token)
	if !firstKey.cacheable || !secondKey.cacheable {
		t.Fatalf("ordinary parse options should be cacheable")
	}
	if firstKey.key == secondKey.key {
		t.Fatalf("audience order should change cache key")
	}
	if strings.Contains(firstKey.key, token) {
		t.Fatalf("cache key exposed raw token: %q", firstKey.key)
	}
	if !strings.Contains(firstKey.key, "jwt:test") || !strings.Contains(firstKey.key, "scope") || !strings.Contains(firstKey.key, string(HS256)) {
		t.Fatalf("cache key should include prefix, scope, and algorithm: %q", firstKey.key)
	}
}

func TestCacheProfileKeyFramesPrefixAndScope(t *testing.T) {
	now := time.Date(2026, 6, 14, 1, 0, 0, 0, time.UTC)
	parse, err := normalizeParseConfig(func() time.Time { return now }, nil)
	if err != nil {
		t.Fatalf("normalize parse profile error = %v", err)
	}
	token := "header.payload.signature"
	first := cacheConfig{
		maxTTL:     time.Minute,
		keyPrefix:  "tenant-a",
		trustScope: "blue|scope:green",
		now:        time.Now,
	}
	second := cacheConfig{
		maxTTL:     time.Minute,
		keyPrefix:  "tenant-a|scope:blue",
		trustScope: "green",
		now:        time.Now,
	}

	firstKey := buildCacheProfile(HS256, first, parse, token)
	secondKey := buildCacheProfile(HS256, second, parse, token)
	if !firstKey.cacheable || !secondKey.cacheable {
		t.Fatalf("ordinary parse options should be cacheable")
	}
	if firstKey.key == secondKey.key {
		t.Fatalf("delimiter-bearing prefix/scope pairs must not collide: %q", firstKey.key)
	}
	if !strings.Contains(firstKey.key, "scope=16:blue|scope:green") {
		t.Fatalf("scope field should be length-framed: %q", firstKey.key)
	}
}
