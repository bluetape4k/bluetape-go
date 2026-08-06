package sqlratelimit

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	postgrestestcontainer "github.com/bluetape4k/bluetape-go/testcontainers/postgres"
	_ "github.com/jackc/pgx/v5/stdlib"
)

func TestMultiPoolExactAdmission(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	t.Cleanup(cancel)
	dsn := postgrestestcontainer.Start(ctx, t)
	pools := make([]*sql.DB, 4)
	for i := range pools {
		db, err := sql.Open("pgx", dsn)
		if err != nil {
			t.Fatal(err)
		}
		pools[i] = db
		t.Cleanup(func() { _ = db.Close() })
	}
	if _, err := pools[0].ExecContext(ctx, SchemaSQL); err != nil {
		t.Fatal(err)
	}
	const burst int64 = 10
	for iteration := range 20 {
		namespace := fmt.Sprintf("multi-pool-%d", iteration)
		limiters := make([]*Limiter, len(pools))
		for i, db := range pools {
			limiter, err := New(db, Options{Namespace: namespace, RatePerSecond: 0.001, Burst: burst})
			if err != nil {
				t.Fatal(err)
			}
			limiters[i] = limiter
		}
		start := make(chan struct{})
		var allowed atomic.Int64
		errs := make(chan error, int(burst)+32)
		var workers sync.WaitGroup
		for i := range int(burst) + 32 {
			workers.Add(1)
			go func() {
				defer workers.Done()
				<-start
				result, err := limiters[i%len(limiters)].Allow(ctx, "hot", 1)
				if err == nil && result.Allowed {
					allowed.Add(result.Requested)
				}
				errs <- err
			}()
		}
		close(start)
		workers.Wait()
		close(errs)
		for err := range errs {
			if err != nil {
				t.Fatalf("iteration %d: %v", iteration, err)
			}
		}
		if got := allowed.Load(); got != burst {
			t.Fatalf("iteration %d allowed=%d want=%d", iteration, got, burst)
		}
	}
}

func TestFractionalRefillCarry(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	t.Cleanup(cancel)
	_, db := openRateLimitPostgres(ctx, t)
	limiter := newPostgresLimiter(t, db, "fractional", Options{RatePerSecond: 0.000001, Burst: 1})
	if _, err := limiter.Allow(ctx, "key", 1); err != nil {
		t.Fatal(err)
	}
	for range 5 {
		time.Sleep(20 * time.Millisecond)
		if result, err := limiter.Allow(ctx, "key", 1); err != nil || result.Allowed {
			t.Fatalf("fractional rejection = %+v, %v", result, err)
		}
	}
	var tokens float64
	if err := db.QueryRowContext(ctx, `select tokens_micros::double precision from public.bluetape_ratelimit_buckets where namespace=$1 and bucket_key=$2`, limiter.opts.namespace, []byte("key")).Scan(&tokens); err != nil {
		t.Fatal(err)
	}
	if tokens <= 0 || tokens >= float64(tokenScale) {
		t.Fatalf("fractional carry = %f microtokens, want between 0 and one token", tokens)
	}
}

func TestCleanupAllowPoolContention(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	t.Cleanup(cancel)
	_, db := openRateLimitPostgres(ctx, t)
	db.SetMaxOpenConns(2)
	db.SetMaxIdleConns(2)
	limiter := newPostgresLimiter(t, db, "pool-contention", Options{RatePerSecond: 0.001, Burst: 5})
	for i := range 20 {
		seedBucketForCleanup(ctx, t, db, limiter, fmt.Sprintf("expired-%02d", i), -time.Minute)
	}

	start := make(chan struct{})
	var allowed atomic.Int64
	var cleaned atomic.Int64
	errCh := make(chan error, 16)
	var workers sync.WaitGroup
	for i := range 12 {
		workers.Add(1)
		go func() {
			defer workers.Done()
			<-start
			result, err := limiter.Allow(ctx, "hot", 1)
			if err == nil && result.Allowed {
				allowed.Add(1)
			}
			errCh <- err
		}()
		_ = i
	}
	for range 4 {
		workers.Add(1)
		go func() {
			defer workers.Done()
			<-start
			count, err := limiter.Cleanup(ctx, 5)
			cleaned.Add(count)
			errCh <- err
		}()
	}
	close(start)
	done := make(chan struct{})
	go func() { workers.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("pool contention workers did not finish")
	}
	close(errCh)
	for err := range errCh {
		if err != nil {
			t.Fatal(err)
		}
	}
	if allowed.Load() != 5 {
		t.Fatalf("allowed=%d want=5", allowed.Load())
	}
	if cleaned.Load() == 0 {
		t.Fatal("cleanup made no progress")
	}
	stats := db.Stats()
	t.Logf("pool diagnostics wait_count=%d wait_duration=%s in_use=%d", stats.WaitCount, stats.WaitDuration, stats.InUse)
}
