package ratelimit

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func BenchmarkTokenBucketAllowAllowed(b *testing.B) {
	limiter, err := New(Options{RatePerSecond: 1_000_000, Burst: int64(b.N) + 1})
	if err != nil {
		b.Fatalf("new limiter: %v", err)
	}
	ctx := context.Background()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := limiter.Allow(ctx, "tenant-1", 1); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkTokenBucketAllowRejected(b *testing.B) {
	clock := newManualClock(time.Unix(0, 0))
	limiter, err := newWithClock(Options{RatePerSecond: 1, Burst: 1}, clock.Now)
	if err != nil {
		b.Fatalf("new limiter: %v", err)
	}
	if _, err := limiter.Allow(context.Background(), "tenant-1", 1); err != nil {
		b.Fatalf("consume burst: %v", err)
	}
	ctx := context.Background()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := limiter.Allow(ctx, "tenant-1", 1); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkHandlerAllowed(b *testing.B) {
	limiter := &stubLimiter{result: Result{Allowed: true, Remaining: 100}}
	handler, err := NewHandler(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}), HandlerOptions{
		Limiter: limiter,
		KeyFunc: func(*http.Request) string { return "tenant-1" },
	})
	if err != nil {
		b.Fatalf("new handler: %v", err)
	}
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		handler.ServeHTTP(httptest.NewRecorder(), req)
	}
}
