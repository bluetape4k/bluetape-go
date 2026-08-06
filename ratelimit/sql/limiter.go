package sqlratelimit

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/bluetape4k/bluetape-go/ratelimit"
)

var _ ratelimit.Limiter = (*Limiter)(nil)

// Limiter is a PostgreSQL-backed token-bucket limiter.
type Limiter struct {
	db       *sql.DB
	opts     options
	testHook func(context.Context, string, testPhase, string) error
}

type testPhase string

const (
	phaseBeforeLinearize testPhase = "before-linearize"
	phaseAfterLinearize  testPhase = "after-linearize"
)

// New creates a limiter with a caller-owned database pool.
// New performs no database I/O and never closes db.
func New(db *sql.DB, opts Options) (*Limiter, error) {
	if db == nil {
		return nil, errors.New("postgres rate limiter database must not be nil")
	}
	normalized, err := opts.normalize()
	if err != nil {
		return nil, err
	}
	return &Limiter{db: db, opts: normalized}, nil
}

// Allow attempts to consume tokens from key's PostgreSQL bucket.
func (l *Limiter) Allow(ctx context.Context, key string, tokens int64) (ratelimit.Result, error) {
	ctx = normalizeContext(ctx)
	if err := ctx.Err(); err != nil {
		return ratelimit.Result{}, err
	}
	if l == nil || l.db == nil {
		return ratelimit.Result{}, errors.New("postgres rate limiter is not initialized")
	}
	normalizedKey, err := l.opts.normalizeKey(key)
	if err != nil {
		return ratelimit.Result{}, err
	}
	if tokens > l.opts.burst {
		return ratelimit.Result{}, errors.New("tokens must not exceed burst")
	}
	requestedMicros, err := tokensToMicros(tokens)
	if err != nil {
		return ratelimit.Result{}, err
	}
	if l.testHook != nil {
		if err := l.testHook(ctx, "allow", phaseBeforeLinearize, normalizedKey); err != nil {
			return ratelimit.Result{}, err
		}
	}
	conn, err := l.db.Conn(ctx)
	if err != nil {
		if contextErr := ctx.Err(); contextErr != nil {
			return ratelimit.Result{}, contextErr
		}
		return ratelimit.Result{}, newOperationError(
			"allow", string(l.opts.namespace), normalizedKey, err,
		)
	}
	defer func() { _ = conn.Close() }()
	var allowed bool
	var remaining, retryMicros, resetMicros int64
	err = conn.QueryRowContext(
		ctx,
		allowQuery,
		l.opts.namespace,
		[]byte(normalizedKey),
		requestedMicros,
		l.opts.burstMicros,
		l.opts.rateMicrosPerSecond,
		l.opts.idleTTLMicros,
	).Scan(&allowed, &remaining, &retryMicros, &resetMicros)
	if errors.Is(err, sql.ErrNoRows) {
		return ratelimit.Result{}, ErrConfigurationMismatch
	}
	if err != nil {
		return ratelimit.Result{}, classifyOperationError(
			"allow", string(l.opts.namespace), normalizedKey, err, ctx.Err(),
		)
	}
	result := ratelimit.Result{
		Allowed:    allowed,
		Requested:  tokens,
		Remaining:  remaining,
		RetryAfter: microsDuration(retryMicros),
		ResetAfter: microsDuration(resetMicros),
	}
	if l.testHook != nil {
		if err := l.testHook(ctx, "allow", phaseAfterLinearize, normalizedKey); err != nil {
			return ratelimit.Result{}, classifyOperationError(
				"allow", string(l.opts.namespace), normalizedKey, err, ctx.Err(),
			)
		}
	}
	return result, nil
}

// Cleanup deletes at most limit buckets whose idle expiry has passed.
// Callers own scheduling, timeouts, and retries. On error the returned count is zero,
// although an indeterminate operation may already have deleted up to limit rows.
func (l *Limiter) Cleanup(ctx context.Context, limit int) (int64, error) {
	ctx = normalizeContext(ctx)
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	if l == nil || l.db == nil {
		return 0, errors.New("postgres rate limiter is not initialized")
	}
	if limit < 1 || limit > MaxCleanupBatch {
		return 0, fmt.Errorf("cleanup limit must be between 1 and %d", MaxCleanupBatch)
	}
	if l.testHook != nil {
		if err := l.testHook(ctx, "cleanup", phaseBeforeLinearize, ""); err != nil {
			return 0, err
		}
	}
	conn, err := l.db.Conn(ctx)
	if err != nil {
		if contextErr := ctx.Err(); contextErr != nil {
			return 0, contextErr
		}
		return 0, newCleanupOperationError(err)
	}
	defer func() { _ = conn.Close() }()
	var count int64
	if err := conn.QueryRowContext(ctx, cleanupQuery, limit).Scan(&count); err != nil {
		return 0, classifyCleanupError(err, ctx.Err())
	}
	if l.testHook != nil {
		if err := l.testHook(ctx, "cleanup", phaseAfterLinearize, ""); err != nil {
			return 0, classifyCleanupError(err, ctx.Err())
		}
	}
	return count, nil
}

func normalizeContext(ctx context.Context) context.Context {
	if ctx == nil {
		return context.Background()
	}
	return ctx
}

func microsDuration(value int64) time.Duration {
	if value <= 0 {
		return 0
	}
	if value > math.MaxInt64/int64(time.Microsecond) {
		return time.Duration(math.MaxInt64)
	}
	return time.Duration(value) * time.Microsecond
}
