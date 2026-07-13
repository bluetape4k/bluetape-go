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
