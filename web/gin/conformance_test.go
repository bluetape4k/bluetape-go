package ginadapter_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/bluetape4k/bluetape-go/ratelimit"
	"github.com/bluetape4k/bluetape-go/resilience"
	"github.com/bluetape4k/bluetape-go/web"
	ginadapter "github.com/bluetape4k/bluetape-go/web/gin"
	"github.com/bluetape4k/bluetape-go/webtest"
	"github.com/gin-gonic/gin"
)

func TestGinAdapterConformance(t *testing.T) {
	gin.SetMode(gin.TestMode)
	webtest.Run(t,
		webtest.Scenario{
			Name:    "problem response uses RFC 9457 writer",
			Adapter: ginProblemAdapter,
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
			Adapter: ginRequestContextAdapter,
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
			Adapter: ginRequestContextAdapter,
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
			Adapter: ginRateLimitAdapter(&fakeLimiter{allow: func(context.Context, string, int64) (ratelimit.Result, error) {
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
			Adapter: ginJWTAdapter,
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
			Adapter:    ginJWTAdapter,
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
			Adapter:    ginResilienceAdapter(nil),
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
			Adapter: ginResilienceAdapter([]resilience.Policy[struct{}]{resilience.PolicyFunc[struct{}](func(resilience.Operation[struct{}]) resilience.Operation[struct{}] {
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
}

func TestGinSpecificConformanceContracts(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Run("rate limit aborts and writes without downstream", func(t *testing.T) {
		middleware, err := ginadapter.NewRateLimit(ginadapter.RateLimitOptions{
			Limiter: &fakeLimiter{allow: func(context.Context, string, int64) (ratelimit.Result, error) {
				return ratelimit.Result{Allowed: false, Remaining: 0}, nil
			}},
		})
		if err != nil {
			t.Fatalf("NewRateLimit() error = %v", err)
		}
		downstreamCalls := 0
		var aborted, written bool
		router := gin.New()
		router.Use(func(c *gin.Context) {
			c.Next()
			aborted = c.IsAborted()
			written = c.Writer.Written()
		})
		router.Use(middleware)
		router.GET("/orders", func(c *gin.Context) {
			downstreamCalls++
			c.Status(http.StatusNoContent)
		})
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, conformanceRequestFactory(context.Background()))
		if recorder.Code != http.StatusTooManyRequests || downstreamCalls != 0 || !aborted || !written {
			t.Fatalf("response = (%d, downstream=%d, aborted=%t, written=%t), want 429/0/true/true", recorder.Code, downstreamCalls, aborted, written)
		}
	})

	t.Run("resilience preserves c.Errors causes without metadata", func(t *testing.T) {
		routeErr := errors.New("private route detail")
		var observed []*gin.Error
		router := gin.New()
		router.Use(func(c *gin.Context) {
			c.Next()
			observed = append([]*gin.Error(nil), c.Errors...)
		})
		router.GET("/errors", ginadapter.WrapResilience(func(c *gin.Context) {
			entry := c.Error(routeErr)
			_ = entry.SetMeta("private")
		}, ginadapter.ResilienceOptions{}))

		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, httptest.NewRequestWithContext(context.Background(), http.MethodGet, "http://example.test/errors", nil))
		if recorder.Code != http.StatusServiceUnavailable || len(observed) != 2 {
			t.Fatalf("response = (%d, errors=%d), want 503 and two sanitized errors", recorder.Code, len(observed))
		}
		if observed[0].Meta != nil || !errors.Is(observed[0].Err, routeErr) {
			t.Fatalf("first observed error = %#v, want cause-preserving error without metadata", observed[0])
		}
	})

	t.Run("JWTReader is available before downstream once", func(t *testing.T) {
		middleware, err := ginadapter.NewJWT(ginadapter.JWTOptions{Parser: &fakeJWTParser{}})
		if err != nil {
			t.Fatalf("NewJWT() error = %v", err)
		}
		downstreamCalls := 0
		router := gin.New()
		router.Use(middleware)
		router.GET("/orders", func(c *gin.Context) {
			downstreamCalls++
			if _, ok := ginadapter.JWTReader(c, ""); !ok {
				t.Error("JWTReader() did not find the verified reader")
			}
			c.Status(http.StatusNoContent)
		})
		req := conformanceRequestFactory(context.Background())
		req.Header.Set("Authorization", "Bearer token")
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, req)
		if recorder.Code != http.StatusNoContent || downstreamCalls != 1 {
			t.Fatalf("response = (%d, downstream=%d), want 204/1", recorder.Code, downstreamCalls)
		}
	})

	t.Run("outer recovery handles resilience panic", func(t *testing.T) {
		router := gin.New()
		router.Use(gin.Recovery())
		router.GET("/panic", ginadapter.WrapResilience(func(*gin.Context) {
			panic("conformance panic")
		}, ginadapter.ResilienceOptions{}))
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, httptest.NewRequestWithContext(context.Background(), http.MethodGet, "http://example.test/panic", nil))
		if recorder.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d, want 500 from outer Recovery", recorder.Code)
		}
	})
}

func ginProblemAdapter(next http.Handler) http.Handler {
	return ginEngine(func(c *gin.Context) {
		next.ServeHTTP(c.Writer, c.Request)
	})
}

func ginRequestContextAdapter(next http.Handler) http.Handler {
	router := gin.New()
	router.Use(ginadapter.RequestContext(web.RequestContextOptions{
		TrustedProxy: func(req *http.Request) bool { return req.Header.Get("X-Trusted") == "yes" },
	}))
	router.Any("/*path", func(c *gin.Context) { next.ServeHTTP(c.Writer, c.Request) })
	return router
}

func ginRateLimitAdapter(limiter ratelimit.Limiter, keyFunc ginadapter.RateLimitKeyFunc) webtest.Adapter {
	return func(next http.Handler) http.Handler {
		middleware, err := ginadapter.NewRateLimit(ginadapter.RateLimitOptions{Limiter: limiter, KeyFunc: keyFunc})
		if err != nil {
			return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				http.Error(w, err.Error(), http.StatusInternalServerError)
			})
		}
		return ginEngine(func(c *gin.Context) {
			middleware(c)
			if !c.IsAborted() {
				next.ServeHTTP(c.Writer, c.Request)
			}
		})
	}
}

func ginJWTAdapter(next http.Handler) http.Handler {
	middleware, err := ginadapter.NewJWT(ginadapter.JWTOptions{Parser: &fakeJWTParser{}})
	if err != nil {
		return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		})
	}
	return ginEngine(func(c *gin.Context) {
		middleware(c)
		if !c.IsAborted() {
			next.ServeHTTP(c.Writer, c.Request)
		}
	})
}

func ginResilienceAdapter(policies []resilience.Policy[struct{}]) webtest.Adapter {
	return func(next http.Handler) http.Handler {
		wrapped := ginadapter.WrapResilience(func(c *gin.Context) { next.ServeHTTP(c.Writer, c.Request) }, ginadapter.ResilienceOptions{Policies: policies})
		return ginEngine(wrapped)
	}
}

func ginEngine(handler gin.HandlerFunc) http.Handler {
	router := gin.New()
	router.Any("/*path", handler)
	return router
}

func conformanceRequestFactory(ctx context.Context) *http.Request {
	return httptest.NewRequestWithContext(ctx, http.MethodGet, "http://example.test/orders", nil)
}

type conformanceProblemError struct{}

func (conformanceProblemError) Error() string { return "invalid order" }

func (conformanceProblemError) ProblemDetails() web.Problem {
	return web.Problem{Status: http.StatusUnprocessableEntity, Title: "Invalid order", Detail: "invalid order"}
}
