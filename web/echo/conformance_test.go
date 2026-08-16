package echoadapter_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/bluetape4k/bluetape-go/jwt"
	"github.com/bluetape4k/bluetape-go/ratelimit"
	"github.com/bluetape4k/bluetape-go/resilience"
	"github.com/bluetape4k/bluetape-go/web"
	echoadapter "github.com/bluetape4k/bluetape-go/web/echo"
	"github.com/bluetape4k/bluetape-go/webtest"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

func TestEchoAdapterConformance(t *testing.T) {
	webtest.Run(t,
		webtest.Scenario{
			Name:    "problem response uses RFC 9457 writer",
			Adapter: echoProblemAdapter,
			NewRequest: func(ctx context.Context) *http.Request {
				return httptest.NewRequestWithContext(ctx, http.MethodGet, "https://example.test/orders?x=1", nil)
			},
			Next: http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				_ = web.WriteProblem(w, req, conformanceProblemError{})
			}),
			Assert: func(t *testing.T, got webtest.Observation) {
				if got.StatusCode != http.StatusUnprocessableEntity || got.NextCalls != 1 {
					t.Fatalf("observation = %#v, want 422 and one next call", got)
				}
				if got.Header.Get("Content-Type") != "application/problem+json" {
					t.Fatalf("Content-Type = %q, want application/problem+json", got.Header.Get("Content-Type"))
				}
				var body map[string]any
				if err := json.Unmarshal(got.Body, &body); err != nil || body["instance"] != "/orders" {
					t.Fatalf("problem body = %q, want RFC 9457 instance", got.Body)
				}
			},
		},
		webtest.Scenario{
			Name:    "request context forwards trusted fields only",
			Adapter: echoRequestContextAdapter,
			NewRequest: func(ctx context.Context) *http.Request {
				req := httptest.NewRequestWithContext(ctx, http.MethodGet, "http://example.test/orders", nil)
				req.Header.Set(web.AuthSubjectHeader, "subject-1")
				req.Header.Set(web.RequestIDHeader, "request-1")
				req.Header.Set("X-Trusted", "yes")
				return req
			},
			Next: http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				value, ok := web.RequestContextFromContext(req.Context())
				if !ok || value.AuthSubject != "subject-1" {
					http.Error(w, "missing trusted context", http.StatusInternalServerError)
					return
				}
				w.WriteHeader(http.StatusNoContent)
			}),
			Assert: func(t *testing.T, got webtest.Observation) {
				if got.StatusCode != http.StatusNoContent || got.NextCalls != 1 || got.NextRequest == nil {
					t.Fatalf("observation = %#v, want trusted context and one next call", got)
				}
			},
		},
		webtest.Scenario{
			Name:    "request context drops untrusted restricted fields",
			Adapter: echoRequestContextAdapter,
			NewRequest: func(ctx context.Context) *http.Request {
				req := httptest.NewRequestWithContext(ctx, http.MethodGet, "http://example.test/orders", nil)
				req.Header.Set(web.AuthSubjectHeader, "spoofed")
				return req
			},
			Next: http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				value, _ := web.RequestContextFromContext(req.Context())
				if value.AuthSubject != "" {
					http.Error(w, "untrusted field forwarded", http.StatusInternalServerError)
					return
				}
				w.WriteHeader(http.StatusNoContent)
			}),
			Assert: func(t *testing.T, got webtest.Observation) {
				if got.StatusCode != http.StatusNoContent || got.NextCalls != 1 {
					t.Fatalf("observation = %#v, want 204 and one next call", got)
				}
			},
		},
		webtest.Scenario{
			Name: "rate limit rejection aborts next and preserves headers",
			Adapter: echoRateLimitAdapter(&fakeLimiter{allow: func(context.Context, string, int64) (ratelimit.Result, error) {
				return ratelimit.Result{Remaining: 0, RetryAfter: 1500 * time.Millisecond}, nil
			}}, nil),
			NewRequest: conformanceRequestFactory,
			Next:       http.NotFoundHandler(),
			Assert: func(t *testing.T, got webtest.Observation) {
				if got.StatusCode != http.StatusTooManyRequests || got.NextCalls != 0 || got.Header.Get("X-RateLimit-Remaining") != "0" {
					t.Fatalf("observation = %#v, want 429, no next, remaining=0", got)
				}
			},
		},
		webtest.Scenario{
			Name:    "JWT success stores reader before next",
			Adapter: echoJWTAdapter,
			NewRequest: func(ctx context.Context) *http.Request {
				req := conformanceRequestFactory(ctx)
				req.Header.Set("Authorization", "Bearer token")
				return req
			},
			Next: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) }),
			Assert: func(t *testing.T, got webtest.Observation) {
				if got.StatusCode != http.StatusNoContent || got.NextCalls != 1 {
					t.Fatalf("observation = %#v, want 204 and one next call", got)
				}
			},
		},
		webtest.Scenario{
			Name:       "JWT missing token is a redacted 401",
			Adapter:    echoJWTAdapter,
			NewRequest: conformanceRequestFactory,
			Next:       http.NotFoundHandler(),
			Assert: func(t *testing.T, got webtest.Observation) {
				if got.StatusCode != http.StatusUnauthorized || got.NextCalls != 0 || strings.Contains(string(got.Body), "token") {
					t.Fatalf("observation = %#v, want redacted 401 and no next", got)
				}
			},
		},
		webtest.Scenario{
			Name:       "resilience route reaches next once",
			Adapter:    echoResilienceAdapter(nil),
			NewRequest: conformanceRequestFactory,
			Next:       http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) }),
			Assert: func(t *testing.T, got webtest.Observation) {
				if got.StatusCode != http.StatusNoContent || got.NextCalls != 1 {
					t.Fatalf("observation = %#v, want 204 and one next call", got)
				}
			},
		},
		webtest.Scenario{
			Name: "resilience policy error is safe 503",
			Adapter: echoResilienceAdapter([]resilience.Policy[struct{}]{resilience.PolicyFunc[struct{}](func(resilience.Operation[struct{}]) resilience.Operation[struct{}] {
				return func(context.Context) (struct{}, error) { return struct{}{}, errors.New("private policy detail") }
			})}),
			NewRequest: conformanceRequestFactory,
			Next:       http.NotFoundHandler(),
			Assert: func(t *testing.T, got webtest.Observation) {
				if got.StatusCode != http.StatusServiceUnavailable || got.NextCalls != 0 || strings.Contains(string(got.Body), "private policy detail") {
					t.Fatalf("observation = %#v, want safe 503 and no next", got)
				}
			},
		},
	)

	t.Run("problem response uses RFC 9457 writer", func(t *testing.T) {
		ctx, recorder := newEchoContext(http.MethodGet, "https://example.test/orders?x=1", nil)
		err := echoadapter.AbortWithProblem(ctx, conformanceProblemError{})
		if err != nil {
			t.Fatalf("AbortWithProblem() error = %v", err)
		}
		if recorder.Code != http.StatusUnprocessableEntity || recorder.Header().Get("Content-Type") != "application/problem+json" || !ctx.Response().Committed {
			t.Fatalf("response = (%d, %q, committed=%t), want 422/application/problem+json/true", recorder.Code, recorder.Header().Get("Content-Type"), ctx.Response().Committed)
		}
		if strings.Contains(recorder.Body.String(), "x=1") {
			t.Fatalf("problem instance leaked query: %s", recorder.Body.String())
		}
	})

	t.Run("request context forwards trusted fields only", func(t *testing.T) {
		req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "http://example.test/orders", nil)
		req.Header.Set(web.AuthSubjectHeader, "subject-1")
		req.Header.Set(web.RequestIDHeader, "request-1")
		req.Header.Set("X-Trusted", "yes")
		ctx, recorder := newEchoContextFromRequest(req)
		middleware := echoadapter.RequestContext(web.RequestContextOptions{
			TrustedProxy: func(req *http.Request) bool { return req.Header.Get("X-Trusted") == "yes" },
		})
		called := false
		err := middleware(func(next echo.Context) error {
			called = true
			value, ok := web.RequestContextFromContext(next.Request().Context())
			if !ok || value.AuthSubject != "subject-1" {
				return errors.New("missing trusted context")
			}
			return next.NoContent(http.StatusNoContent)
		})(ctx)
		if err != nil || !called || recorder.Code != http.StatusNoContent {
			t.Fatalf("err=%v called=%t status=%d, want nil/true/204", err, called, recorder.Code)
		}
	})

	t.Run("request context drops untrusted restricted fields", func(t *testing.T) {
		req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "http://example.test/orders", nil)
		req.Header.Set(web.AuthSubjectHeader, "spoofed")
		ctx, recorder := newEchoContextFromRequest(req)
		err := echoadapter.RequestContext(web.RequestContextOptions{})(func(next echo.Context) error {
			value, _ := web.RequestContextFromContext(next.Request().Context())
			if value.AuthSubject != "" {
				return errors.New("untrusted field forwarded")
			}
			return next.NoContent(http.StatusNoContent)
		})(ctx)
		if err != nil || recorder.Code != http.StatusNoContent {
			t.Fatalf("err=%v status=%d, want nil/204", err, recorder.Code)
		}
	})

	t.Run("rate limit rejection aborts next and preserves headers", func(t *testing.T) {
		ctx, recorder := newEchoContext(http.MethodGet, "http://example.test/orders", nil)
		middleware, err := echoadapter.NewRateLimit(echoadapter.RateLimitOptions{
			Limiter: &fakeLimiter{allow: func(context.Context, string, int64) (ratelimit.Result, error) {
				return ratelimit.Result{Remaining: 0, RetryAfter: 1500 * time.Millisecond}, nil
			}},
		})
		if err != nil {
			t.Fatalf("NewRateLimit() error = %v", err)
		}
		nextCalls := 0
		err = middleware(func(next echo.Context) error {
			nextCalls++
			return next.NoContent(http.StatusNoContent)
		})(ctx)
		if err != nil || recorder.Code != http.StatusTooManyRequests || nextCalls != 0 || recorder.Header().Get("X-RateLimit-Remaining") != "0" || !ctx.Response().Committed {
			t.Fatalf("err=%v status=%d next=%d remaining=%q committed=%t, want nil/429/0/0/true", err, recorder.Code, nextCalls, recorder.Header().Get("X-RateLimit-Remaining"), ctx.Response().Committed)
		}
	})

	t.Run("JWT success stores reader before next", func(t *testing.T) {
		ctx, recorder := newEchoContext(http.MethodGet, "http://example.test/orders", nil)
		ctx.Request().Header.Set("Authorization", "Bearer token")
		middleware, err := echoadapter.NewJWT(echoadapter.JWTOptions{Parser: &fakeJWTParser{}})
		if err != nil {
			t.Fatalf("NewJWT() error = %v", err)
		}
		nextCalls := 0
		err = middleware(func(next echo.Context) error {
			nextCalls++
			if _, ok := echoadapter.JWTReader(next, ""); !ok {
				return errors.New("missing JWT reader")
			}
			return next.NoContent(http.StatusNoContent)
		})(ctx)
		if err != nil || recorder.Code != http.StatusNoContent || nextCalls != 1 {
			t.Fatalf("err=%v status=%d next=%d, want nil/204/1", err, recorder.Code, nextCalls)
		}
	})

	t.Run("JWT missing token is a redacted 401", func(t *testing.T) {
		ctx, recorder := newEchoContext(http.MethodGet, "http://example.test/orders", nil)
		middleware, err := echoadapter.NewJWT(echoadapter.JWTOptions{Parser: &fakeJWTParser{}})
		if err != nil {
			t.Fatalf("NewJWT() error = %v", err)
		}
		nextCalls := 0
		err = middleware(func(next echo.Context) error {
			nextCalls++
			return next.NoContent(http.StatusNoContent)
		})(ctx)
		if err != nil || recorder.Code != http.StatusUnauthorized || nextCalls != 0 || strings.Contains(recorder.Body.String(), "token") {
			t.Fatalf("err=%v status=%d next=%d body=%q, want nil/401/0/redacted", err, recorder.Code, nextCalls, recorder.Body.String())
		}
	})

	t.Run("resilience route reaches next once", func(t *testing.T) {
		ctx, recorder := newEchoContext(http.MethodGet, "http://example.test/orders", nil)
		nextCalls := 0
		err := echoadapter.WrapResilience(func(next echo.Context) error {
			nextCalls++
			return next.NoContent(http.StatusNoContent)
		}, echoadapter.ResilienceOptions{})(ctx)
		if err != nil || recorder.Code != http.StatusNoContent || nextCalls != 1 {
			t.Fatalf("err=%v status=%d next=%d, want nil/204/1", err, recorder.Code, nextCalls)
		}
	})

	t.Run("resilience policy error is safe 503", func(t *testing.T) {
		ctx, recorder := newEchoContext(http.MethodGet, "http://example.test/orders", nil)
		policy := resilience.PolicyFunc[struct{}](func(resilience.Operation[struct{}]) resilience.Operation[struct{}] {
			return func(context.Context) (struct{}, error) { return struct{}{}, errors.New("private policy detail") }
		})
		nextCalls := 0
		err := echoadapter.WrapResilience(func(next echo.Context) error {
			nextCalls++
			return next.NoContent(http.StatusNoContent)
		}, echoadapter.ResilienceOptions{Policies: []resilience.Policy[struct{}]{policy}})(ctx)
		if err != nil || recorder.Code != http.StatusServiceUnavailable || nextCalls != 0 || strings.Contains(recorder.Body.String(), "private policy detail") {
			t.Fatalf("err=%v status=%d next=%d body=%q, want nil/503/0/redacted", err, recorder.Code, nextCalls, recorder.Body.String())
		}
	})
}

func TestEchoSpecificConformanceContracts(t *testing.T) {
	t.Run("outer recovery handles resilience panic", func(t *testing.T) {
		server := echo.New()
		recoverConfig := middleware.DefaultRecoverConfig
		recoverConfig.DisablePrintStack = true
		server.Use(middleware.RecoverWithConfig(recoverConfig))
		server.GET("/panic", echoadapter.WrapResilience(func(echo.Context) error {
			panic("conformance panic")
		}, echoadapter.ResilienceOptions{}))
		recorder := httptest.NewRecorder()
		server.ServeHTTP(recorder, httptest.NewRequestWithContext(context.Background(), http.MethodGet, "http://example.test/panic", nil))
		if recorder.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d, want 500 from outer Recover", recorder.Code)
		}
	})

	t.Run("callback receives sanitized JWT request copy", func(t *testing.T) {
		ctx, recorder := newEchoContext(http.MethodGet, "http://example.test/orders", strings.NewReader("private body"))
		ctx.Request().Header.Set("Authorization", "Bearer token")
		original := ctx.Request()
		var callbackRequest *http.Request
		middleware, err := echoadapter.NewJWT(echoadapter.JWTOptions{
			Parser: &fakeJWTParser{err: errors.New("private parser detail")},
			ErrorHandler: func(ctx echo.Context, err error) {
				callbackRequest = ctx.Request()
				_ = echoadapter.AbortWithProblem(ctx, err)
			},
		})
		if err != nil {
			t.Fatalf("NewJWT() error = %v", err)
		}
		if err := middleware(func(echo.Context) error { return nil })(ctx); err != nil {
			t.Fatalf("middleware() error = %v", err)
		}
		callbackHeader := ""
		if callbackRequest != nil {
			callbackHeader = callbackRequest.Header.Get("Authorization")
		}
		if callbackRequest == nil || callbackHeader != "" || recorder.Code != http.StatusUnauthorized {
			t.Fatalf("callback request/header/status = %#v/%q/%d, want sanitized/empty/401", callbackRequest, callbackHeader, recorder.Code)
		}
		body, readErr := io.ReadAll(callbackRequest.Body)
		if readErr != nil || len(body) != 0 || original.Body == nil {
			t.Fatalf("callback body = %q/%v, want isolated empty body and original body", body, readErr)
		}
		originalBody, readErr := io.ReadAll(original.Body)
		if readErr != nil || string(originalBody) != "private body" {
			t.Fatalf("original body = %q/%v, want private body and no callback consumption", originalBody, readErr)
		}
	})

	t.Run("committed response is not overwritten", func(t *testing.T) {
		ctx, recorder := newEchoContext(http.MethodGet, "http://example.test/orders", nil)
		if err := ctx.String(http.StatusAccepted, "committed"); err != nil {
			t.Fatalf("String() error = %v", err)
		}
		if err := echoadapter.AbortWithProblem(ctx, conformanceProblemError{}); err != nil {
			t.Fatalf("AbortWithProblem() error = %v", err)
		}
		if recorder.Code != http.StatusAccepted || recorder.Body.String() != "committed" {
			t.Fatalf("response = (%d, %q), want 202/committed", recorder.Code, recorder.Body.String())
		}
	})
}

func echoProblemAdapter(next http.Handler) http.Handler {
	return echoEngine(func(c echo.Context) error {
		next.ServeHTTP(c.Response(), c.Request())
		return nil
	})
}

func echoRequestContextAdapter(next http.Handler) http.Handler {
	return echoEngine(func(c echo.Context) error {
		next.ServeHTTP(c.Response(), c.Request())
		return nil
	}, echoadapter.RequestContext(web.RequestContextOptions{
		TrustedProxy: func(req *http.Request) bool { return req.Header.Get("X-Trusted") == "yes" },
	}))
}

func echoRateLimitAdapter(limiter ratelimit.Limiter, keyFunc echoadapter.RateLimitKeyFunc) webtest.Adapter {
	return func(next http.Handler) http.Handler {
		middleware, err := echoadapter.NewRateLimit(echoadapter.RateLimitOptions{Limiter: limiter, KeyFunc: keyFunc})
		if err != nil {
			return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				http.Error(w, err.Error(), http.StatusInternalServerError)
			})
		}
		return echoEngine(func(c echo.Context) error {
			next.ServeHTTP(c.Response(), c.Request())
			return nil
		}, middleware)
	}
}

func echoJWTAdapter(next http.Handler) http.Handler {
	middleware, err := echoadapter.NewJWT(echoadapter.JWTOptions{Parser: &fakeJWTParser{}})
	if err != nil {
		return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		})
	}
	return echoEngine(func(c echo.Context) error {
		next.ServeHTTP(c.Response(), c.Request())
		return nil
	}, middleware)
}

func echoResilienceAdapter(policies []resilience.Policy[struct{}]) webtest.Adapter {
	return func(next http.Handler) http.Handler {
		wrapped := echoadapter.WrapResilience(func(c echo.Context) error {
			next.ServeHTTP(c.Response(), c.Request())
			return nil
		}, echoadapter.ResilienceOptions{Policies: policies})
		return echoEngine(wrapped)
	}
}

func echoEngine(handler echo.HandlerFunc, middleware ...echo.MiddlewareFunc) http.Handler {
	server := echo.New()
	server.Use(middleware...)
	server.Any("/*path", handler)
	return server
}

func conformanceRequestFactory(ctx context.Context) *http.Request {
	return httptest.NewRequestWithContext(ctx, http.MethodGet, "http://example.test/orders", nil)
}

func newEchoContext(method, target string, body io.Reader) (echo.Context, *httptest.ResponseRecorder) {
	req := httptest.NewRequestWithContext(context.Background(), method, target, body)
	ctx, recorder := newEchoContextFromRequest(req)
	return ctx, recorder
}

func newEchoContextFromRequest(req *http.Request) (echo.Context, *httptest.ResponseRecorder) {
	recorder := httptest.NewRecorder()
	return echo.New().NewContext(req, recorder), recorder
}

type fakeLimiter struct {
	allow func(context.Context, string, int64) (ratelimit.Result, error)
}

func (f *fakeLimiter) Allow(ctx context.Context, key string, tokens int64) (ratelimit.Result, error) {
	return f.allow(ctx, key, tokens)
}

type fakeJWTParser struct {
	err error
}

func (f *fakeJWTParser) Parse(string, ...jwt.ParseOption) (*jwt.Reader, error) {
	if f.err != nil {
		return nil, f.err
	}
	return &jwt.Reader{}, nil
}

func (f *fakeJWTParser) TryParse(token string, options ...jwt.ParseOption) (*jwt.Reader, bool) {
	reader, err := f.Parse(token, options...)
	return reader, err == nil
}

type conformanceProblemError struct{}

func (conformanceProblemError) Error() string { return "invalid order" }

func (conformanceProblemError) ProblemDetails() web.Problem {
	return web.Problem{Status: http.StatusUnprocessableEntity, Title: "Invalid order", Detail: "invalid order"}
}
