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
	"sort"
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
	rateLimitProviderBenchmarkEnv     = "BLUETAPE_RATELIMIT_PROVIDER_BENCH"
	rateLimitProviderStartupLimit     = 90 * time.Second
	rateLimitProviderOperationLimit   = 10 * time.Second
	rateLimitProviderCleanupLimit     = 10 * time.Second
	rateLimitProviderWorkers          = 8
	rateLimitProviderCapacity         = int64(4)
	rateLimitProviderRefill           = 0.001
	rateLimitProviderIdleTTL          = 3 * time.Hour
	rateLimitRedisVersionAuthority    = "7.4"
	rateLimitPostgresVersionAuthority = "16"
	rateLimitRedisImageReference      = "redis:7.4-alpine@sha256:6ab0b6e7381779332f97b8ca76193e45b0756f38d4c0dcda72dbb3c32061ab99"
	rateLimitPostgresImageReference   = "postgres:16-alpine@sha256:57c72fd2a128e416c7fcc499958864df5301e940bca0a56f58fddf30ffc07777"
)

var rateLimitProviderBenchmarkSink []rateLimitRoundResult

type rateLimitRoundOptions struct {
	workers      int
	attemptLimit time.Duration
	roundLimit   time.Duration
	joinLimit    time.Duration
}

type rateLimitRoundResult struct {
	worker int
	result ratelimit.Result
}

type rateLimitRoundMessage struct {
	rateLimitRoundResult
	err error
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
	joinLimit := opts.joinLimit
	if joinLimit <= 0 {
		joinLimit = opts.roundLimit
	}

	deadlineCtx, cancelDeadline := context.WithTimeout(ctx, opts.roundLimit)
	defer cancelDeadline()
	roundCtx, cancelRound := context.WithCancel(deadlineCtx)
	defer cancelRound()

	start := make(chan struct{})
	ready := make(chan struct{}, opts.workers)
	results := make(chan rateLimitRoundMessage, opts.workers)

	for workerID := range opts.workers {
		go func() {
			ready <- struct{}{}
			<-start

			attemptCtx, cancelAttempt := context.WithTimeout(roundCtx, opts.attemptLimit)
			result, err := worker(attemptCtx, workerID)
			cancelAttempt()
			results <- rateLimitRoundMessage{
				rateLimitRoundResult: rateLimitRoundResult{worker: workerID, result: result},
				err:                  err,
			}
		}()
	}

	for range opts.workers {
		<-ready
	}
	close(start)

	collected := make([]rateLimitRoundResult, 0, opts.workers)
	var firstErr error
	roundDone := roundCtx.Done()
	var joinTimer *time.Timer
	var joinDone <-chan time.Time
	defer func() {
		if joinTimer != nil {
			joinTimer.Stop()
		}
	}()

	for len(collected) < opts.workers {
		select {
		case message := <-results:
			collected = append(collected, message.rateLimitRoundResult)
			if message.err != nil && firstErr == nil {
				firstErr = fmt.Errorf("rate-limit round worker %d: %w", message.worker, message.err)
				cancelRound()
			}
		case <-roundDone:
			roundDone = nil
			if firstErr == nil {
				firstErr = roundCtx.Err()
			}
			joinTimer = time.NewTimer(joinLimit)
			joinDone = joinTimer.C
		case <-joinDone:
			return collected, errors.Join(
				firstErr,
				fmt.Errorf("rate-limit round join timed out after %s: completed %d/%d: %w", joinLimit, len(collected), opts.workers, context.DeadlineExceeded),
			)
		}
	}
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
	newLimiter      func(string) (ratelimit.Limiter, error)
	resetNamespace  func(context.Context, string) error
}

type rateLimitProviderNamespaces struct {
	mu     sync.Mutex
	values map[string]struct{}
}

func newRateLimitProviderNamespaces() *rateLimitProviderNamespaces {
	return &rateLimitProviderNamespaces{values: make(map[string]struct{})}
}

func (namespaces *rateLimitProviderNamespaces) add(namespace string) {
	namespaces.mu.Lock()
	defer namespaces.mu.Unlock()
	namespaces.values[namespace] = struct{}{}
}

func (namespaces *rateLimitProviderNamespaces) remove(namespace string) {
	namespaces.mu.Lock()
	defer namespaces.mu.Unlock()
	delete(namespaces.values, namespace)
}

func (namespaces *rateLimitProviderNamespaces) snapshot() []string {
	namespaces.mu.Lock()
	defer namespaces.mu.Unlock()
	values := make([]string, 0, len(namespaces.values))
	for namespace := range namespaces.values {
		values = append(values, namespace)
	}
	sort.Strings(values)
	return values
}

func cleanupRateLimitProviderNamespaces(
	ctx context.Context,
	namespaces *rateLimitProviderNamespaces,
	reset func(context.Context, string) error,
) error {
	var joined error
	for _, namespace := range namespaces.snapshot() {
		joined = errors.Join(joined, reset(ctx, namespace))
	}
	return joined
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
			if err := runRateLimitProviderPreflight(fixture); err != nil {
				b.Fatalf("preflight %s rate-limit provider: %v", fixture.name, err)
			}
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
		namespace, limiter, newErr := prepareRateLimitProviderIteration(fixture)
		if newErr != nil {
			b.Fatalf("prepare %s limiter iteration: %v", fixture.name, newErr)
		}
		if scenario == "AllowRejected" && newErr == nil {
			newErr = seedRejectedRateLimit(limiter, "benchmark-key")
		}

		var results []rateLimitRoundResult
		var runErr error
		if newErr == nil {
			b.StartTimer()
			results, runErr = executeRateLimitScenario(limiter, scenario)
			b.StopTimer()
		}

		cleanupCtx, cleanupCancel := rateLimitProviderOperationContext()
		cleanupErr := fixture.resetNamespace(cleanupCtx, namespace)
		cleanupCancel()

		rateLimitProviderBenchmarkSink = results
		verifyErr := error(nil)
		if newErr == nil {
			verifyErr = verifyRateLimitScenario(scenario, results)
		}
		if err := errors.Join(newErr, runErr, verifyErr, cleanupErr); err != nil {
			b.Fatalf("%s/%s: %v", fixture.name, scenario, err)
		}
	}
}

func prepareRateLimitProviderIteration(fixture rateLimitProviderFixture) (string, ratelimit.Limiter, error) {
	namespace, err := newRateLimitProviderNamespace()
	if err != nil {
		return "", nil, err
	}
	resetCtx, resetCancel := rateLimitProviderOperationContext()
	resetErr := fixture.resetNamespace(resetCtx, namespace)
	resetCancel()
	if resetErr != nil {
		return namespace, nil, resetErr
	}
	limiter, err := fixture.newLimiter(namespace)
	return namespace, limiter, err
}

func runRateLimitProviderPreflight(fixture rateLimitProviderFixture) error {
	namespace, err := newRateLimitProviderNamespace()
	if err != nil {
		return err
	}
	limiter, newErr := fixture.newLimiter(namespace)
	var results []rateLimitRoundResult
	var runErr error
	if newErr == nil {
		results, runErr = runSingleRateLimitAllow(limiter, "preflight-key")
	}
	verifyErr := error(nil)
	if newErr == nil && runErr == nil {
		verifyErr = verifyRateLimitScenario("AllowAvailable", results)
	}
	cleanupCtx, cleanupCancel := rateLimitProviderOperationContext()
	cleanupErr := fixture.resetNamespace(cleanupCtx, namespace)
	cleanupCancel()
	return errors.Join(newErr, runErr, verifyErr, cleanupErr)
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
	client := redis.NewClient(&redis.Options{Addr: address})
	ownedNamespaces := newRateLimitProviderNamespaces()
	resetNamespace := func(ctx context.Context, namespace string) error {
		err := deleteRedisRateLimitNamespace(ctx, client, namespace)
		if err == nil {
			ownedNamespaces.remove(namespace)
		}
		return err
	}
	registerRateLimitProviderCleanup(tb, "Redis", rateLimitProviderCleanup{
		deleteNamespace: func(ctx context.Context) error {
			return cleanupRateLimitProviderNamespaces(ctx, ownedNamespaces, resetNamespace)
		},
		closeClient: func(context.Context) error {
			return client.Close()
		},
	})
	if err := client.Ping(ctx).Err(); err != nil {
		tb.Fatalf("ping Redis rate-limit provider: %v", err)
	}
	serverInfo, err := client.Info(ctx, "server").Result()
	if err != nil {
		tb.Fatalf("query Redis server version: %v", err)
	}
	providerVersion, err := parseRedisRateLimitProviderVersion(serverInfo)
	if err != nil {
		tb.Fatal(err)
	}
	if !rateLimitProviderVersionMatchesAuthority(providerVersion, rateLimitRedisVersionAuthority) {
		tb.Fatalf("Redis provider version %q does not match pinned image authority %q", sanitizeRateLimitProviderMetadata(providerVersion), rateLimitRedisVersionAuthority)
	}
	return rateLimitProviderFixture{
		name:            "Redis",
		providerVersion: providerVersion,
		imageReference:  rateLimitRedisImageReference,
		newLimiter: func(namespace string) (ratelimit.Limiter, error) {
			ownedNamespaces.add(namespace)
			return redisratelimit.New(redisratelimit.Options{
				Client:        client,
				Namespace:     namespace,
				RatePerSecond: rateLimitProviderRefill,
				Burst:         rateLimitProviderCapacity,
				IdleTTL:       rateLimitProviderIdleTTL,
			})
		},
		resetNamespace: resetNamespace,
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
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		tb.Fatalf("open PostgreSQL rate-limit provider: %v", err)
	}
	ownedNamespaces := newRateLimitProviderNamespaces()
	resetNamespace := func(ctx context.Context, namespace string) error {
		_, err := db.ExecContext(ctx, `delete from public.bluetape_ratelimit_buckets where namespace=$1`, []byte(namespace))
		if err == nil {
			ownedNamespaces.remove(namespace)
		}
		return err
	}
	registerRateLimitProviderCleanup(tb, "PostgreSQL", rateLimitProviderCleanup{
		deleteNamespace: func(ctx context.Context) error {
			return cleanupRateLimitProviderNamespaces(ctx, ownedNamespaces, resetNamespace)
		},
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
	var providerVersion string
	if err := db.QueryRowContext(ctx, `show server_version`).Scan(&providerVersion); err != nil {
		tb.Fatalf("query PostgreSQL server version: %v", err)
	}
	providerVersion = strings.TrimSpace(providerVersion)
	if !rateLimitProviderVersionMatchesAuthority(providerVersion, rateLimitPostgresVersionAuthority) {
		tb.Fatalf("PostgreSQL provider version %q does not match pinned image authority %q", sanitizeRateLimitProviderMetadata(providerVersion), rateLimitPostgresVersionAuthority)
	}
	return rateLimitProviderFixture{
		name:            "PostgreSQL",
		providerVersion: providerVersion,
		imageReference:  rateLimitPostgresImageReference,
		newLimiter: func(namespace string) (ratelimit.Limiter, error) {
			ownedNamespaces.add(namespace)
			return sqlratelimit.New(db, sqlratelimit.Options{
				Namespace:     namespace,
				RatePerSecond: rateLimitProviderRefill,
				Burst:         rateLimitProviderCapacity,
				IdleTTL:       rateLimitProviderIdleTTL,
			})
		},
		resetNamespace: resetNamespace,
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

func newRateLimitProviderNamespace() (string, error) {
	var value [16]byte
	if _, err := rand.Read(value[:]); err != nil {
		return "", fmt.Errorf("generate rate-limit provider namespace: %w", err)
	}
	return hex.EncodeToString(value[:]), nil
}

func parseRedisRateLimitProviderVersion(serverInfo string) (string, error) {
	for line := range strings.SplitSeq(serverInfo, "\n") {
		line = strings.TrimSpace(line)
		if value, ok := strings.CutPrefix(line, "redis_version:"); ok {
			version := strings.TrimSpace(value)
			if version == "" {
				break
			}
			return version, nil
		}
	}
	return "", errors.New("Redis server info has no reported version")
}

func rateLimitProviderVersionMatchesAuthority(reported, authority string) bool {
	reported = strings.TrimSpace(reported)
	authority = strings.TrimSpace(authority)
	if reported == "" || authority == "" {
		return false
	}
	return reported == authority || strings.HasPrefix(reported, authority+".")
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

func TestRunRateLimitRoundReturnsWhenWorkerIgnoresCancellation(t *testing.T) {
	const workers = 8
	rogueRelease := make(chan struct{})
	rogueExited := make(chan struct{})

	type outcome struct {
		results []rateLimitRoundResult
		err     error
	}
	done := make(chan outcome, 1)
	go func() {
		results, err := runRateLimitRound(context.Background(), rateLimitRoundOptions{
			workers:      workers,
			attemptLimit: 20 * time.Millisecond,
			roundLimit:   20 * time.Millisecond,
			joinLimit:    20 * time.Millisecond,
		}, func(ctx context.Context, worker int) (ratelimit.Result, error) {
			if worker == 0 {
				defer close(rogueExited)
				<-rogueRelease
				return ratelimit.Result{}, nil
			}
			<-ctx.Done()
			return ratelimit.Result{}, ctx.Err()
		})
		done <- outcome{results: results, err: err}
	}()

	select {
	case got := <-done:
		if !errors.Is(got.err, context.DeadlineExceeded) || !strings.Contains(got.err.Error(), "join timed out") {
			close(rogueRelease)
			<-rogueExited
			t.Fatalf("run rate-limit round error = %v, want explicit join timeout", got.err)
		}
		if len(got.results) != workers-1 {
			close(rogueRelease)
			<-rogueExited
			t.Fatalf("drained results = %d, want %d", len(got.results), workers-1)
		}
		close(rogueRelease)
		select {
		case <-rogueExited:
		case <-time.After(time.Second):
			t.Fatal("rogue worker did not exit after release")
		}
	case <-time.After(500 * time.Millisecond):
		close(rogueRelease)
		<-done
		t.Fatal("rate-limit round blocked on a worker that ignored cancellation")
	}
}

func TestRateLimitProviderNamespacesAreUniqueLowercaseHex(t *testing.T) {
	seen := make(map[string]struct{})
	for range 64 {
		namespace, err := newRateLimitProviderNamespace()
		if err != nil {
			t.Fatal(err)
		}
		if len(namespace) != 32 || strings.Trim(namespace, "0123456789abcdef") != "" {
			t.Fatalf("namespace = %q, want 32 lowercase hex characters", namespace)
		}
		if _, ok := seen[namespace]; ok {
			t.Fatalf("duplicate namespace %q", namespace)
		}
		seen[namespace] = struct{}{}
	}
}

func TestPrepareRateLimitProviderIterationResetsFreshNamespaceBeforeLimiter(t *testing.T) {
	var events []string
	fixture := rateLimitProviderFixture{
		newLimiter: func(namespace string) (ratelimit.Limiter, error) {
			events = append(events, "new:"+namespace)
			return preflightRateLimiter{}, nil
		},
		resetNamespace: func(_ context.Context, namespace string) error {
			events = append(events, "reset:"+namespace)
			return nil
		},
	}

	first, _, err := prepareRateLimitProviderIteration(fixture)
	if err != nil {
		t.Fatal(err)
	}
	second, _, err := prepareRateLimitProviderIteration(fixture)
	if err != nil {
		t.Fatal(err)
	}
	if first == second {
		t.Fatalf("iteration namespaces are equal: %q", first)
	}
	for _, namespace := range []string{first, second} {
		if len(namespace) != 32 || strings.Trim(namespace, "0123456789abcdef") != "" {
			t.Fatalf("iteration namespace = %q, want 32 lowercase hex characters", namespace)
		}
	}
	want := []string{"reset:" + first, "new:" + first, "reset:" + second, "new:" + second}
	if !reflect.DeepEqual(events, want) {
		t.Fatalf("iteration events = %v, want %v", events, want)
	}
}

func TestParseRedisRateLimitProviderVersion(t *testing.T) {
	version, err := parseRedisRateLimitProviderVersion("# Server\r\nredis_version:7.4.2\r\nredis_git_sha1:00000000\r\n")
	if err != nil {
		t.Fatal(err)
	}
	if version != "7.4.2" {
		t.Fatalf("Redis version = %q, want 7.4.2", version)
	}
	if _, err := parseRedisRateLimitProviderVersion("# Server\r\nredis_mode:standalone\r\n"); err == nil {
		t.Fatal("missing Redis version error = nil")
	}
}

func TestRateLimitProviderVersionMatchesAuthority(t *testing.T) {
	tests := []struct {
		name      string
		reported  string
		authority string
		want      bool
	}{
		{name: "Redis patch", reported: "7.4.2", authority: "7.4", want: true},
		{name: "PostgreSQL patch", reported: "16.4 (Debian 16.4-1)", authority: "16", want: true},
		{name: "wrong minor", reported: "7.5.0", authority: "7.4", want: false},
		{name: "wrong major prefix", reported: "160.1", authority: "16", want: false},
		{name: "blank", reported: "", authority: "16", want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := rateLimitProviderVersionMatchesAuthority(tt.reported, tt.authority); got != tt.want {
				t.Fatalf("version match = %v, want %v", got, tt.want)
			}
		})
	}
}

type preflightRateLimiter struct {
	events *[]string
}

func (limiter preflightRateLimiter) Allow(context.Context, string, int64) (ratelimit.Result, error) {
	*limiter.events = append(*limiter.events, "allow")
	return ratelimit.Result{Allowed: true, Requested: 1, Remaining: rateLimitProviderCapacity - 1}, nil
}

func TestRateLimitProviderPreflightAllowsThenResetsFreshNamespace(t *testing.T) {
	var events []string
	var namespace string
	fixture := rateLimitProviderFixture{
		newLimiter: func(got string) (ratelimit.Limiter, error) {
			namespace = got
			events = append(events, "new")
			return preflightRateLimiter{events: &events}, nil
		},
		resetNamespace: func(context.Context, string) error {
			events = append(events, "reset")
			return nil
		},
	}

	if err := runRateLimitProviderPreflight(fixture); err != nil {
		t.Fatal(err)
	}
	if len(namespace) != 32 || strings.Trim(namespace, "0123456789abcdef") != "" {
		t.Fatalf("preflight namespace = %q, want 32 lowercase hex characters", namespace)
	}
	if !reflect.DeepEqual(events, []string{"new", "allow", "reset"}) {
		t.Fatalf("preflight events = %v, want [new allow reset]", events)
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
