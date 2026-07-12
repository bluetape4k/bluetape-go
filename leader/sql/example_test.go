package sqlleader_test

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/bluetape4k/bluetape-go/leader"
	sqlleader "github.com/bluetape4k/bluetape-go/leader/sql"
	_ "github.com/jackc/pgx/v5/stdlib"
)

func ExampleNew() {
	// runPostgresLeader is compile-checked but not executed by this example.
	_ = runPostgresLeader
}

func runPostgresLeader(ctx context.Context, dsn string, stopProtectedWork func()) error {
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return err
	}
	defer db.Close()

	// Development/bootstrap only. Production runs SchemaSQL as a migration.
	if _, err := db.ExecContext(ctx, sqlleader.SchemaSQL); err != nil {
		return err
	}
	opts := leader.Options{
		Group:         "billing-workers",
		MemberID:      "worker-1",
		Lease:         30 * time.Second,
		RenewInterval: 10 * time.Second,
	}
	elector, err := sqlleader.New(db, opts)
	if err != nil {
		return err
	}

	campaignCtx, campaignCancel := context.WithTimeout(ctx, 15*time.Second)
	defer campaignCancel()
	err = elector.Campaign(campaignCtx)
	switch {
	case err == nil:
		// A confirmed owner-token probe may return success even after campaignCtx expired.
		// From this point the caller owns cleanup and must not discard elector.
		defer boundedResign(elector, opts.Lease)
		if campaignCtx.Err() != nil {
			return campaignCtx.Err()
		}
	case errors.Is(err, leader.ErrCommitUnknown), errors.Is(err, leader.ErrCleanupPending):
		stopProtectedWork()
		return boundedResign(elector, opts.Lease)
	default:
		return err
	}

	poll := time.NewTicker(opts.RenewInterval / 2)
	defer poll.Stop()
	for {
		select {
		case <-ctx.Done():
			stopProtectedWork()
			return ctx.Err()
		case <-poll.C:
			if !elector.IsLeader() {
				stopProtectedWork()
				return boundedResign(elector, opts.Lease)
			}
		}
	}
}

func boundedResign(elector leader.Elector, lease time.Duration) error {
	var lastErr error
	var lastFailure time.Time
	attemptTimeout := min(5*time.Second, max(100*time.Millisecond, lease/4))
	for range 3 {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), attemptTimeout)
		lastErr = elector.Resign(cleanupCtx)
		cancel()
		if lastErr == nil {
			return nil
		}
		lastFailure = time.Now()
	}
	if wait := time.Until(lastFailure.Add(lease)); wait > 0 {
		timer := time.NewTimer(wait)
		<-timer.C
	}
	return lastErr
}
