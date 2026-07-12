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
	run := func(ctx context.Context, startProtectedWork func(context.Context)) (resultErr error) {
		runtimeDB, err := sql.Open("pgx", "postgres://app_runtime:password@primary/app")
		if err != nil {
			return err
		}
		defer func() { _ = runtimeDB.Close() }()

		// A migration owner must apply sqlleader.SchemaSQL before this runtime pool starts.
		opts := leader.Options{
			Group:         "billing-workers",
			MemberID:      "worker-1",
			Lease:         30 * time.Second,
			RenewInterval: 10 * time.Second,
		}
		elector, err := sqlleader.New(runtimeDB, opts)
		if err != nil {
			return err
		}
		campaignCtx, cancelCampaign := context.WithTimeout(ctx, 15*time.Second)
		defer cancelCampaign()
		campaignErr := elector.Campaign(campaignCtx)
		if campaignErr != nil {
			if errors.Is(campaignErr, leader.ErrCommitUnknown) || errors.Is(campaignErr, leader.ErrCleanupPending) {
				return errors.Join(campaignErr, boundedResign(elector, opts.Lease))
			}
			return campaignErr
		}
		defer func() { resultErr = errors.Join(resultErr, boundedResign(elector, opts.Lease)) }()
		if campaignCtx.Err() != nil {
			return campaignCtx.Err()
		}

		protectedCtx, stopProtectedWork := context.WithCancel(ctx)
		defer stopProtectedWork()
		startProtectedWork(protectedCtx)
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
					return leader.ErrNotLeader
				}
			}
		}
	}

	_ = run(context.Background(), func(context.Context) {})
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
