package sqlratelimit_test

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/bluetape4k/bluetape-go/ratelimit"
	sqlratelimit "github.com/bluetape4k/bluetape-go/ratelimit/sql"
	_ "github.com/jackc/pgx/v5/stdlib"
)

func ExampleNew() {
	ctx := context.Background()

	// Migration credentials own schema changes and are not used at runtime.
	migrationDB, err := sql.Open("pgx", "postgres://migration@primary/app")
	if err != nil {
		return
	}
	defer migrationDB.Close()
	if _, err = migrationDB.ExecContext(ctx, sqlratelimit.SchemaSQL); err != nil {
		return
	}

	// The caller also owns and closes the runtime pool.
	runtimeDB, err := sql.Open("pgx", "postgres://runtime@primary/app")
	if err != nil {
		return
	}
	defer runtimeDB.Close()

	limiter, err := sqlratelimit.New(runtimeDB, sqlratelimit.Options{
		Namespace:     "api-v1",
		RatePerSecond: 100,
		Burst:         200,
		IdleTTL:       10 * time.Minute,
	})
	if err != nil {
		return
	}

	result, err := limiter.Allow(ctx, "tenant:blue", 1)
	if err != nil {
		// Ignore result on every error. Commit-unknown may have consumed once,
		// so do not replay automatically.
		_ = errors.Is(err, ratelimit.ErrCommitUnknown)
		return
	}
	_ = result

	// A caller-owned scheduler supplies a fresh bounded context per run.
	cleanupCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	deleted, err := limiter.Cleanup(cleanupCtx, 100)
	if err != nil {
		// Ignore deleted on every error. A retry advances current expired work;
		// it does not replay an idempotent batch.
		return
	}
	_ = deleted
}
