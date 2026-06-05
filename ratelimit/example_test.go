package ratelimit_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"

	"github.com/bluetape4k/bluetape-go/ratelimit"
)

func ExampleTokenBucket() {
	limiter, _ := ratelimit.New(ratelimit.Options{
		RatePerSecond: 2,
		Burst:         2,
	})

	first, _ := limiter.Allow(context.Background(), "tenant-1", 1)
	second, _ := limiter.Allow(context.Background(), "tenant-1", 1)
	third, _ := limiter.Allow(context.Background(), "tenant-1", 1)

	fmt.Println(first.Allowed, second.Allowed, third.Allowed)
	// Output:
	// true true false
}

func ExampleNewHandler() {
	limiter, _ := ratelimit.New(ratelimit.Options{
		RatePerSecond: 1,
		Burst:         1,
	})
	handler, _ := ratelimit.NewHandler(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("ok"))
	}), ratelimit.HandlerOptions{
		Limiter: limiter,
		KeyFunc: func(*http.Request) string { return "tenant-1" },
	})

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	first := httptest.NewRecorder()
	handler.ServeHTTP(first, req)
	second := httptest.NewRecorder()
	handler.ServeHTTP(second, req)

	fmt.Println(first.Code, second.Code)
	// Output:
	// 200 429
}
