package echoadapter_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/bluetape4k/bluetape-go/ratelimit"
	echoadapter "github.com/bluetape4k/bluetape-go/web/echo"
	"github.com/labstack/echo/v4"
)

func TestRateLimitWriteFailureIsObservedWithoutLeakingCause(t *testing.T) {
	writeErr := errors.New("private response transport detail")
	cases := []struct {
		name       string
		result     ratelimit.Result
		allowErr   error
		wantStatus int
	}{
		{
			name:       "rejection",
			result:     ratelimit.Result{Remaining: 0, RetryAfter: time.Second},
			wantStatus: http.StatusTooManyRequests,
		},
		{
			name:       "backend",
			allowErr:   errors.New("private limiter detail"),
			wantStatus: http.StatusServiceUnavailable,
		},
		{
			name:       "cancellation",
			allowErr:   context.Canceled,
			wantStatus: http.StatusRequestTimeout,
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			writer := &failingResponseWriter{err: writeErr}
			request := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "http://example.test/orders", nil)
			ctx := echo.New().NewContext(request, writer)
			middleware, err := echoadapter.NewRateLimit(echoadapter.RateLimitOptions{
				Limiter: &fakeLimiter{allow: func(context.Context, string, int64) (ratelimit.Result, error) {
					return tt.result, tt.allowErr
				}},
			})
			if err != nil {
				t.Fatalf("NewRateLimit() error = %v", err)
			}

			if err := middleware(func(echo.Context) error {
				t.Fatal("rate-limit failure must not call downstream")
				return nil
			})(ctx); err != nil {
				t.Fatalf("middleware() error = %v, want nil after committed response", err)
			}

			observed := echoadapter.RateLimitWriteError(ctx)
			if observed == nil {
				t.Fatal("RateLimitWriteError() = nil, want observer")
			}
			if observed.Error() != "rate limit problem response write failed" {
				t.Fatalf("observer error = %q, want redacted fixed message", observed.Error())
			}
			if strings.Contains(observed.Error(), writeErr.Error()) {
				t.Fatalf("observer leaked writer cause: %q", observed.Error())
			}
			if !errors.Is(observed, writeErr) {
				t.Fatalf("observer does not unwrap writer cause: %v", observed)
			}
			if writer.status != tt.wantStatus || ctx.Response().Status != tt.wantStatus {
				t.Fatalf("status = writer:%d echo:%d, want %d", writer.status, ctx.Response().Status, tt.wantStatus)
			}
			if !ctx.Response().Committed {
				t.Fatal("failed Problem response must remain committed")
			}
		})
	}
}

func TestRateLimitWriteObserverPreservesNormalAndCustomPaths(t *testing.T) {
	t.Run("normal writer", func(t *testing.T) {
		ctx, recorder := newEchoContext(http.MethodGet, "http://example.test/orders", nil)
		middleware, err := echoadapter.NewRateLimit(echoadapter.RateLimitOptions{
			Limiter: &fakeLimiter{allow: func(context.Context, string, int64) (ratelimit.Result, error) {
				return ratelimit.Result{Remaining: 0}, nil
			}},
		})
		if err != nil {
			t.Fatalf("NewRateLimit() error = %v", err)
		}
		if err := middleware(func(echo.Context) error {
			t.Fatal("rejection must not call downstream")
			return nil
		})(ctx); err != nil {
			t.Fatalf("middleware() error = %v", err)
		}
		if recorder.Code != http.StatusTooManyRequests || echoadapter.RateLimitWriteError(ctx) != nil {
			t.Fatalf("status=%d observer=%v, want 429/nil", recorder.Code, echoadapter.RateLimitWriteError(ctx))
		}
	})

	t.Run("custom handler", func(t *testing.T) {
		ctx, recorder := newEchoContext(http.MethodGet, "http://example.test/orders", nil)
		middleware, err := echoadapter.NewRateLimit(echoadapter.RateLimitOptions{
			Limiter: &fakeLimiter{allow: func(context.Context, string, int64) (ratelimit.Result, error) {
				return ratelimit.Result{}, errors.New("private limiter detail")
			}},
			ErrorHandler: func(c echo.Context, _ ratelimit.Result, _ error) {
				_ = c.NoContent(http.StatusTeapot)
			},
		})
		if err != nil {
			t.Fatalf("NewRateLimit() error = %v", err)
		}
		if err := middleware(func(echo.Context) error {
			t.Fatal("backend failure must not call downstream")
			return nil
		})(ctx); err != nil {
			t.Fatalf("middleware() error = %v", err)
		}
		if recorder.Code != http.StatusTeapot || echoadapter.RateLimitWriteError(ctx) != nil {
			t.Fatalf("status=%d observer=%v, want 418/nil", recorder.Code, echoadapter.RateLimitWriteError(ctx))
		}
	})
}

func TestRateLimitWriteErrorIsNilForNilContext(t *testing.T) {
	if got := echoadapter.RateLimitWriteError(nil); got != nil {
		t.Fatalf("RateLimitWriteError(nil) = %v, want nil", got)
	}
}

type failingResponseWriter struct {
	header http.Header
	status int
	err    error
}

func (w *failingResponseWriter) Header() http.Header {
	if w.header == nil {
		w.header = make(http.Header)
	}
	return w.header
}

func (w *failingResponseWriter) WriteHeader(status int) {
	if w.status == 0 {
		w.status = status
	}
}

func (w *failingResponseWriter) Write([]byte) (int, error) {
	return 0, w.err
}
