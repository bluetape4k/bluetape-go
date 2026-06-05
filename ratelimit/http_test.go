package ratelimit

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

type stubLimiter struct {
	result Result
	err    error
	key    string
	tokens int64
}

func (l *stubLimiter) Allow(_ context.Context, key string, tokens int64) (Result, error) {
	l.key = key
	l.tokens = tokens
	return l.result, l.err
}

func TestHandlerAllowsRequest(t *testing.T) {
	limiter := &stubLimiter{result: Result{Allowed: true, Remaining: 4}}
	nextCalled := false
	handler, err := NewHandler(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		nextCalled = true
		w.WriteHeader(http.StatusNoContent)
	}), HandlerOptions{
		Limiter: limiter,
		KeyFunc: func(*http.Request) string { return "tenant-1" },
		Tokens:  2,
	})
	if err != nil {
		t.Fatalf("new handler: %v", err)
	}

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if !nextCalled || rec.Code != http.StatusNoContent {
		t.Fatalf("handler did not pass request: called=%v code=%d", nextCalled, rec.Code)
	}
	if limiter.key != "tenant-1" || limiter.tokens != 2 {
		t.Fatalf("limiter call = (%q,%d)", limiter.key, limiter.tokens)
	}
}

func TestHandlerRejectsRequestWithRetryAfter(t *testing.T) {
	limiter := &stubLimiter{result: Result{
		Allowed:    false,
		Remaining:  0,
		RetryAfter: 1500 * time.Millisecond,
	}}
	handler, err := NewHandler(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatalf("next handler must not run")
	}), HandlerOptions{Limiter: limiter, KeyFunc: func(*http.Request) string { return "tenant-1" }})
	if err != nil {
		t.Fatalf("new handler: %v", err)
	}

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("code = %d, want %d", rec.Code, http.StatusTooManyRequests)
	}
	if retryAfter := rec.Header().Get("Retry-After"); retryAfter != "2" {
		t.Fatalf("Retry-After = %q, want 2", retryAfter)
	}
}

func TestHandlerUsesCustomErrorHandler(t *testing.T) {
	backendErr := fmt.Errorf("redis unavailable")
	limiter := &stubLimiter{err: backendErr}
	customCalled := false
	handler, err := NewHandler(nil, HandlerOptions{
		Limiter: limiter,
		KeyFunc: func(*http.Request) string { return "tenant-1" },
		ErrorHandler: func(w http.ResponseWriter, _ *http.Request, _ Result, err error) {
			customCalled = true
			if !errors.Is(err, backendErr) {
				t.Fatalf("err = %v, want %v", err, backendErr)
			}
			w.WriteHeader(http.StatusTeapot)
		},
	})
	if err != nil {
		t.Fatalf("new handler: %v", err)
	}

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if !customCalled || rec.Code != http.StatusTeapot {
		t.Fatalf("custom handler called=%v code=%d", customCalled, rec.Code)
	}
}

func TestHandlerDefaultErrorMapsBackendErrorToServiceUnavailable(t *testing.T) {
	limiter := &stubLimiter{err: fmt.Errorf("redis unavailable")}
	handler, err := NewHandler(nil, HandlerOptions{
		Limiter: limiter,
		KeyFunc: func(*http.Request) string { return "tenant-1" },
	})
	if err != nil {
		t.Fatalf("new handler: %v", err)
	}

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("code = %d, want %d", rec.Code, http.StatusServiceUnavailable)
	}
}

func TestRemoteIPKeyDoesNotTrustProxyHeaders(t *testing.T) {
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	req.RemoteAddr = "10.0.0.5:12345"
	req.Header.Set("X-Forwarded-For", "203.0.113.10")

	if key := RemoteIPKey(req); key != "10.0.0.5" {
		t.Fatalf("key = %q, want remote host", key)
	}
}

func TestNewHandlerRejectsInvalidOptions(t *testing.T) {
	if _, err := NewHandler(nil, HandlerOptions{}); err == nil {
		t.Fatalf("expected missing limiter error")
	}
	limiter := &stubLimiter{result: Result{Allowed: true}}
	if _, err := NewHandler(nil, HandlerOptions{Limiter: limiter, Tokens: -1}); err == nil {
		t.Fatalf("expected negative tokens error")
	}
}
