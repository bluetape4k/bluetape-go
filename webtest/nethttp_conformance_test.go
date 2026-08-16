package webtest_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/bluetape4k/bluetape-go/ratelimit"
	"github.com/bluetape4k/bluetape-go/resilience"
	"github.com/bluetape4k/bluetape-go/web"
	"github.com/bluetape4k/bluetape-go/webtest"
)

func TestProblemResponseConformance(t *testing.T) {
	webtest.Run(t, webtest.Scenario{
		Name:    "problem response preserves status media type and instance",
		Adapter: identityAdapter,
		NewRequest: func(ctx context.Context) *http.Request {
			return httptest.NewRequestWithContext(ctx, http.MethodGet, "https://example.test/orders?x=1", nil)
		},
		Next: http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			if err := web.WriteProblem(w, req, problemError{problem: web.Problem{
				Type:   "https://example.test/problems/invalid-order",
				Title:  "Invalid order",
				Status: http.StatusUnprocessableEntity,
				Detail: "order total is invalid",
			}}); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}
		}),
		Assert: func(t *testing.T, got webtest.Observation) {
			if got.StatusCode != http.StatusUnprocessableEntity {
				t.Fatalf("status = %d, want %d", got.StatusCode, http.StatusUnprocessableEntity)
			}
			if got.Header.Get("Content-Type") != "application/problem+json" {
				t.Fatalf("Content-Type = %q, want application/problem+json", got.Header.Get("Content-Type"))
			}
			var body map[string]any
			if err := json.Unmarshal(got.Body, &body); err != nil {
				t.Fatalf("problem body is not JSON: %v", err)
			}
			if body["instance"] != "/orders" || body["detail"] != "order total is invalid" {
				t.Fatalf("problem body = %#v, want instance and detail", body)
			}
		},
	})
}

func TestRequestContextConformanceHonorsTrustBoundary(t *testing.T) {
	var original []*http.Request
	adapter := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			withContext, _, err := web.WithRequestContextOnRequest(req, web.RequestContextOptions{
				TrustedProxy: func(req *http.Request) bool { return req.Header.Get("X-Trusted") == "yes" },
			})
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			next.ServeHTTP(w, withContext)
		})
	}

	webtest.Run(t,
		webtest.Scenario{
			Name:    "trusted proxy forwards restricted context",
			Adapter: adapter,
			NewRequest: func(ctx context.Context) *http.Request {
				req := httptest.NewRequestWithContext(ctx, http.MethodGet, "http://example.test/", nil)
				req.Header.Set("X-Request-ID", "request-1")
				req.Header.Set("X-Correlation-ID", "correlation-1")
				req.Header.Set("X-Trusted", "yes")
				req.Header.Set("X-Auth-Subject", "subject-1")
				req.Header.Set("Traceparent", "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01")
				original = append(original, req)
				return req
			},
			Next: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) }),
			Assert: func(t *testing.T, got webtest.Observation) {
				if got.StatusCode != http.StatusNoContent || got.NextRequest == nil {
					t.Fatalf("observation = %#v, want next request and 204", got)
				}
				value, ok := web.RequestContextFromContext(got.NextRequest.Context())
				if !ok || value.RequestID != "request-1" || value.CorrelationID != "correlation-1" || value.AuthSubject != "subject-1" {
					t.Fatalf("request context = %#v, %t", value, ok)
				}
				if got.NextRequest == original[0] {
					t.Fatal("request context adapter reused the original request")
				}
			},
		},
		webtest.Scenario{
			Name:    "untrusted proxy drops restricted context",
			Adapter: adapter,
			NewRequest: func(ctx context.Context) *http.Request {
				req := httptest.NewRequestWithContext(ctx, http.MethodGet, "http://example.test/", nil)
				req.Header.Set("X-Request-ID", "request-2")
				req.Header.Set("X-Trusted", "no")
				req.Header.Set("X-Auth-Subject", "spoofed")
				req.Header.Set("Traceparent", "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01")
				original = append(original, req)
				return req
			},
			Next: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) }),
			Assert: func(t *testing.T, got webtest.Observation) {
				if got.NextRequest == nil {
					t.Fatal("untrusted request did not reach next")
				}
				value, ok := web.RequestContextFromContext(got.NextRequest.Context())
				if !ok || value.RequestID != "request-2" || value.AuthSubject != "" || value.TraceParent != "" {
					t.Fatalf("untrusted request context = %#v, %t", value, ok)
				}
				if _, ok := web.RequestContextFromContext(original[1].Context()); ok {
					t.Fatal("original request was mutated")
				}
			},
		},
	)
}

func TestResilienceHandlerConformanceMapsPolicyAndTimeoutErrors(t *testing.T) {
	deny := resilience.PolicyFunc[struct{}](func(resilience.Operation[struct{}]) resilience.Operation[struct{}] {
		return func(context.Context) (struct{}, error) { return struct{}{}, errors.New("bulkhead full") }
	})
	timeout, err := resilience.NewTimeout[struct{}](resilience.TimeoutOptions{Name: "http", Timeout: 10 * time.Millisecond})
	if err != nil {
		t.Fatalf("NewTimeout failed: %v", err)
	}

	webtest.Run(t,
		webtest.Scenario{
			Name: "policy error uses caller mapping",
			Adapter: resilienceAdapter([]resilience.Policy[struct{}]{deny}, func(w http.ResponseWriter, _ *http.Request, err error) {
				http.Error(w, err.Error(), http.StatusTooManyRequests)
			}),
			NewRequest: requestFactory,
			Next: http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
				t.Fatal("rejected request reached next")
			}),
			Assert: func(t *testing.T, got webtest.Observation) {
				if got.StatusCode != http.StatusTooManyRequests || got.NextCalls != 0 || !strings.Contains(string(got.Body), "bulkhead full") {
					t.Fatalf("observation = %#v, want mapped policy error", got)
				}
			},
		},
		webtest.Scenario{
			Name:       "timeout cancels next and maps service unavailable",
			Adapter:    resilienceAdapter([]resilience.Policy[struct{}]{timeout}, nil),
			NewRequest: requestFactory,
			Next: http.HandlerFunc(func(_ http.ResponseWriter, req *http.Request) {
				<-req.Context().Done()
			}),
			Assert: func(t *testing.T, got webtest.Observation) {
				if got.StatusCode != http.StatusServiceUnavailable || got.NextCalls != 1 {
					t.Fatalf("observation = %#v, want timeout mapping and one next call", got)
				}
			},
		},
	)
}

func TestRateLimitHandlerConformanceUsesRemoteKeyAndMapsRejection(t *testing.T) {
	limiter := &stubLimiter{result: ratelimit.Result{Allowed: true, Remaining: 4}}
	webtest.Run(t, webtest.Scenario{
		Name:    "remote key ignores forwarding header",
		Adapter: ratelimitAdapter(limiter, nil),
		NewRequest: func(ctx context.Context) *http.Request {
			req := httptest.NewRequestWithContext(ctx, http.MethodGet, "http://example.test/", nil)
			req.RemoteAddr = "10.0.0.5:12345"
			req.Header.Set("X-Forwarded-For", "203.0.113.10")
			return req
		},
		Next: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) }),
		Assert: func(t *testing.T, got webtest.Observation) {
			if got.StatusCode != http.StatusNoContent || limiter.key != "10.0.0.5" || limiter.tokens != 1 {
				t.Fatalf("status=%d key=%q tokens=%d, want 204/10.0.0.5/1", got.StatusCode, limiter.key, limiter.tokens)
			}
		},
	})

	rejected := &stubLimiter{result: ratelimit.Result{Allowed: false, Remaining: 0, RetryAfter: 1500 * time.Millisecond}}
	webtest.Run(t, webtest.Scenario{
		Name:       "rejection maps retry after",
		Adapter:    ratelimitAdapter(rejected, nil),
		NewRequest: requestFactory,
		Next: http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
			t.Fatal("rejected request reached next")
		}),
		Assert: func(t *testing.T, got webtest.Observation) {
			if got.StatusCode != http.StatusTooManyRequests || got.NextCalls != 0 || got.Header.Get("Retry-After") != "2" {
				t.Fatalf("observation = %#v, want 429, no next, Retry-After=2", got)
			}
		},
	})
}

func TestRateLimitHandlerConformanceMapsBackendError(t *testing.T) {
	backendErr := errors.New("redis unavailable")
	webtest.Run(t, webtest.Scenario{
		Name:       "backend error maps service unavailable",
		Adapter:    ratelimitAdapter(&stubLimiter{err: backendErr}, nil),
		NewRequest: requestFactory,
		Next:       http.NotFoundHandler(),
		Assert: func(t *testing.T, got webtest.Observation) {
			if got.StatusCode != http.StatusServiceUnavailable || !strings.Contains(string(got.Body), backendErr.Error()) {
				t.Fatalf("observation = %#v, want 503 backend error", got)
			}
		},
	})
}

func TestRoundTripperConformanceClosesRetryableResponseBody(t *testing.T) {
	retry, err := resilience.NewRetry[*http.Response](resilience.RetryOptions{ //nolint:bodyclose
		Name:        "http",
		MaxAttempts: 2,
		Backoff:     resilience.NoBackoff(),
	})
	if err != nil {
		t.Fatalf("NewRetry failed: %v", err)
	}
	firstBody := webtest.NewCloseTracker(strings.NewReader("unavailable"))
	calls := 0
	client := http.Client{Transport: resilience.NewRoundTripper(resilience.RoundTripperOptions{
		Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			calls++
			if calls == 1 {
				return &http.Response{StatusCode: http.StatusServiceUnavailable, Status: "503 Service Unavailable", Body: firstBody}, nil
			}
			return &http.Response{StatusCode: http.StatusOK, Status: "200 OK", Body: io.NopCloser(strings.NewReader("ok"))}, nil
		}),
		Policies:        []resilience.Policy[*http.Response]{retry},
		RetryableStatus: resilience.RetryableServerError,
	})}

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "http://example.test/", nil)
	if err != nil {
		t.Fatalf("NewRequestWithContext failed: %v", err)
	}
	response, err := client.Do(req)
	if err != nil {
		t.Fatalf("client.Do failed: %v", err)
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode != http.StatusOK || !firstBody.Closed() || firstBody.CloseCount() != 1 {
		t.Fatalf("status=%d closed=%t count=%d, want 200/true/1", response.StatusCode, firstBody.Closed(), firstBody.CloseCount())
	}
}

func TestResilienceHookConformanceIsSynchronousAndCaseLocal(t *testing.T) {
	type contextKey struct{}
	marker := "marker"
	ctx := context.WithValue(context.Background(), contextKey{}, marker)
	var mu sync.Mutex
	var events []resilience.Event
	retry, err := resilience.NewRetry[string](resilience.RetryOptions{
		Name:        "hook",
		MaxAttempts: 1,
		OnEvent: func(eventContext context.Context, event resilience.Event) {
			if eventContext.Value(contextKey{}) != marker {
				t.Errorf("hook context marker missing")
			}
			mu.Lock()
			events = append(events, event)
			mu.Unlock()
		},
	})
	if err != nil {
		t.Fatalf("NewRetry failed: %v", err)
	}
	_, err = resilience.Run(ctx, func(context.Context) (string, error) { return "", errors.New("boom") }, retry)
	if err == nil {
		t.Fatal("Run returned nil error")
	}
	mu.Lock()
	firstCount := len(events)
	mu.Unlock()
	if firstCount != 1 || events[0].Kind != resilience.EventFailure {
		t.Fatalf("events = %#v, want one failure event", events)
	}

	secondEvents := make([]resilience.Event, 0, 1)
	secondRetry, err := resilience.NewRetry[string](resilience.RetryOptions{
		Name:        "second",
		MaxAttempts: 1,
		OnEvent:     func(_ context.Context, event resilience.Event) { secondEvents = append(secondEvents, event) },
	})
	if err != nil {
		t.Fatalf("NewRetry second failed: %v", err)
	}
	if _, err := resilience.Run(context.Background(), func(context.Context) (string, error) { return "ok", nil }, secondRetry); err != nil {
		t.Fatalf("second Run failed: %v", err)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(events) != firstCount || len(secondEvents) != 1 || secondEvents[0].Kind != resilience.EventSuccess {
		t.Fatalf("event isolation = first:%d second:%#v", len(events), secondEvents)
	}
}

func TestResiliencePanicConformanceFinalizesCircuitState(t *testing.T) {
	var events []resilience.Event
	breaker, err := resilience.NewCircuitBreaker[struct{}](resilience.CircuitBreakerOptions{
		Name:             "panic",
		FailureThreshold: 1,
		OpenTimeout:      time.Minute,
		OnEvent:          func(_ context.Context, event resilience.Event) { events = append(events, event) },
	})
	if err != nil {
		t.Fatalf("NewCircuitBreaker failed: %v", err)
	}

	var recovered any
	func() {
		defer func() { recovered = recover() }()
		_, _ = breaker.Apply(func(context.Context) (struct{}, error) { panic("boom") })(context.Background())
	}()
	if recovered != "boom" {
		t.Fatalf("recovered = %v, want boom", recovered)
	}
	if breaker.State() != resilience.CircuitStateOpen {
		t.Fatalf("state = %q, want open", breaker.State())
	}
	if len(events) != 1 || events[0].Kind != resilience.EventCircuitStateTransition {
		t.Fatalf("events = %#v, want one transition", events)
	}
}

type problemError struct{ problem web.Problem }

func (e problemError) Error() string               { return e.problem.Detail }
func (e problemError) ProblemDetails() web.Problem { return e.problem }

type stubLimiter struct {
	result ratelimit.Result
	err    error
	key    string
	tokens int64
}

func (l *stubLimiter) Allow(_ context.Context, key string, tokens int64) (ratelimit.Result, error) {
	l.key = key
	l.tokens = tokens
	return l.result, l.err
}

func resilienceAdapter(policies []resilience.Policy[struct{}], errorHandler resilience.HandlerErrorHandler) webtest.Adapter {
	return func(next http.Handler) http.Handler {
		handler := resilience.NewHandler(next, resilience.HandlerOptions{Policies: policies, ErrorHandler: errorHandler})
		return handler
	}
}

func ratelimitAdapter(limiter ratelimit.Limiter, keyFunc ratelimit.KeyFunc) webtest.Adapter {
	return func(next http.Handler) http.Handler {
		handler, err := ratelimit.NewHandler(next, ratelimit.HandlerOptions{Limiter: limiter, KeyFunc: keyFunc})
		if err != nil {
			return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				http.Error(w, err.Error(), http.StatusInternalServerError)
			})
		}
		return handler
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) { return fn(req) }
