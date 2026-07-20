package ratelimit_test

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/bluetape4k/bluetape-go/ratelimit"
	redisratelimit "github.com/bluetape4k/bluetape-go/ratelimit/redis"
	sqlratelimit "github.com/bluetape4k/bluetape-go/ratelimit/sql"
	postgrestestcontainer "github.com/bluetape4k/bluetape-go/testcontainers/postgres"
	redistestcontainer "github.com/bluetape4k/bluetape-go/testcontainers/redis"
	"github.com/redis/go-redis/v9"

	_ "github.com/jackc/pgx/v5/stdlib"
)

const (
	rateLimitProviderBenchmarkEnv   = "BLUETAPE_RATELIMIT_PROVIDER_BENCH"
	rateLimitProviderStartupLimit   = 90 * time.Second
	rateLimitProviderOperationLimit = 10 * time.Second
	rateLimitProviderCleanupLimit   = 10 * time.Second
	rateLimitProviderWorkers        = 8
	rateLimitProviderCapacity       = int64(4)
	rateLimitProviderRefill         = 0.001
	rateLimitProviderIdleTTL        = 3 * time.Hour
)

var rateLimitProviderBenchmarkSink []rateLimitRoundResult

type rateLimitRoundOptions struct {
	workers      int
	attemptLimit time.Duration
	roundLimit   time.Duration
}

type rateLimitRoundResult struct {
	worker int
	result ratelimit.Result
}

func runRateLimitRound(
	ctx context.Context,
	opts rateLimitRoundOptions,
	worker func(context.Context, int) (ratelimit.Result, error),
) ([]rateLimitRoundResult, error) {
	if ctx == nil {
		return nil, errors.New("rate-limit round context must not be nil")
	}
	if opts.workers <= 0 || opts.attemptLimit <= 0 || opts.roundLimit <= 0 || worker == nil {
		return nil, errors.New("rate-limit round requires positive limits and a worker")
	}

	deadlineCtx, cancelDeadline := context.WithTimeout(ctx, opts.roundLimit)
	defer cancelDeadline()
	roundCtx, cancelRound := context.WithCancel(deadlineCtx)
	defer cancelRound()

	start := make(chan struct{})
	ready := make(chan struct{}, opts.workers)
	results := make(chan rateLimitRoundResult, opts.workers)
	var workers sync.WaitGroup
	var firstErr error
	var firstErrOnce sync.Once

	for workerID := range opts.workers {
		workers.Add(1)
		go func() {
			defer workers.Done()
			ready <- struct{}{}
			<-start

			attemptCtx, cancelAttempt := context.WithTimeout(roundCtx, opts.attemptLimit)
			result, err := worker(attemptCtx, workerID)
			cancelAttempt()
			if err != nil {
				firstErrOnce.Do(func() {
					firstErr = fmt.Errorf("rate-limit round worker %d: %w", workerID, err)
					cancelRound()
				})
			}
			results <- rateLimitRoundResult{worker: workerID, result: result}
		}()
	}

	for range opts.workers {
		<-ready
	}
	close(start)

	joined := make(chan struct{})
	go func() {
		workers.Wait()
		close(results)
		close(joined)
	}()

	collected := make([]rateLimitRoundResult, 0, opts.workers)
	for result := range results {
		collected = append(collected, result)
	}
	<-joined
	return collected, firstErr
}

func countRateLimitAvailable(results []rateLimitRoundResult) int {
	available := 0
	for _, result := range results {
		if result.result.Allowed {
			available++
		}
	}
	return available
}

type rateLimitProviderCleanup struct {
	deleteNamespace func(context.Context) error
	closeClient     func(context.Context) error
}

func (cleanup rateLimitProviderCleanup) run(ctx context.Context) error {
	namespaceErr := runRateLimitProviderCleanupStage(ctx, cleanup.deleteNamespace)
	clientErr := runRateLimitProviderCleanupStage(ctx, cleanup.closeClient)
	return errors.Join(namespaceErr, clientErr)
}

func runRateLimitProviderCleanupStage(ctx context.Context, cleanup func(context.Context) error) error {
	if cleanup == nil {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), rateLimitProviderCleanupLimit)
	defer cancel()
	return cleanup(cleanupCtx)
}

func registerRateLimitProviderCleanup(tb testing.TB, name string, cleanup rateLimitProviderCleanup) {
	tb.Helper()
	tb.Cleanup(func() {
		if err := cleanup.run(context.Background()); err != nil {
			tb.Errorf("cleanup %s rate-limit provider fixture: %v", name, err)
		}
	})
}

type rateLimitProviderFixture struct {
	name            string
	providerVersion string
	imageReference  string
	limiter         ratelimit.Limiter
	resetNamespace  func(context.Context) error
}

type rateLimitProviderFactory struct {
	name string
	open func(testing.TB) rateLimitProviderFixture
}

var rateLimitProviderFactories = []rateLimitProviderFactory{
	{name: "Redis", open: newRedisRateLimitProviderFixture},
	{name: "PostgreSQL", open: newPostgresRateLimitProviderFixture},
}

var rateLimitDistinctKeys = [...]string{
	"distinct-0",
	"distinct-1",
	"distinct-2",
	"distinct-3",
	"distinct-4",
	"distinct-5",
	"distinct-6",
	"distinct-7",
}

func BenchmarkProviderRateLimitLocal(b *testing.B) {
	b.Run("Local", func(b *testing.B) {
		b.Run("TokenBucket", func(b *testing.B) {
			b.Log("benchmark_class=algorithm_baseline provider_ranking=false")
			b.Run("AllowAvailable", func(b *testing.B) {
				runLocalRateLimitBenchmark(b, "AllowAvailable")
			})
			b.Run("AllowRejected", func(b *testing.B) {
				runLocalRateLimitBenchmark(b, "AllowRejected")
			})
		})
	})
}

func runLocalRateLimitBenchmark(b *testing.B, scenario string) {
	b.Helper()
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		b.StopTimer()
		limiter, err := ratelimit.New(ratelimit.Options{
			RatePerSecond: rateLimitProviderRefill,
			Burst:         rateLimitProviderCapacity,
			IdleTTL:       rateLimitProviderIdleTTL,
		})
		if err != nil {
			b.Fatalf("new local token bucket: %v", err)
		}
		if scenario == "AllowRejected" {
			if err := seedRejectedRateLimit(limiter, "benchmark-key"); err != nil {
				b.Fatalf("seed local rejected bucket: %v", err)
			}
		}

		b.StartTimer()
		results, runErr := runSingleRateLimitAllow(limiter, "benchmark-key")
		b.StopTimer()

		rateLimitProviderBenchmarkSink = results
		if err := errors.Join(runErr, verifyRateLimitScenario(scenario, results)); err != nil {
			b.Fatalf("local token bucket %s: %v", scenario, err)
		}
	}
}

func BenchmarkProviderRateLimitContainers(b *testing.B) {
	if os.Getenv(rateLimitProviderBenchmarkEnv) != "1" {
		b.Skipf("set %s=1 to run provider container benchmarks", rateLimitProviderBenchmarkEnv)
	}
	for _, factory := range rateLimitProviderFactories {
		b.Run(factory.name, func(b *testing.B) {
			b.StopTimer()
			fixture := factory.open(b)
			b.Logf(
				"provider_version=%q image_reference=%q",
				sanitizeRateLimitProviderMetadata(fixture.providerVersion),
				sanitizeRateLimitProviderMetadata(fixture.imageReference),
			)
			b.Run("AllowAvailable", func(b *testing.B) {
				runContainerRateLimitBenchmark(b, fixture, "AllowAvailable")
			})
			b.Run("AllowRejected", func(b *testing.B) {
				runContainerRateLimitBenchmark(b, fixture, "AllowRejected")
			})
			b.Run("AllowParallel/N=8", func(b *testing.B) {
				runContainerRateLimitBenchmark(b, fixture, "AllowParallel")
			})
			b.Run("AllowDistinctKeys/N=8", func(b *testing.B) {
				runContainerRateLimitBenchmark(b, fixture, "AllowDistinctKeys")
			})
		})
	}
}

func runContainerRateLimitBenchmark(b *testing.B, fixture rateLimitProviderFixture, scenario string) {
	b.Helper()
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		b.StopTimer()
		resetCtx, resetCancel := rateLimitProviderOperationContext()
		resetErr := fixture.resetNamespace(resetCtx)
		resetCancel()
		if resetErr != nil {
			b.Fatalf("reset %s namespace: %v", fixture.name, resetErr)
		}
		if scenario == "AllowRejected" {
			if err := seedRejectedRateLimit(fixture.limiter, "benchmark-key"); err != nil {
				b.Fatalf("seed %s rejected bucket: %v", fixture.name, err)
			}
		}

		b.StartTimer()
		results, runErr := executeRateLimitScenario(fixture.limiter, scenario)
		b.StopTimer()

		rateLimitProviderBenchmarkSink = results
		if err := errors.Join(runErr, verifyRateLimitScenario(scenario, results)); err != nil {
			b.Fatalf("%s/%s: %v", fixture.name, scenario, err)
		}
	}
}

func executeRateLimitScenario(limiter ratelimit.Limiter, scenario string) ([]rateLimitRoundResult, error) {
	switch scenario {
	case "AllowAvailable", "AllowRejected":
		return runSingleRateLimitAllow(limiter, "benchmark-key")
	case "AllowParallel":
		ctx, cancel := rateLimitProviderOperationContext()
		defer cancel()
		return runRateLimitRound(ctx, rateLimitRoundOptions{
			workers:      rateLimitProviderWorkers,
			attemptLimit: rateLimitProviderOperationLimit,
			roundLimit:   rateLimitProviderOperationLimit,
		}, func(ctx context.Context, _ int) (ratelimit.Result, error) {
			return limiter.Allow(ctx, "parallel-key", 1)
		})
	case "AllowDistinctKeys":
		ctx, cancel := rateLimitProviderOperationContext()
		defer cancel()
		return runRateLimitRound(ctx, rateLimitRoundOptions{
			workers:      rateLimitProviderWorkers,
			attemptLimit: rateLimitProviderOperationLimit,
			roundLimit:   rateLimitProviderOperationLimit,
		}, func(ctx context.Context, worker int) (ratelimit.Result, error) {
			return limiter.Allow(ctx, rateLimitDistinctKeys[worker], 1)
		})
	default:
		return nil, fmt.Errorf("unknown rate-limit benchmark scenario %q", scenario)
	}
}

func runSingleRateLimitAllow(limiter ratelimit.Limiter, key string) ([]rateLimitRoundResult, error) {
	ctx, cancel := rateLimitProviderOperationContext()
	defer cancel()
	result, err := limiter.Allow(ctx, key, 1)
	return []rateLimitRoundResult{{worker: 0, result: result}}, err
}

func seedRejectedRateLimit(limiter ratelimit.Limiter, key string) error {
	ctx, cancel := rateLimitProviderOperationContext()
	defer cancel()
	result, err := limiter.Allow(ctx, key, rateLimitProviderCapacity)
	if err != nil {
		return err
	}
	if !result.Allowed || result.Requested != rateLimitProviderCapacity {
		return fmt.Errorf("seed result = %+v, want allowed capacity request", result)
	}
	return nil
}

func verifyRateLimitScenario(scenario string, results []rateLimitRoundResult) error {
	wantResults := 1
	wantAvailable := 1
	switch scenario {
	case "AllowAvailable":
	case "AllowRejected":
		wantAvailable = 0
	case "AllowParallel":
		wantResults = rateLimitProviderWorkers
		wantAvailable = int(rateLimitProviderCapacity)
	case "AllowDistinctKeys":
		wantResults = rateLimitProviderWorkers
		wantAvailable = rateLimitProviderWorkers
	default:
		return fmt.Errorf("unknown rate-limit benchmark scenario %q", scenario)
	}
	if len(results) != wantResults {
		return fmt.Errorf("results = %d, want %d", len(results), wantResults)
	}
	available := countRateLimitAvailable(results)
	if available != wantAvailable {
		return fmt.Errorf("available results = %d, want %d", available, wantAvailable)
	}
	if scenario == "AllowParallel" && int64(available) > rateLimitProviderCapacity {
		return fmt.Errorf("parallel available results = %d, exceeds capacity %d", available, rateLimitProviderCapacity)
	}

	fullRefill := time.Duration(float64(rateLimitProviderCapacity)/rateLimitProviderRefill) * time.Second
	for _, roundResult := range results {
		result := roundResult.result
		if result.Requested != 1 {
			return fmt.Errorf("worker %d requested = %d, want 1", roundResult.worker, result.Requested)
		}
		if result.Remaining < 0 || result.Remaining > rateLimitProviderCapacity {
			return fmt.Errorf("worker %d remaining = %d, want range [0,%d]", roundResult.worker, result.Remaining, rateLimitProviderCapacity)
		}
		if result.RetryAfter < 0 || result.RetryAfter > fullRefill {
			return fmt.Errorf("worker %d retry after = %s, want range [0,%s]", roundResult.worker, result.RetryAfter, fullRefill)
		}
		if result.ResetAfter < 0 || result.ResetAfter > fullRefill {
			return fmt.Errorf("worker %d reset after = %s, want range [0,%s]", roundResult.worker, result.ResetAfter, fullRefill)
		}
		if result.Allowed && result.RetryAfter != 0 {
			return fmt.Errorf("worker %d allowed retry after = %s, want 0", roundResult.worker, result.RetryAfter)
		}
		if !result.Allowed && result.RetryAfter == 0 {
			return fmt.Errorf("worker %d rejected retry after = 0", roundResult.worker)
		}
	}
	return nil
}

func newRedisRateLimitProviderFixture(tb testing.TB) rateLimitProviderFixture {
	tb.Helper()
	ctx, cancel := rateLimitProviderSetupContext()
	defer cancel()
	server := redistestcontainer.StartServer(ctx, tb)
	details, err := server.ConnectionDetails(ctx)
	if err != nil {
		tb.Fatalf("redis connection details: %v", err)
	}
	address, err := details.Require(redistestcontainer.AddressKey)
	if err != nil {
		tb.Fatalf("redis address: %v", err)
	}
	namespace := mustRateLimitProviderNamespace(tb)
	client := redis.NewClient(&redis.Options{Addr: address})
	deleteNamespace := func(ctx context.Context) error {
		return deleteRedisRateLimitNamespace(ctx, client, namespace)
	}
	registerRateLimitProviderCleanup(tb, "Redis", rateLimitProviderCleanup{
		deleteNamespace: deleteNamespace,
		closeClient: func(context.Context) error {
			return client.Close()
		},
	})
	if err := client.Ping(ctx).Err(); err != nil {
		tb.Fatalf("ping Redis rate-limit provider: %v", err)
	}
	limiter, err := redisratelimit.New(redisratelimit.Options{
		Client:        client,
		Namespace:     namespace,
		RatePerSecond: rateLimitProviderRefill,
		Burst:         rateLimitProviderCapacity,
		IdleTTL:       rateLimitProviderIdleTTL,
	})
	if err != nil {
		tb.Fatalf("new Redis rate limiter: %v", err)
	}
	return rateLimitProviderFixture{
		name:            "Redis",
		providerVersion: "7.4",
		imageReference:  "redis:7.4-alpine@sha256:6ab0b6e7381779332f97b8ca76193e45b0756f38d4c0dcda72dbb3c32061ab99",
		limiter:         limiter,
		resetNamespace:  deleteNamespace,
	}
}

func newPostgresRateLimitProviderFixture(tb testing.TB) rateLimitProviderFixture {
	tb.Helper()
	ctx, cancel := rateLimitProviderSetupContext()
	defer cancel()
	server := postgrestestcontainer.StartServer(ctx, tb)
	details, err := server.ConnectionDetails(ctx)
	if err != nil {
		tb.Fatalf("PostgreSQL connection details: %v", err)
	}
	dsn, err := details.Require(postgrestestcontainer.ConnectionStringKey)
	if err != nil {
		tb.Fatalf("PostgreSQL connection string: %v", err)
	}
	namespace := mustRateLimitProviderNamespace(tb)
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		tb.Fatalf("open PostgreSQL rate-limit provider: %v", err)
	}
	deleteNamespace := func(ctx context.Context) error {
		_, err := db.ExecContext(ctx, `delete from public.bluetape_ratelimit_buckets where namespace=$1`, []byte(namespace))
		return err
	}
	registerRateLimitProviderCleanup(tb, "PostgreSQL", rateLimitProviderCleanup{
		deleteNamespace: deleteNamespace,
		closeClient: func(context.Context) error {
			return db.Close()
		},
	})
	if err := db.PingContext(ctx); err != nil {
		tb.Fatalf("ping PostgreSQL rate-limit provider: %v", err)
	}
	if _, err := db.ExecContext(ctx, sqlratelimit.SchemaSQL); err != nil {
		tb.Fatalf("create PostgreSQL rate-limit schema: %v", err)
	}
	limiter, err := sqlratelimit.New(db, sqlratelimit.Options{
		Namespace:     namespace,
		RatePerSecond: rateLimitProviderRefill,
		Burst:         rateLimitProviderCapacity,
		IdleTTL:       rateLimitProviderIdleTTL,
	})
	if err != nil {
		tb.Fatalf("new PostgreSQL rate limiter: %v", err)
	}
	return rateLimitProviderFixture{
		name:            "PostgreSQL",
		providerVersion: "16",
		imageReference:  "postgres:16-alpine@sha256:57c72fd2a128e416c7fcc499958864df5301e940bca0a56f58fddf30ffc07777",
		limiter:         limiter,
		resetNamespace:  deleteNamespace,
	}
}

func deleteRedisRateLimitNamespace(ctx context.Context, client *redis.Client, namespace string) error {
	pattern := "bluetape:ratelimit:" + namespace + ":bucket:*"
	var cursor uint64
	var joined error
	for {
		keys, next, err := client.Scan(ctx, cursor, pattern, 128).Result()
		if err != nil {
			return errors.Join(joined, err)
		}
		if len(keys) > 0 {
			joined = errors.Join(joined, client.Del(ctx, keys...).Err())
		}
		cursor = next
		if cursor == 0 {
			return joined
		}
	}
}

func rateLimitProviderSetupContext() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), rateLimitProviderStartupLimit)
}

func rateLimitProviderOperationContext() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), rateLimitProviderOperationLimit)
}

func mustRateLimitProviderNamespace(tb testing.TB) string {
	tb.Helper()
	var value [16]byte
	if _, err := rand.Read(value[:]); err != nil {
		tb.Fatalf("generate rate-limit provider namespace: %v", err)
	}
	return hex.EncodeToString(value[:])
}

func sanitizeRateLimitProviderMetadata(value string) string {
	return strings.Map(func(r rune) rune {
		if r < 0x20 || r == 0x7f {
			return -1
		}
		return r
	}, value)
}

func TestRunRateLimitRoundStartsEightWorkersAndJoins(t *testing.T) {
	const workers = 8
	entered := make(chan struct{}, workers)
	release := make(chan struct{})
	var active atomic.Int32

	type outcome struct {
		results []rateLimitRoundResult
		err     error
	}
	done := make(chan outcome, 1)
	go func() {
		results, err := runRateLimitRound(context.Background(), rateLimitRoundOptions{
			workers:      workers,
			attemptLimit: time.Second,
			roundLimit:   2 * time.Second,
		}, func(context.Context, int) (ratelimit.Result, error) {
			active.Add(1)
			defer active.Add(-1)
			entered <- struct{}{}
			<-release
			return ratelimit.Result{Allowed: true, Requested: 1}, nil
		})
		done <- outcome{results: results, err: err}
	}()

	for range workers {
		select {
		case <-entered:
		case <-time.After(time.Second):
			t.Fatal("workers did not cross the start barrier")
		}
	}
	if got := active.Load(); got != workers {
		t.Fatalf("active workers before release = %d, want %d", got, workers)
	}
	close(release)

	select {
	case got := <-done:
		if got.err != nil {
			t.Fatalf("run rate-limit round: %v", got.err)
		}
		if len(got.results) != workers {
			t.Fatalf("results = %d, want %d", len(got.results), workers)
		}
		if available := countRateLimitAvailable(got.results); available != workers {
			t.Fatalf("available results = %d, want %d", available, workers)
		}
	case <-time.After(time.Second):
		t.Fatal("rate-limit round did not join within the bound")
	}
	if got := active.Load(); got != 0 {
		t.Fatalf("active workers after return = %d, want 0", got)
	}
}

func TestRunRateLimitRoundCancelsPeersAndDrainsOnFirstError(t *testing.T) {
	const workers = 8
	injected := errors.New("injected worker-0 failure")
	var active atomic.Int32

	started := time.Now()
	results, err := runRateLimitRound(context.Background(), rateLimitRoundOptions{
		workers:      workers,
		attemptLimit: time.Second,
		roundLimit:   2 * time.Second,
	}, func(ctx context.Context, worker int) (ratelimit.Result, error) {
		active.Add(1)
		defer active.Add(-1)
		if worker == 0 {
			return ratelimit.Result{}, injected
		}
		<-ctx.Done()
		return ratelimit.Result{}, ctx.Err()
	})
	elapsed := time.Since(started)

	if !errors.Is(err, injected) {
		t.Fatalf("run rate-limit round error = %v, want injected error", err)
	}
	if elapsed >= time.Second {
		t.Fatalf("run rate-limit round returned after %s, want under 1s", elapsed)
	}
	if len(results) != workers {
		t.Fatalf("results = %d, want %d", len(results), workers)
	}
	if got := active.Load(); got != 0 {
		t.Fatalf("active workers after return = %d, want 0", got)
	}
}

func TestRateLimitProviderCleanupRunsNamespaceThenClientAndJoinsErrors(t *testing.T) {
	namespaceErr := errors.New("injected namespace cleanup failure")
	clientErr := errors.New("injected client close failure")
	var order []string

	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	coordinator := rateLimitProviderCleanup{
		deleteNamespace: func(ctx context.Context) error {
			if ctx.Err() != nil {
				t.Fatalf("namespace cleanup inherited cancellation: %v", ctx.Err())
			}
			if _, ok := ctx.Deadline(); !ok {
				t.Fatal("namespace cleanup context has no deadline")
			}
			order = append(order, "namespace")
			return namespaceErr
		},
		closeClient: func(ctx context.Context) error {
			if ctx.Err() != nil {
				t.Fatalf("client cleanup inherited cancellation: %v", ctx.Err())
			}
			if _, ok := ctx.Deadline(); !ok {
				t.Fatal("client cleanup context has no deadline")
			}
			order = append(order, "client")
			return clientErr
		},
	}

	err := coordinator.run(canceled)
	if !reflect.DeepEqual(order, []string{"namespace", "client"}) {
		t.Fatalf("cleanup order = %v, want [namespace client]", order)
	}
	if !errors.Is(err, namespaceErr) || !errors.Is(err, clientErr) {
		t.Fatalf("cleanup error = %v, want both injected errors", err)
	}
}
