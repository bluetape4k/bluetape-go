package echoadapter_test

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/bluetape4k/bluetape-go/ratelimit"
	echoadapter "github.com/bluetape4k/bluetape-go/web/echo"
	"github.com/labstack/echo/v4"
)

func TestRateLimitBackendErrorIsRedacted(t *testing.T) {
	ctx, recorder := newEchoContext(http.MethodGet, "http://example.test/orders", nil)
	middleware, err := echoadapter.NewRateLimit(echoadapter.RateLimitOptions{
		Limiter: &fakeLimiter{allow: func(context.Context, string, int64) (ratelimit.Result, error) {
			return ratelimit.Result{}, errors.New("private backend detail")
		}},
	})
	if err != nil {
		t.Fatalf("NewRateLimit() error = %v", err)
	}
	if err := middleware(func(next echo.Context) error {
		return next.NoContent(http.StatusNoContent)
	})(ctx); err != nil {
		t.Fatalf("middleware() error = %v", err)
	}
	if recorder.Code != http.StatusServiceUnavailable || strings.Contains(recorder.Body.String(), "private backend detail") {
		t.Fatalf("status/body = %d/%q, want redacted 503", recorder.Code, recorder.Body.String())
	}
}

func TestRateLimitRejectsTypedNilLimiter(t *testing.T) {
	var limiter *fakeLimiter
	if _, err := echoadapter.NewRateLimit(echoadapter.RateLimitOptions{Limiter: limiter}); err == nil {
		t.Fatal("NewRateLimit() accepted typed-nil limiter")
	}
}

func TestRateLimitNilNextReturnsNotFound(t *testing.T) {
	ctx, recorder := newEchoContext(http.MethodGet, "http://example.test/orders", nil)
	middleware, err := echoadapter.NewRateLimit(echoadapter.RateLimitOptions{
		Limiter: &fakeLimiter{allow: func(context.Context, string, int64) (ratelimit.Result, error) {
			return ratelimit.Result{Allowed: true}, nil
		}},
	})
	if err != nil {
		t.Fatalf("NewRateLimit() error = %v", err)
	}
	if err := middleware(nil)(ctx); err != nil {
		t.Fatalf("middleware() error = %v", err)
	}
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusNotFound)
	}
}
