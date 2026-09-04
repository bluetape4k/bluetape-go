package echoadapter_test

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/bluetape4k/bluetape-go/jwt"
	"github.com/bluetape4k/bluetape-go/ratelimit"
	"github.com/bluetape4k/bluetape-go/resilience"
	"github.com/bluetape4k/bluetape-go/web"
	echoadapter "github.com/bluetape4k/bluetape-go/web/echo"
	"github.com/labstack/echo/v4"
)

const echoBenchmarkRequestURL = "http://example.test/orders"

// BenchmarkEchoAdapter measures request-path overhead for the Echo adapter.
func BenchmarkEchoAdapter(b *testing.B) {
	b.ReportAllocs()
	b.StopTimer()
	fixture := newEchoBenchmarkFixture(b)
	b.StartTimer()

	benchmarks := []struct {
		name           string
		handler        http.Handler
		token          string
		requestFactory func(string) *http.Request
	}{
		{name: "NoOp", handler: fixture.noOp},
		{name: "DirectCore", handler: fixture.directCore},
		{name: "Bridge", handler: fixture.bridge},
		{name: "FullAdapter", handler: fixture.fullAdapter, token: fixture.token},
		{
			name:           "FullAdapterRetry",
			handler:        fixture.fullAdapterRetry,
			token:          fixture.token,
			requestFactory: newEchoBenchmarkRetryRequest,
		},
		{
			name:           "ReplayableBody",
			handler:        fixture.fullAdapterRetry,
			token:          fixture.token,
			requestFactory: newEchoBenchmarkBodyRequest,
		},
	}
	for _, benchmark := range benchmarks {
		benchmark := benchmark
		b.Run(benchmark.name+"/Serial", func(b *testing.B) {
			b.ReportAllocs()
			runEchoSerialBenchmark(b, benchmark.handler, benchmark.token, benchmark.requestFactory)
		})
		b.Run(benchmark.name+"/Parallel", func(b *testing.B) {
			b.ReportAllocs()
			runEchoParallelBenchmark(b, benchmark.handler, benchmark.token, benchmark.requestFactory)
		})
	}
}

// BenchmarkEchoAdapterColdConstruction measures middleware construction cost.
func BenchmarkEchoAdapterColdConstruction(b *testing.B) {
	b.ReportAllocs()
	b.StopTimer()
	provider, _, limiter := newEchoBenchmarkDependencies(b)
	b.StartTimer()
	for i := 0; i < b.N; i++ {
		if _, err := buildFullEchoAdapter(provider, limiter); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkEchoAdapterColdFirstRequest separates construction from first request.
func BenchmarkEchoAdapterColdFirstRequest(b *testing.B) {
	b.ReportAllocs()
	b.StopTimer()
	provider, token, limiter := newEchoBenchmarkDependencies(b)
	b.StartTimer()
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		handler, err := buildFullEchoAdapter(provider, limiter)
		if err != nil {
			b.Fatal(err)
		}
		b.StartTimer()
		status := serveEchoBenchmarkRequest(handler, token, nil)
		b.StopTimer()
		if status != http.StatusNoContent {
			b.Fatalf("status = %d, want 204", status)
		}
		b.StartTimer()
	}
}

// BenchmarkEchoAdapterWarmRequest measures a reused Echo handler.
func BenchmarkEchoAdapterWarmRequest(b *testing.B) {
	b.ReportAllocs()
	b.StopTimer()
	fixture := newEchoBenchmarkFixture(b)
	for i := 0; i < 10; i++ {
		recorder := httptest.NewRecorder()
		fixture.fullAdapter.ServeHTTP(recorder, newEchoBenchmarkRequest(fixture.token))
	}
	b.StartTimer()
	b.Run("Serial", func(b *testing.B) {
		b.ReportAllocs()
		runEchoSerialBenchmark(b, fixture.fullAdapter, fixture.token, nil)
	})
	b.Run("Parallel", func(b *testing.B) {
		b.ReportAllocs()
		runEchoParallelBenchmark(b, fixture.fullAdapter, fixture.token, nil)
	})
}

type echoBenchmarkFixture struct {
	noOp             http.Handler
	directCore       http.Handler
	bridge           http.Handler
	fullAdapter      http.Handler
	fullAdapterRetry http.Handler
	token            string
}

func newEchoBenchmarkFixture(b *testing.B) echoBenchmarkFixture {
	b.Helper()
	provider, token, limiter := newEchoBenchmarkDependencies(b)
	bridge, err := buildBridgeEchoAdapter()
	if err != nil {
		b.Fatal(err)
	}
	full, err := buildFullEchoAdapter(provider, limiter)
	if err != nil {
		b.Fatal(err)
	}
	fullRetry, err := buildFullEchoAdapterWithRetry(provider, limiter)
	if err != nil {
		b.Fatal(err)
	}
	noOp := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
	direct := http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		requestWithContext, _, err := web.WithRequestContextOnRequest(request, web.RequestContextOptions{
			GenerateID: func() (string, error) { return "benchmark-request", nil },
		})
		if err != nil {
			http.Error(w, "request context failed", http.StatusInternalServerError)
			return
		}
		noOp.ServeHTTP(w, requestWithContext)
	})
	return echoBenchmarkFixture{
		noOp:             noOp,
		directCore:       direct,
		bridge:           bridge,
		fullAdapter:      full,
		fullAdapterRetry: fullRetry,
		token:            token,
	}
}

func newEchoBenchmarkDependencies(b *testing.B) (*jwt.Provider, string, ratelimit.Limiter) {
	b.Helper()
	provider, err := jwt.NewFixedHMACProvider(jwt.HS256, []byte("0123456789abcdef0123456789abcdef"))
	if err != nil {
		b.Fatal(err)
	}
	token, err := provider.Compose(jwt.WithSubject("benchmark-account"))
	if err != nil {
		b.Fatal(err)
	}
	return provider, token, echoBenchmarkLimiter{}
}

func buildBridgeEchoAdapter() (http.Handler, error) {
	server := echo.New()
	server.Use(echoadapter.RequestContext(web.RequestContextOptions{
		GenerateID: func() (string, error) { return "benchmark-request", nil },
	}))
	server.GET("/orders", func(c echo.Context) error { return c.NoContent(http.StatusNoContent) })
	return server, nil
}

func buildFullEchoAdapter(provider jwt.Parser, limiter ratelimit.Limiter) (http.Handler, error) {
	return buildFullEchoAdapterWithPolicies(provider, limiter)
}

func buildFullEchoAdapterWithRetry(provider jwt.Parser, limiter ratelimit.Limiter) (http.Handler, error) {
	retry, err := resilience.NewRetry[struct{}](resilience.RetryOptions{
		Name:        "echo-adapter-benchmark",
		MaxAttempts: 2,
	})
	if err != nil {
		return nil, err
	}
	return buildFullEchoAdapterWithPolicies(provider, limiter, retry)
}

func buildFullEchoAdapterWithPolicies(
	provider jwt.Parser,
	limiter ratelimit.Limiter,
	policies ...resilience.Policy[struct{}],
) (http.Handler, error) {
	rateLimit, err := echoadapter.NewRateLimit(echoadapter.RateLimitOptions{Limiter: limiter})
	if err != nil {
		return nil, err
	}
	authentication, err := echoadapter.NewJWT(echoadapter.JWTOptions{Parser: provider})
	if err != nil {
		return nil, err
	}
	server := echo.New()
	server.Use(
		echoadapter.RequestContext(web.RequestContextOptions{
			GenerateID: func() (string, error) { return "benchmark-request", nil },
		}),
		rateLimit,
		authentication,
	)
	server.GET("/orders", echoadapter.WrapResilience(func(c echo.Context) error {
		return serveEchoBenchmarkRoute(c, false)
	}, echoadapter.ResilienceOptions{Policies: policies}))
	server.POST("/orders", echoadapter.WrapResilience(func(c echo.Context) error {
		return serveEchoBenchmarkRoute(c, true)
	}, echoadapter.ResilienceOptions{Policies: policies}))
	return server, nil
}

func newEchoBenchmarkRequest(token string) *http.Request {
	request := httptest.NewRequestWithContext(context.Background(), http.MethodGet, echoBenchmarkRequestURL, nil)
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	return request
}

func newEchoBenchmarkRetryRequest(token string) *http.Request {
	request := httptest.NewRequestWithContext(
		context.WithValue(context.Background(), echoBenchmarkRetryStateKey{}, &echoBenchmarkRetryState{}),
		http.MethodGet,
		echoBenchmarkRequestURL,
		nil,
	)
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	return request
}

func newEchoBenchmarkBodyRequest(token string) *http.Request {
	const payload = "benchmark-payload"
	request := httptest.NewRequestWithContext(
		context.WithValue(context.Background(), echoBenchmarkRetryStateKey{}, &echoBenchmarkRetryState{}),
		http.MethodPost,
		echoBenchmarkRequestURL,
		strings.NewReader(payload),
	)
	request.GetBody = func() (io.ReadCloser, error) {
		return io.NopCloser(strings.NewReader(payload)), nil
	}
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	return request
}

func serveEchoBenchmarkRoute(c echo.Context, bodyRequired bool) error {
	if _, ok := echoadapter.JWTReader(c, ""); !ok {
		return c.NoContent(http.StatusInternalServerError)
	}
	if bodyRequired {
		body, err := io.ReadAll(c.Request().Body)
		if err != nil || string(body) != "benchmark-payload" {
			return errors.New("benchmark request body was not replayed")
		}
	}
	if state, _ := c.Request().Context().Value(echoBenchmarkRetryStateKey{}).(*echoBenchmarkRetryState); state != nil && state.attempts.Add(1) == 1 {
		return errors.New("benchmark transient route failure")
	}
	return c.NoContent(http.StatusNoContent)
}

type echoBenchmarkRetryState struct {
	attempts atomic.Int32
}

type echoBenchmarkRetryStateKey struct{}

func runEchoSerialBenchmark(b *testing.B, handler http.Handler, token string, requestFactory func(string) *http.Request) {
	b.Helper()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if status := serveEchoBenchmarkRequest(handler, token, requestFactory); status != http.StatusNoContent {
			b.Fatalf("status = %d, want 204", status)
		}
	}
}

func runEchoParallelBenchmark(b *testing.B, handler http.Handler, token string, requestFactory func(string) *http.Request) {
	b.Helper()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			if status := serveEchoBenchmarkRequest(handler, token, requestFactory); status != http.StatusNoContent {
				b.Errorf("status = %d, want 204", status)
			}
		}
	})
}

func serveEchoBenchmarkRequest(handler http.Handler, token string, requestFactory func(string) *http.Request) int {
	recorder := httptest.NewRecorder()
	if requestFactory == nil {
		requestFactory = newEchoBenchmarkRequest
	}
	handler.ServeHTTP(recorder, requestFactory(token))
	return recorder.Code
}

type echoBenchmarkLimiter struct{}

func (echoBenchmarkLimiter) Allow(ctx context.Context, _ string, tokens int64) (ratelimit.Result, error) {
	if err := ctx.Err(); err != nil {
		return ratelimit.Result{}, err
	}
	return ratelimit.Result{Allowed: tokens > 0, Remaining: 100}, nil
}

var _ ratelimit.Limiter = echoBenchmarkLimiter{}
