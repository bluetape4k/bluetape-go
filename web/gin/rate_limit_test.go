package ginadapter_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/bluetape4k/bluetape-go/ratelimit"
	ginadapter "github.com/bluetape4k/bluetape-go/web/gin"
	"github.com/gin-gonic/gin"
)

func TestNewRateLimitAllowsAndCallsDownstreamOnce(t *testing.T) {
	gin.SetMode(gin.TestMode)
	limiter := fakeLimiter{allow: func(context.Context, string, int64) (ratelimit.Result, error) {
		return ratelimit.Result{Allowed: true, Remaining: 4}, nil
	}}
	router := gin.New()
	downstreamCalls := 0
	middleware, err := ginadapter.NewRateLimit(ginadapter.RateLimitOptions{
		Limiter: &limiter,
		KeyFunc: func(c *gin.Context) string { return c.Request.URL.Path },
	})
	if err != nil {
		t.Fatalf("NewRateLimit() error = %v", err)
	}
	router.Use(middleware)
	router.GET("/orders", func(c *gin.Context) {
		downstreamCalls++
		c.Status(http.StatusNoContent)
	})

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "http://example.test/orders", nil))
	if recorder.Code != http.StatusNoContent || downstreamCalls != 1 {
		t.Fatalf("response = (%d, downstream=%d), want (204, 1)", recorder.Code, downstreamCalls)
	}
	if got := limiter.keys(); len(got) != 1 || got[0] != "/orders" {
		t.Fatalf("keys = %v, want [/orders]", got)
	}
}

func TestNewRateLimitRejectsWithProblemAndPreservesQuotaHeaders(t *testing.T) {
	gin.SetMode(gin.TestMode)
	limiter := fakeLimiter{allow: func(context.Context, string, int64) (ratelimit.Result, error) {
		return ratelimit.Result{
			Allowed:    false,
			Remaining:  0,
			RetryAfter: 1500 * time.Millisecond,
		}, nil
	}}
	router := gin.New()
	downstreamCalls := 0
	middleware, err := ginadapter.NewRateLimit(ginadapter.RateLimitOptions{Limiter: &limiter})
	if err != nil {
		t.Fatalf("NewRateLimit() error = %v", err)
	}
	router.Use(middleware)
	router.GET("/orders", func(c *gin.Context) {
		downstreamCalls++
		c.Status(http.StatusNoContent)
	})

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "http://example.test/orders", nil))
	if recorder.Code != http.StatusTooManyRequests || downstreamCalls != 0 {
		t.Fatalf("response = (%d, downstream=%d), want (429, 0)", recorder.Code, downstreamCalls)
	}
	if got := recorder.Header().Get("Retry-After"); got != "2" {
		t.Fatalf("Retry-After = %q, want 2", got)
	}
	if got := recorder.Header().Get("X-RateLimit-Remaining"); got != "0" {
		t.Fatalf("X-RateLimit-Remaining = %q, want 0", got)
	}
	if !strings.Contains(recorder.Body.String(), "rate limit exceeded") {
		t.Fatalf("body = %q, want safe rate limit detail", recorder.Body.String())
	}
}

func TestNewRateLimitRedactsBackendErrorAndMapsCancellation(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		status     int
		bodyDenied string
	}{
		{name: "backend", err: errors.New("database password leaked"), status: http.StatusServiceUnavailable, bodyDenied: "database password"},
		{name: "canceled", err: context.Canceled, status: http.StatusRequestTimeout, bodyDenied: "context canceled"},
		{name: "deadline", err: context.DeadlineExceeded, status: http.StatusGatewayTimeout, bodyDenied: "context deadline exceeded"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gin.SetMode(gin.TestMode)
			limiter := fakeLimiter{allow: func(context.Context, string, int64) (ratelimit.Result, error) {
				return ratelimit.Result{}, tt.err
			}}
			router := gin.New()
			downstreamCalls := 0
			middleware, err := ginadapter.NewRateLimit(ginadapter.RateLimitOptions{Limiter: &limiter})
			if err != nil {
				t.Fatalf("NewRateLimit() error = %v", err)
			}
			router.Use(middleware)
			router.GET("/orders", func(c *gin.Context) {
				downstreamCalls++
				c.Status(http.StatusNoContent)
			})

			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "http://example.test/orders", nil))
			if recorder.Code != tt.status || downstreamCalls != 0 {
				t.Fatalf("response = (%d, downstream=%d), want (%d, 0)", recorder.Code, downstreamCalls, tt.status)
			}
			if strings.Contains(recorder.Body.String(), tt.bodyDenied) {
				t.Fatalf("body exposed unsafe detail: %q", recorder.Body.String())
			}
		})
	}
}

func TestNewRateLimitInvokesCustomErrorHandlerAfterAborting(t *testing.T) {
	gin.SetMode(gin.TestMode)
	limiter := fakeLimiter{allow: func(context.Context, string, int64) (ratelimit.Result, error) {
		return ratelimit.Result{Allowed: false, Remaining: 1}, nil
	}}
	var (
		called  bool
		aborted bool
	)
	middleware, err := ginadapter.NewRateLimit(ginadapter.RateLimitOptions{
		Limiter: &limiter,
		ErrorHandler: func(c *gin.Context, result ratelimit.Result, err error) {
			called = true
			aborted = c.IsAborted()
			if err != nil || result.Remaining != 1 {
				t.Errorf("callback arguments = (%+v, %v)", result, err)
			}
			c.Status(http.StatusTeapot)
		},
	})
	if err != nil {
		t.Fatalf("NewRateLimit() error = %v", err)
	}
	router := gin.New()
	downstreamCalls := 0
	router.Use(middleware)
	router.GET("/orders", func(c *gin.Context) {
		downstreamCalls++
		c.Status(http.StatusNoContent)
	})

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "http://example.test/orders", nil))
	if recorder.Code != http.StatusTeapot || !called || !aborted || downstreamCalls != 0 {
		t.Fatalf("response = (%d, called=%t, aborted=%t, downstream=%d)", recorder.Code, called, aborted, downstreamCalls)
	}
}

func TestNewRateLimitValidatesOptionsAndTypedNilLimiter(t *testing.T) {
	var typedNil *fakeLimiter
	for name, options := range map[string]ginadapter.RateLimitOptions{
		"nil":       {Limiter: nil},
		"typed nil": {Limiter: typedNil},
		"negative":  {Limiter: &fakeLimiter{}, Tokens: -1},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := ginadapter.NewRateLimit(options); err == nil {
				t.Fatal("NewRateLimit() error = nil, want validation error")
			}
		})
	}
}

func TestNewRateLimitDoesNotLeakGinContextAcrossConcurrentRequests(t *testing.T) {
	gin.SetMode(gin.TestMode)
	var (
		mu   sync.Mutex
		keys []string
	)
	limiter := fakeLimiter{allow: func(_ context.Context, key string, _ int64) (ratelimit.Result, error) {
		mu.Lock()
		keys = append(keys, key)
		mu.Unlock()
		return ratelimit.Result{Allowed: true}, nil
	}}
	middleware, err := ginadapter.NewRateLimit(ginadapter.RateLimitOptions{
		Limiter: &limiter,
		KeyFunc: func(c *gin.Context) string { return c.Request.URL.Path },
	})
	if err != nil {
		t.Fatalf("NewRateLimit() error = %v", err)
	}
	router := gin.New()
	router.Use(middleware)
	router.GET("/orders/:id", func(c *gin.Context) { c.Status(http.StatusNoContent) })

	const requests = 32
	var wg sync.WaitGroup
	for i := 0; i < requests; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "http://example.test/orders/"+string(rune('a'+i%26)), nil))
			if recorder.Code != http.StatusNoContent {
				t.Errorf("status = %d, want 204", recorder.Code)
			}
		}(i)
	}
	wg.Wait()
	if len(keys) != requests {
		t.Fatalf("captured keys = %d, want %d", len(keys), requests)
	}
}

type fakeLimiter struct {
	mu    sync.Mutex
	allow func(context.Context, string, int64) (ratelimit.Result, error)
	seen  []string
}

func (f *fakeLimiter) Allow(ctx context.Context, key string, tokens int64) (ratelimit.Result, error) {
	f.mu.Lock()
	f.seen = append(f.seen, key)
	f.mu.Unlock()
	if f.allow == nil {
		return ratelimit.Result{Allowed: true}, nil
	}
	return f.allow(ctx, key, tokens)
}

func (f *fakeLimiter) keys() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.seen...)
}

var _ ratelimit.Limiter = (*fakeLimiter)(nil)
