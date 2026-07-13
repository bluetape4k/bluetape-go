package sqlratelimit

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/bluetape4k/bluetape-go/ratelimit"
	postgrestestcontainer "github.com/bluetape4k/bluetape-go/testcontainers/postgres"
	_ "github.com/jackc/pgx/v5/stdlib"
)

func TestSchemaSQLHasFixedContract(t *testing.T) {
	normalized := strings.ToLower(SchemaSQL)
	for _, required := range []string{
		"public.bluetape_ratelimit_buckets",
		"namespace bytea not null",
		"bucket_key bytea not null",
		"tokens_micros numeric(30, 6) not null",
		"primary key (namespace, bucket_key)",
		"bluetape_ratelimit_buckets_expires_at_idx",
		"(expires_at)",
	} {
		if !strings.Contains(normalized, required) {
			t.Fatalf("SchemaSQL missing %q", required)
		}
	}
	if MaxCleanupBatch != 1000 {
		t.Fatalf("MaxCleanupBatch = %d", MaxCleanupBatch)
	}
}

func TestAllowDoesNotRegressTimeAfterConflictLockWait(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	t.Cleanup(cancel)
	dsn, admin := openRateLimitPostgres(ctx, t)
	worker, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = worker.Close() })
	worker.SetMaxOpenConns(1)
	worker.SetMaxIdleConns(1)
	limiter := newPostgresLimiter(t, worker, "lock-wait", Options{RatePerSecond: 1, Burst: 2, IdleTTL: 10 * time.Second})
	if _, err := limiter.Allow(ctx, "key", 1); err != nil {
		t.Fatal(err)
	}
	var backendPID int
	if err := worker.QueryRowContext(ctx, `select pg_backend_pid()`).Scan(&backendPID); err != nil {
		t.Fatal(err)
	}

	tx, err := admin.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = tx.Rollback() })
	var future time.Time
	err = tx.QueryRowContext(ctx, `update public.bluetape_ratelimit_buckets
set tokens_micros=0, updated_at=pg_catalog.clock_timestamp()+interval '1 second',
    expires_at=pg_catalog.clock_timestamp()+interval '11 seconds'
where namespace=$1 and bucket_key=$2 returning updated_at`, limiter.opts.namespace, []byte("key")).Scan(&future)
	if err != nil {
		t.Fatal(err)
	}

	type response struct {
		result ratelimit.Result
		err    error
	}
	resultCh := make(chan response, 1)
	go func() {
		result, err := limiter.Allow(ctx, "key", 1)
		resultCh <- response{result: result, err: err}
	}()
	waitUntil(t, 5*time.Second, func() bool {
		var waitType string
		err := admin.QueryRowContext(ctx, `select coalesce(wait_event_type,'') from pg_catalog.pg_stat_activity where pid=$1`, backendPID).Scan(&waitType)
		return err == nil && waitType == "Lock"
	})
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	got := <-resultCh
	if got.err != nil || got.result.Allowed {
		t.Fatalf("blocked Allow = %+v, %v", got.result, got.err)
	}
	var updated time.Time
	if err := admin.QueryRowContext(ctx, `select updated_at from public.bluetape_ratelimit_buckets where namespace=$1 and bucket_key=$2`, limiter.opts.namespace, []byte("key")).Scan(&updated); err != nil {
		t.Fatal(err)
	}
	if updated.Before(future) {
		t.Fatalf("updated_at regressed: got %s, want >= %s", updated, future)
	}
}

func TestAllowFailureBoundaries(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	t.Cleanup(cancel)
	_, db := openRateLimitPostgres(ctx, t)

	t.Run("lost-response", func(t *testing.T) {
		limiter := newPostgresLimiter(t, db, "lost-response", Options{RatePerSecond: 1, Burst: 2, IdleTTL: 2 * time.Second})
		cause := errors.New("raw endpoint response marker")
		limiter.testHook = func(operation string, phase testPhase, key string) error {
			if operation == "allow" && phase == phaseAfterLinearize {
				return cause
			}
			return nil
		}
		result, err := limiter.Allow(ctx, "raw-key-marker", 1)
		if result != (ratelimit.Result{}) || !errors.Is(err, cause) || !errors.Is(err, ratelimit.ErrCommitUnknown) {
			t.Fatalf("lost response = %+v, %v", result, err)
		}
		var opErr ratelimit.OperationError
		if !errors.As(err, &opErr) || opErr.Operation() != "allow" {
			t.Fatalf("lost response operation error = %T, %v", err, err)
		}
		limiter.testHook = nil
		next, err := limiter.Allow(ctx, "raw-key-marker", 2)
		if err != nil || next.Allowed {
			t.Fatalf("lost response debit missing = %+v, %v", next, err)
		}
	})

	t.Run("cancel-after-scan-returns-confirmed-result", func(t *testing.T) {
		limiter := newPostgresLimiter(t, db, "after-scan", Options{RatePerSecond: 1, Burst: 1, IdleTTL: time.Second})
		started := make(chan struct{})
		resume := make(chan struct{})
		limiter.testHook = func(operation string, phase testPhase, key string) error {
			if operation == "allow" && phase == phaseAfterLinearize {
				close(started)
				<-resume
			}
			return nil
		}
		callCtx, callCancel := context.WithCancel(ctx)
		resultCh := make(chan struct {
			result ratelimit.Result
			err    error
		}, 1)
		go func() {
			result, err := limiter.Allow(callCtx, "key", 1)
			resultCh <- struct {
				result ratelimit.Result
				err    error
			}{result, err}
		}()
		<-started
		callCancel()
		close(resume)
		got := <-resultCh
		if got.err != nil || !got.result.Allowed {
			t.Fatalf("confirmed result = %+v, %v", got.result, got.err)
		}
	})

	t.Run("server-error-is-known-rollback", func(t *testing.T) {
		limiter := newPostgresLimiter(t, db, "known-rollback", Options{RatePerSecond: 1, Burst: 1, IdleTTL: time.Second})
		if _, err := db.ExecContext(ctx, `drop table public.bluetape_ratelimit_buckets`); err != nil {
			t.Fatal(err)
		}
		result, err := limiter.Allow(ctx, "key", 1)
		if result != (ratelimit.Result{}) || err == nil || errors.Is(err, ratelimit.ErrCommitUnknown) {
			t.Fatalf("known rollback = %+v, %v", result, err)
		}
		var opErr ratelimit.OperationError
		if !errors.As(err, &opErr) {
			t.Fatalf("known rollback type = %T", err)
		}
	})
}

func TestAllowInFlightCancellation(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	t.Cleanup(cancel)
	dsn, admin := openRateLimitPostgres(ctx, t)
	worker, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = worker.Close() })
	worker.SetMaxOpenConns(1)
	worker.SetMaxIdleConns(1)
	limiter := newPostgresLimiter(t, worker, "in-flight-cancel", Options{RatePerSecond: 1, Burst: 2, IdleTTL: 2 * time.Second})
	if _, err := limiter.Allow(ctx, "key", 1); err != nil {
		t.Fatal(err)
	}
	before := bucketSnapshotForTest(ctx, t, admin, limiter.opts.namespace, "key")
	var backendPID int
	if err := worker.QueryRowContext(ctx, `select pg_backend_pid()`).Scan(&backendPID); err != nil {
		t.Fatal(err)
	}
	tx, err := admin.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = tx.Rollback() })
	if _, err := tx.ExecContext(ctx, `select 1 from public.bluetape_ratelimit_buckets where namespace=$1 and bucket_key=$2 for update`, limiter.opts.namespace, []byte("key")); err != nil {
		t.Fatal(err)
	}

	callCtx, callCancel := context.WithCancel(ctx)
	resultCh := make(chan struct {
		result ratelimit.Result
		err    error
	}, 1)
	go func() {
		result, err := limiter.Allow(callCtx, "key", 1)
		resultCh <- struct {
			result ratelimit.Result
			err    error
		}{result, err}
	}()
	waitUntil(t, 5*time.Second, func() bool {
		var waitType string
		err := admin.QueryRowContext(ctx, `select coalesce(wait_event_type,'') from pg_catalog.pg_stat_activity where pid=$1`, backendPID).Scan(&waitType)
		return err == nil && waitType == "Lock"
	})
	callCancel()
	select {
	case got := <-resultCh:
		if got.result != (ratelimit.Result{}) || !errors.Is(got.err, context.Canceled) || !errors.Is(got.err, ratelimit.ErrCommitUnknown) {
			t.Fatalf("in-flight cancellation = %+v, %v", got.result, got.err)
		}
		var opErr ratelimit.OperationError
		if !errors.As(got.err, &opErr) || opErr.Operation() != "allow" {
			t.Fatalf("in-flight cancellation type = %T, %v", got.err, got.err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("in-flight cancellation did not return")
	}
	if err := tx.Rollback(); err != nil {
		t.Fatal(err)
	}
	after := bucketSnapshotForTest(ctx, t, admin, limiter.opts.namespace, "key")
	if before.rate != after.rate || before.burst != after.burst || before.ttl != after.ttl {
		t.Fatalf("canceled Allow changed configuration: before=%+v after=%+v", before, after)
	}
	next, err := limiter.Allow(ctx, "key", 2)
	if err != nil || next.Allowed {
		t.Fatalf("in-flight cancellation admitted more than one debit: %+v, %v", next, err)
	}
}

func TestCleanupPostgres(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	t.Cleanup(cancel)
	dsn, db := openRateLimitPostgres(ctx, t)
	limiter := newPostgresLimiter(t, db, "cleanup", Options{RatePerSecond: 1, Burst: 2, IdleTTL: 2 * time.Second})

	t.Run("bounded-delete-and-live-safety", func(t *testing.T) {
		for _, key := range []string{"expired-1", "expired-2", "expired-3"} {
			seedBucketForCleanup(ctx, t, db, limiter, key, -time.Minute)
		}
		seedBucketForCleanup(ctx, t, db, limiter, "live", time.Minute)
		count, err := limiter.Cleanup(ctx, 2)
		if err != nil || count != 2 {
			t.Fatalf("Cleanup(2) = %d, %v", count, err)
		}
		if got := bucketCountForTest(ctx, t, db, limiter.opts.namespace); got != 2 {
			t.Fatalf("remaining bucket count = %d, want 2", got)
		}
		count, err = limiter.Cleanup(ctx, 2)
		if err != nil || count != 1 {
			t.Fatalf("second Cleanup(2) = %d, %v", count, err)
		}
		if got := bucketCountForTest(ctx, t, db, limiter.opts.namespace); got != 1 {
			t.Fatalf("live bucket was deleted, count=%d", got)
		}
	})

	t.Run("concurrent-workers-do-not-double-count", func(t *testing.T) {
		for i := range 20 {
			seedBucketForCleanup(ctx, t, db, limiter, fmt.Sprintf("concurrent-%02d", i), -time.Minute)
		}
		poolB, err := sql.Open("pgx", dsn)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = poolB.Close() })
		limiterB, err := New(poolB, Options{Namespace: string(limiter.opts.namespace), RatePerSecond: 1, Burst: 2, IdleTTL: 2 * time.Second})
		if err != nil {
			t.Fatal(err)
		}
		counts := make(chan int64, 2)
		errs := make(chan error, 2)
		for _, current := range []*Limiter{limiter, limiterB} {
			go func() {
				count, err := current.Cleanup(ctx, 20)
				counts <- count
				errs <- err
			}()
		}
		var total int64
		for range 2 {
			total += <-counts
			if err := <-errs; err != nil {
				t.Fatal(err)
			}
		}
		if total != 20 {
			t.Fatalf("concurrent cleanup total = %d, want 20", total)
		}
	})

	t.Run("lost-response-returns-zero-count", func(t *testing.T) {
		seedBucketForCleanup(ctx, t, db, limiter, "lost", -time.Minute)
		cause := errors.New("cleanup response lost")
		limiter.testHook = func(operation string, phase testPhase, key string) error {
			if operation == "cleanup" && phase == phaseAfterLinearize {
				return cause
			}
			return nil
		}
		count, err := limiter.Cleanup(ctx, 1)
		if count != 0 || !errors.Is(err, cause) || !errors.Is(err, ratelimit.ErrCommitUnknown) {
			t.Fatalf("lost Cleanup = %d, %v", count, err)
		}
		var opErr ratelimit.OperationError
		if !errors.As(err, &opErr) || opErr.Operation() != "cleanup" {
			t.Fatalf("cleanup operation error = %T, %v", err, err)
		}
		limiter.testHook = nil
		if count, err := limiter.Cleanup(ctx, 1); err != nil || count < 0 || count > 1 {
			t.Fatalf("cleanup retry = %d, %v", count, err)
		}
	})
}

func seedBucketForCleanup(ctx context.Context, t *testing.T, db *sql.DB, limiter *Limiter, key string, expiryOffset time.Duration) {
	t.Helper()
	_, err := db.ExecContext(ctx, `insert into public.bluetape_ratelimit_buckets (
namespace,bucket_key,rate_micros_per_second,burst_micros,idle_ttl_micros,tokens_micros,
last_allowed,updated_at,expires_at) values ($1,$2,$3::bigint,$4::bigint,$5::bigint,$4::numeric,false,
pg_catalog.clock_timestamp()-interval '2 minutes',pg_catalog.clock_timestamp()+$6::bigint*interval '1 microsecond')
on conflict (namespace,bucket_key) do update set expires_at=excluded.expires_at`,
		limiter.opts.namespace, []byte(key), limiter.opts.rateMicrosPerSecond, limiter.opts.burstMicros,
		limiter.opts.idleTTLMicros, expiryOffset.Microseconds())
	if err != nil {
		t.Fatal(err)
	}
}

func bucketCountForTest(ctx context.Context, t *testing.T, db *sql.DB, namespace []byte) int64 {
	t.Helper()
	var count int64
	if err := db.QueryRowContext(ctx, `select count(*) from public.bluetape_ratelimit_buckets where namespace=$1`, namespace).Scan(&count); err != nil {
		t.Fatal(err)
	}
	return count
}

func waitUntil(t *testing.T, timeout time.Duration, condition func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal(fmt.Errorf("condition not met within %s", timeout))
}

func TestAllowPostgres(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	t.Cleanup(cancel)
	dsn, db := openRateLimitPostgres(ctx, t)

	t.Run("burst-rejection-and-refill", func(t *testing.T) {
		limiter := newPostgresLimiter(t, db, "allow", Options{RatePerSecond: 100, Burst: 2, IdleTTL: time.Second})
		first, err := limiter.Allow(ctx, "user", 1)
		if err != nil || !first.Allowed || first.Requested != 1 || first.Remaining != 1 {
			t.Fatalf("first Allow = %+v, %v", first, err)
		}
		second, err := limiter.Allow(ctx, "user", 1)
		if err != nil || !second.Allowed || second.Remaining != 0 {
			t.Fatalf("second Allow = %+v, %v", second, err)
		}
		rejected, err := limiter.Allow(ctx, "user", 1)
		if err != nil || rejected.Allowed || rejected.RetryAfter <= 0 || rejected.ResetAfter <= 0 {
			t.Fatalf("rejected Allow = %+v, %v", rejected, err)
		}
		time.Sleep(20 * time.Millisecond)
		refilled, err := limiter.Allow(ctx, "user", 1)
		if err != nil || !refilled.Allowed {
			t.Fatalf("refilled Allow = %+v, %v", refilled, err)
		}
	})

	t.Run("key-namespace-and-pool-isolation", func(t *testing.T) {
		poolB, err := sql.Open("pgx", dsn)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = poolB.Close() })
		sharedA := newPostgresLimiter(t, db, "shared", Options{RatePerSecond: 1, Burst: 1, IdleTTL: time.Second})
		sharedB := newPostgresLimiter(t, poolB, "shared", Options{RatePerSecond: 1, Burst: 1, IdleTTL: time.Second})
		if result, err := sharedA.Allow(ctx, "a\x00b", 1); err != nil || !result.Allowed {
			t.Fatalf("shared first = %+v, %v", result, err)
		}
		if result, err := sharedB.Allow(ctx, "a\x00b", 1); err != nil || result.Allowed {
			t.Fatalf("shared second = %+v, %v", result, err)
		}
		if result, err := sharedA.Allow(ctx, string([]byte{0xff, 0xfe}), 1); err != nil || !result.Allowed {
			t.Fatalf("invalid utf8 key = %+v, %v", result, err)
		}
		isolated := newPostgresLimiter(t, db, "other", Options{RatePerSecond: 1, Burst: 1, IdleTTL: time.Second})
		if result, err := isolated.Allow(ctx, "a\x00b", 1); err != nil || !result.Allowed {
			t.Fatalf("isolated namespace = %+v, %v", result, err)
		}
	})

	t.Run("configuration-mismatch-is-zero-result", func(t *testing.T) {
		original := newPostgresLimiter(t, db, "mismatch", Options{RatePerSecond: 1, Burst: 2, IdleTTL: 2 * time.Second})
		if _, err := original.Allow(ctx, "key", 1); err != nil {
			t.Fatal(err)
		}
		before := bucketSnapshotForTest(ctx, t, db, original.opts.namespace, "key")
		changed := newPostgresLimiter(t, db, "mismatch", Options{RatePerSecond: 2, Burst: 2, IdleTTL: 2 * time.Second})
		result, err := changed.Allow(ctx, "key", 1)
		if result != (ratelimit.Result{}) || !errors.Is(err, ErrConfigurationMismatch) {
			t.Fatalf("mismatch Allow = %+v, %v", result, err)
		}
		after := bucketSnapshotForTest(ctx, t, db, original.opts.namespace, "key")
		if before != after {
			t.Fatalf("mismatch changed row: before=%+v after=%+v", before, after)
		}
	})
}

type bucketSnapshot struct {
	rate, burst, ttl int64
	tokens, updated  string
	xmin             string
}

func bucketSnapshotForTest(ctx context.Context, t *testing.T, db *sql.DB, namespace []byte, key string) bucketSnapshot {
	t.Helper()
	var got bucketSnapshot
	err := db.QueryRowContext(ctx, `select rate_micros_per_second,burst_micros,idle_ttl_micros,
tokens_micros::text,updated_at::text,xmin::text from public.bluetape_ratelimit_buckets
where namespace=$1 and bucket_key=$2`, namespace, []byte(key)).Scan(
		&got.rate, &got.burst, &got.ttl, &got.tokens, &got.updated, &got.xmin)
	if err != nil {
		t.Fatal(err)
	}
	return got
}

func openRateLimitPostgres(ctx context.Context, t *testing.T) (string, *sql.DB) {
	t.Helper()
	dsn := postgrestestcontainer.Start(ctx, t)
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := db.PingContext(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, SchemaSQL); err != nil {
		t.Fatal(err)
	}
	return dsn, db
}

func newPostgresLimiter(t *testing.T, db *sql.DB, namespace string, opts Options) *Limiter {
	t.Helper()
	opts.Namespace = namespace + "-" + strings.NewReplacer("/", "-", " ", "-").Replace(t.Name())
	limiter, err := New(db, opts)
	if err != nil {
		t.Fatal(err)
	}
	return limiter
}
