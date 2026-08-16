package jwks

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	jose "github.com/go-jose/go-jose/v4"
)

func BenchmarkLookupCacheHit(b *testing.B) {
	provider, requests, cleanup := benchmarkProvider(b)
	defer cleanup()
	if _, err := provider.Lookup(context.Background(), "benchmark", RS256); err != nil {
		b.Fatal(err)
	}
	baseline := requests.Load()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := provider.Lookup(context.Background(), "benchmark", RS256); err != nil {
			b.Fatal(err)
		}
	}
	b.StopTimer()
	reportHTTPRequests(b, requests, baseline)
}

func BenchmarkLookupParallelHit(b *testing.B) {
	provider, requests, cleanup := benchmarkProvider(b)
	defer cleanup()
	if _, err := provider.Lookup(context.Background(), "benchmark", RS256); err != nil {
		b.Fatal(err)
	}
	baseline := requests.Load()
	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			if _, err := provider.Lookup(context.Background(), "benchmark", RS256); err != nil {
				b.Error(err)
			}
		}
	})
	b.StopTimer()
	reportHTTPRequests(b, requests, baseline)
}

func BenchmarkLookupForcedRefresh(b *testing.B) {
	provider, requests, cleanup := benchmarkProvider(b)
	defer cleanup()
	if err := provider.Refresh(context.Background()); err != nil {
		b.Fatal(err)
	}
	baseline := requests.Load()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := provider.Refresh(context.Background()); err != nil {
			b.Fatal(err)
		}
	}
	b.StopTimer()
	reportHTTPRequests(b, requests, baseline)
}

func BenchmarkLookupColdMiss(b *testing.B) {
	provider, requests, cleanup := benchmarkProvider(b)
	defer cleanup()
	configureExpiringBenchmarkProvider(provider)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := provider.Lookup(context.Background(), "benchmark", RS256); err != nil {
			b.Fatal(err)
		}
	}
	b.StopTimer()
	reportHTTPRequests(b, requests, 0)
}

func BenchmarkLookupTTLExpiry(b *testing.B) {
	provider, requests, cleanup := benchmarkProvider(b)
	defer cleanup()
	now := time.Now()
	provider.cfg.cacheTTL = time.Nanosecond
	provider.cfg.now = func() time.Time { return now }
	if _, err := provider.Lookup(context.Background(), "benchmark", RS256); err != nil {
		b.Fatal(err)
	}
	baseline := requests.Load()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		now = now.Add(2 * time.Nanosecond)
		if _, err := provider.Lookup(context.Background(), "benchmark", RS256); err != nil {
			b.Fatal(err)
		}
	}
	b.StopTimer()
	reportHTTPRequests(b, requests, baseline)
}

func BenchmarkLookupParallelMiss(b *testing.B) {
	provider, requests, cleanup := benchmarkProvider(b)
	defer cleanup()
	configureExpiringBenchmarkProvider(provider)
	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			if _, err := provider.Lookup(context.Background(), "benchmark", RS256); err != nil {
				b.Error(err)
			}
		}
	})
	b.StopTimer()
	reportHTTPRequests(b, requests, 0)
}

func reportHTTPRequests(b *testing.B, requests *atomic.Int64, baseline int64) {
	b.Helper()
	b.ReportMetric(float64(requests.Load()-baseline)/float64(b.N), "http-requests/op")
}

func configureExpiringBenchmarkProvider(provider *Provider) {
	var ticks atomic.Int64
	provider.cfg.cacheTTL = time.Nanosecond
	provider.cfg.now = func() time.Time {
		return time.Unix(0, ticks.Add(int64(time.Microsecond))).UTC()
	}
}

func benchmarkProvider(b *testing.B) (*Provider, *atomic.Int64, func()) {
	b.Helper()
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		b.Fatal(err)
	}
	body, err := json.Marshal(jose.JSONWebKeySet{Keys: []jose.JSONWebKey{{
		Key:       &privateKey.PublicKey,
		KeyID:     "benchmark",
		Algorithm: string(RS256),
		Use:       "sig",
	}}})
	if err != nil {
		b.Fatal(err)
	}
	var requests atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests.Add(1)
		_, _ = w.Write(body)
	}))
	provider, err := New(server.URL, WithCacheTTL(time.Hour))
	if err != nil {
		server.Close()
		b.Fatal(err)
	}
	return provider, &requests, server.Close
}
