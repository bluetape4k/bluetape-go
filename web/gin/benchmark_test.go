package ginadapter_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/bluetape4k/bluetape-go/jwt"
	"github.com/bluetape4k/bluetape-go/ratelimit"
	"github.com/bluetape4k/bluetape-go/resilience"
	"github.com/bluetape4k/bluetape-go/web"
	ginadapter "github.com/bluetape4k/bluetape-go/web/gin"
	"github.com/gin-gonic/gin"
)

const benchmarkRequestURL = "http://example.test/orders"

func BenchmarkGinAdapter(b *testing.B) {
	gin.SetMode(gin.TestMode)
	b.ReportAllocs()
	b.StopTimer()
	fixture := newGinBenchmarkFixture(b)
	b.StartTimer()

	benchmarks := []struct {
		name    string
		handler http.Handler
		token   string
	}{
		{name: "NoOp", handler: fixture.noOp},
		{name: "DirectCore", handler: fixture.directCore},
		{name: "Bridge", handler: fixture.bridge},
		{name: "FullAdapter", handler: fixture.fullAdapter, token: fixture.token},
	}
	for _, benchmark := range benchmarks {
		benchmark := benchmark
		b.Run(benchmark.name+"/Serial", func(b *testing.B) {
			b.ReportAllocs()
			runGinSerialBenchmark(b, benchmark.handler, benchmark.token)
		})
		b.Run(benchmark.name+"/Parallel", func(b *testing.B) {
			b.ReportAllocs()
			runGinParallelBenchmark(b, benchmark.handler, benchmark.token)
		})
	}
}

func BenchmarkGinAdapterColdConstruction(b *testing.B) {
	gin.SetMode(gin.TestMode)
	b.ReportAllocs()
	b.StopTimer()
	provider, token, limiter := newGinBenchmarkDependencies(b)
	b.StartTimer()
	for i := 0; i < b.N; i++ {
		if _, err := buildFullGinAdapter(provider, token, limiter); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkGinAdapterColdFirstRequest(b *testing.B) {
	gin.SetMode(gin.TestMode)
	b.ReportAllocs()
	b.StopTimer()
	provider, token, limiter := newGinBenchmarkDependencies(b)
	b.StartTimer()
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		handler, err := buildFullGinAdapter(provider, token, limiter)
		if err != nil {
			b.Fatal(err)
		}
		request := newBenchmarkRequest(token)
		recorder := httptest.NewRecorder()
		b.StartTimer()
		handler.ServeHTTP(recorder, request)
		b.StopTimer()
		if recorder.Code != http.StatusNoContent {
			b.Fatalf("status = %d, want 204", recorder.Code)
		}
	}
}

func BenchmarkGinAdapterWarmRequest(b *testing.B) {
	gin.SetMode(gin.TestMode)
	b.ReportAllocs()
	b.StopTimer()
	fixture := newGinBenchmarkFixture(b)
	for i := 0; i < 10; i++ {
		record := httptest.NewRecorder()
		fixture.fullAdapter.ServeHTTP(record, newBenchmarkRequest(fixture.token))
	}
	b.StartTimer()
	b.Run("Serial", func(b *testing.B) {
		b.ReportAllocs()
		runGinSerialBenchmark(b, fixture.fullAdapter, fixture.token)
	})
	b.Run("Parallel", func(b *testing.B) {
		b.ReportAllocs()
		runGinParallelBenchmark(b, fixture.fullAdapter, fixture.token)
	})
}

type ginBenchmarkFixture struct {
	noOp        http.Handler
	directCore  http.Handler
	bridge      http.Handler
	fullAdapter http.Handler
	token       string
}

func newGinBenchmarkFixture(b *testing.B) ginBenchmarkFixture {
	b.Helper()
	provider, token, limiter := newGinBenchmarkDependencies(b)
	bridge, err := buildBridgeGinAdapter()
	if err != nil {
		b.Fatal(err)
	}
	full, err := buildFullGinAdapter(provider, token, limiter)
	if err != nil {
		b.Fatal(err)
	}
	noOp := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
	direct := http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		requestWithContext, _, err := web.WithRequestContextOnRequest(request, web.RequestContextOptions{GenerateID: func() (string, error) { return "benchmark-request", nil }})
		if err != nil {
			http.Error(w, "request context failed", http.StatusInternalServerError)
			return
		}
		noOp.ServeHTTP(w, requestWithContext)
	})
	return ginBenchmarkFixture{noOp: noOp, directCore: direct, bridge: bridge, fullAdapter: full, token: token}
}

func newGinBenchmarkDependencies(b *testing.B) (*jwt.Provider, string, ratelimit.Limiter) {
	b.Helper()
	provider, err := jwt.NewFixedHMACProvider(jwt.HS256, []byte("0123456789abcdef0123456789abcdef"))
	if err != nil {
		b.Fatal(err)
	}
	token, err := provider.Compose(jwt.WithSubject("benchmark-account"))
	if err != nil {
		b.Fatal(err)
	}
	return provider, token, benchmarkLimiter{}
}

func buildBridgeGinAdapter() (http.Handler, error) {
	router := gin.New()
	router.Use(ginadapter.RequestContext(web.RequestContextOptions{GenerateID: func() (string, error) { return "benchmark-request", nil }}))
	router.GET("/orders", func(c *gin.Context) { c.Status(http.StatusNoContent) })
	return router, nil
}

func buildFullGinAdapter(provider jwt.Parser, token string, limiter ratelimit.Limiter) (http.Handler, error) {
	rateLimit, err := ginadapter.NewRateLimit(ginadapter.RateLimitOptions{Limiter: limiter})
	if err != nil {
		return nil, err
	}
	authentication, err := ginadapter.NewJWT(ginadapter.JWTOptions{Parser: provider})
	if err != nil {
		return nil, err
	}
	router := gin.New()
	router.Use(ginadapter.RequestContext(web.RequestContextOptions{GenerateID: func() (string, error) { return "benchmark-request", nil }}))
	router.Use(rateLimit, authentication)
	router.GET("/orders", ginadapter.WrapResilience(func(c *gin.Context) {
		if _, ok := ginadapter.JWTReader(c, ""); !ok {
			c.Status(http.StatusInternalServerError)
			return
		}
		c.Status(http.StatusNoContent)
	}, ginadapter.ResilienceOptions{Policies: []resilience.Policy[struct{}]{}}))
	return router, nil
}

func newBenchmarkRequest(token string) *http.Request {
	request := httptest.NewRequest(http.MethodGet, benchmarkRequestURL, nil)
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	return request
}

func runGinSerialBenchmark(b *testing.B, handler http.Handler, token string) {
	b.Helper()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, newBenchmarkRequest(token))
		if recorder.Code != http.StatusNoContent {
			b.Fatalf("status = %d, want 204", recorder.Code)
		}
	}
}

func runGinParallelBenchmark(b *testing.B, handler http.Handler, token string) {
	b.Helper()
	workers := runtime.GOMAXPROCS(0)
	if workers < 1 {
		workers = 1
	}
	if workers > b.N {
		workers = b.N
	}
	start := make(chan struct{})
	var next atomic.Int64
	var completed atomic.Int64
	var failures atomic.Int64
	var wait sync.WaitGroup
	wait.Add(workers)
	for i := 0; i < workers; i++ {
		go func() {
			defer wait.Done()
			<-start
			for {
				index := next.Add(1) - 1
				if index >= int64(b.N) {
					return
				}
				recorder := httptest.NewRecorder()
				handler.ServeHTTP(recorder, newBenchmarkRequest(token))
				if recorder.Code != http.StatusNoContent {
					failures.Add(1)
					continue
				}
				completed.Add(1)
			}
		}()
	}
	b.ResetTimer()
	close(start)
	wait.Wait()
	b.StopTimer()
	if failures.Load() != 0 {
		b.Fatalf("parallel requests failed: %d", failures.Load())
	}
	if completed.Load() != int64(b.N) {
		b.Fatalf("completed requests = %d, want %d", completed.Load(), b.N)
	}
}

type benchmarkLimiter struct{}

func (benchmarkLimiter) Allow(ctx context.Context, _ string, tokens int64) (ratelimit.Result, error) {
	if err := ctx.Err(); err != nil {
		return ratelimit.Result{}, err
	}
	return ratelimit.Result{Allowed: tokens > 0, Remaining: 100}, nil
}

var _ ratelimit.Limiter = benchmarkLimiter{}
