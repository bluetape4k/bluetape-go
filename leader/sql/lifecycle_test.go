package sqlleader

import (
	"context"
	"database/sql"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/bluetape4k/bluetape-go/leader"
	bttesting "github.com/bluetape4k/bluetape-go/testing"
)

func TestPostgresLifecycle(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	t.Cleanup(cancel)
	db := openPostgresDB(ctx, t)

	t.Run("campaign-blocks", func(t *testing.T) { testCampaignBlocksUntilContextOrTakeover(t, db) })
	t.Run("campaign-state", func(t *testing.T) { testCampaignRejectsAlreadyOwnedAndInProgress(t, db) })
	t.Run("renewal", func(t *testing.T) { testRenewalExtendsLease(t, db) })
	t.Run("renewal-loss", func(t *testing.T) { testZeroRowRenewalClearsLeadership(t, db) })
	t.Run("resign-token-safe", func(t *testing.T) { testResignIsIdempotentAndTokenSafe(t, db) })
	t.Run("resign-timeout", func(t *testing.T) { testResignTimeoutRetainsCleanupForRetry(t, db) })
	t.Run("generation", func(t *testing.T) { testOldGenerationCannotClearNewOwnership(t, db) })
	t.Run("backoff", func(t *testing.T) { testContentionBackoffIsBoundedAndNotTightLoop(t, db) })
	t.Run("leader-context", func(t *testing.T) { testLeaderRejectsNilAndCanceledContext(t, db) })
	t.Run("leader-empty", func(t *testing.T) { testLeaderReturnsEmptyForMissingOrExpiredLease(t, db) })
	t.Run("concurrent-resign", func(t *testing.T) { testConcurrentResignIsIdempotent(t, db) })
	t.Run("canceled-campaign", func(t *testing.T) { testCanceledCampaignThenResignLeavesNoWorker(t, db) })
	t.Run("constrained-pool", func(t *testing.T) { testConstrainedPoolTimesOutWithoutLeaseOverstay(t, db) })
	t.Run("shared-pool", func(t *testing.T) { testSharedPoolMultiElectorShutdown(t, db) })
}

func testCampaignBlocksUntilContextOrTakeover(t *testing.T, db *sql.DB) {
	owner := lifecycleElector(t, db, "campaign-blocks", "owner", 600*time.Millisecond, 150*time.Millisecond)
	contender := lifecycleElector(t, db, "campaign-blocks", "contender", 600*time.Millisecond, 150*time.Millisecond)
	ctx := context.Background()
	if err := owner.Campaign(ctx); err != nil {
		t.Fatal(err)
	}
	waitCtx, cancel := context.WithTimeout(ctx, 90*time.Millisecond)
	err := contender.Campaign(waitCtx)
	cancel()
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Campaign() error=%v, want deadline", err)
	}
	if err := owner.Resign(ctx); err != nil {
		t.Fatal(err)
	}
	takeoverCtx, takeoverCancel := context.WithTimeout(ctx, time.Second)
	defer takeoverCancel()
	if err := contender.Campaign(takeoverCtx); err != nil {
		t.Fatalf("takeover Campaign(): %v", err)
	}
	if err := contender.Resign(ctx); err != nil {
		t.Fatal(err)
	}
}

func testCampaignRejectsAlreadyOwnedAndInProgress(t *testing.T, db *sql.DB) {
	ctx := context.Background()
	owned := lifecycleElector(t, db, "campaign-owned", "member", time.Second, 250*time.Millisecond)
	if err := owned.Campaign(ctx); err != nil {
		t.Fatal(err)
	}
	if err := owned.Campaign(ctx); !errors.Is(err, leader.ErrAlreadyLeader) {
		t.Fatalf("Campaign() error=%v, want ErrAlreadyLeader", err)
	}
	defer func() { _ = owned.Resign(ctx) }()

	holder := lifecycleElector(t, db, "campaign-progress", "holder", time.Second, 250*time.Millisecond)
	waiter := lifecycleElector(t, db, "campaign-progress", "waiter", time.Second, 250*time.Millisecond)
	if err := holder.Campaign(ctx); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = holder.Resign(ctx) }()
	waitCtx, cancel := context.WithCancel(ctx)
	done := make(chan error, 1)
	go func() { done <- waiter.Campaign(waitCtx) }()
	bttesting.Eventually(t, time.Second, func() bool { return campaignActive(waiter) })
	if err := waiter.Campaign(ctx); !errors.Is(err, leader.ErrCampaignInProgress) {
		t.Fatalf("Campaign() error=%v, want ErrCampaignInProgress", err)
	}
	cancel()
	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Fatalf("blocked Campaign() error=%v", err)
	}
}

func testRenewalExtendsLease(t *testing.T, db *sql.DB) {
	e := lifecycleElector(t, db, "renewal-extends", "member", 450*time.Millisecond, 80*time.Millisecond)
	ctx := context.Background()
	if err := e.Campaign(ctx); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = e.Resign(ctx) }()
	before := leaseUntil(t, db, e.key)
	bttesting.Eventually(t, time.Second, func() bool { return leaseUntil(t, db, e.key).After(before) })
}

func testZeroRowRenewalClearsLeadership(t *testing.T, db *sql.DB) {
	e := lifecycleElector(t, db, "renewal-loss", "member", 400*time.Millisecond, 60*time.Millisecond)
	ctx := context.Background()
	if err := e.Campaign(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `delete from public.bluetape_leader_leases where leader_key=$1`, e.key); err != nil {
		t.Fatal(err)
	}
	bttesting.Eventually(t, time.Second, func() bool { return !e.IsLeader() })
	owned, cleanup, _, done := lifecycleState(e)
	if owned || cleanup || done != nil {
		t.Fatalf("state owned=%v cleanup=%v done=%v", owned, cleanup, done != nil)
	}
}

func testResignIsIdempotentAndTokenSafe(t *testing.T, db *sql.DB) {
	e := lifecycleElector(t, db, "resign-token-safe", "member", time.Second, 300*time.Millisecond)
	ctx := context.Background()
	if err := e.Campaign(ctx); err != nil {
		t.Fatal(err)
	}
	const replacement = "replacement-token"
	if _, err := db.ExecContext(ctx, `update public.bluetape_leader_leases set owner_token=$2 where leader_key=$1`, e.key, replacement); err != nil {
		t.Fatal(err)
	}
	if err := e.Resign(ctx); err != nil {
		t.Fatal(err)
	}
	if err := e.Resign(ctx); err != nil {
		t.Fatalf("second Resign(): %v", err)
	}
	var owner string
	if err := db.QueryRowContext(ctx, `select owner_token from public.bluetape_leader_leases where leader_key=$1`, e.key).Scan(&owner); err != nil {
		t.Fatal(err)
	}
	if owner != replacement {
		t.Fatalf("owner=%q, want replacement", owner)
	}
}

func testResignTimeoutRetainsCleanupForRetry(t *testing.T, db *sql.DB) {
	e := lifecycleElector(t, db, "resign-timeout", "member", 500*time.Millisecond, 50*time.Millisecond)
	entered := make(chan struct{})
	release := make(chan struct{})
	var once sync.Once
	e.testHook = func(operation, phase string) error {
		if operation == "renew" && phase == "after" {
			once.Do(func() { close(entered) })
			<-release
		}
		return nil
	}
	if err := e.Campaign(context.Background()); err != nil {
		t.Fatal(err)
	}
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("renew hook was not reached")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	err := e.Resign(ctx)
	cancel()
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Resign() error=%v, want deadline", err)
	}
	owned, cleanup, cancelFn, done := lifecycleState(e)
	if owned || !cleanup || cancelFn == nil || done == nil {
		t.Fatalf("retry state owned=%v cleanup=%v cancel=%v done=%v", owned, cleanup, cancelFn != nil, done != nil)
	}
	close(release)
	retryCtx, retryCancel := context.WithTimeout(context.Background(), time.Second)
	defer retryCancel()
	if err := e.Resign(retryCtx); err != nil {
		t.Fatalf("retry Resign(): %v", err)
	}
	_, cleanup, cancelFn, done = lifecycleState(e)
	if cleanup || cancelFn != nil || done != nil {
		t.Fatalf("cleanup retained after retry: cleanup=%v cancel=%v done=%v", cleanup, cancelFn != nil, done != nil)
	}
}

func testOldGenerationCannotClearNewOwnership(t *testing.T, db *sql.DB) {
	e := lifecycleElector(t, db, "old-generation", "member", time.Second, 250*time.Millisecond)
	oldDone := make(chan struct{})
	currentDone := make(chan struct{})
	e.mu.Lock()
	e.owned = true
	e.generation = 2
	e.done = currentDone
	e.mu.Unlock()
	e.clearOwnershipAfterLoss(1, oldDone, true)
	owned, cleanup, _, done := lifecycleState(e)
	if !owned || cleanup || done != currentDone {
		t.Fatalf("old generation changed current state")
	}
}

func testContentionBackoffIsBoundedAndNotTightLoop(t *testing.T, _ *sql.DB) {
	b := newBackoff("stable-owner-token", 400*time.Millisecond)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	started := time.Now()
	for range 4 {
		if err := b.wait(ctx); err != nil {
			t.Fatal(err)
		}
	}
	elapsed := time.Since(started)
	if elapsed < 70*time.Millisecond || elapsed > 450*time.Millisecond {
		t.Fatalf("four waits took %s", elapsed)
	}
	done, doneCancel := context.WithCancel(ctx)
	doneCancel()
	started = time.Now()
	if err := b.wait(done); !errors.Is(err, context.Canceled) {
		t.Fatalf("wait() error=%v", err)
	}
	if time.Since(started) > 20*time.Millisecond {
		t.Fatal("canceled wait allocated a visible delay")
	}
}

func testLeaderRejectsNilAndCanceledContext(t *testing.T, db *sql.DB) {
	e := lifecycleElector(t, db, "leader-context", "member", time.Second, 250*time.Millisecond)
	if _, err := e.Leader(nil); !errors.Is(err, leader.ErrInvalidContext) {
		t.Fatalf("Leader(nil) error=%v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := e.Leader(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("Leader(canceled) error=%v", err)
	}
}

func testLeaderReturnsEmptyForMissingOrExpiredLease(t *testing.T, db *sql.DB) {
	e := lifecycleElector(t, db, "leader-empty", "member", time.Second, 250*time.Millisecond)
	ctx := context.Background()
	owner, err := e.Leader(ctx)
	if err != nil || owner != "" {
		t.Fatalf("missing Leader() owner=%q err=%v", owner, err)
	}
	if _, err := db.ExecContext(ctx, `insert into public.bluetape_leader_leases
(leader_key,group_name,member_id,owner_token,lease_until,created_at,updated_at)
values($1,'g','m','expired',pg_catalog.clock_timestamp()-interval '1 second',
pg_catalog.clock_timestamp(),pg_catalog.clock_timestamp())`, e.key); err != nil {
		t.Fatal(err)
	}
	owner, err = e.Leader(ctx)
	if err != nil || owner != "" {
		t.Fatalf("expired Leader() owner=%q err=%v", owner, err)
	}
}

func testConcurrentResignIsIdempotent(t *testing.T, db *sql.DB) {
	e := lifecycleElector(t, db, "concurrent-resign", "member", time.Second, 100*time.Millisecond)
	if err := e.Campaign(context.Background()); err != nil {
		t.Fatal(err)
	}
	const callers = 8
	errs := make(chan error, callers)
	var wg sync.WaitGroup
	for range callers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			errs <- e.Resign(ctx)
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("Resign(): %v", err)
		}
	}
	if e.IsLeader() {
		t.Fatal("elector remained leader")
	}
}

func testCanceledCampaignThenResignLeavesNoWorker(t *testing.T, db *sql.DB) {
	holder := lifecycleElector(t, db, "canceled-campaign", "holder", time.Second, 200*time.Millisecond)
	waiter := lifecycleElector(t, db, "canceled-campaign", "waiter", time.Second, 200*time.Millisecond)
	if err := holder.Campaign(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = holder.Resign(context.Background()) }()
	ctx, cancel := context.WithTimeout(context.Background(), 70*time.Millisecond)
	err := waiter.Campaign(ctx)
	cancel()
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Campaign() error=%v", err)
	}
	if err := waiter.Resign(context.Background()); err != nil {
		t.Fatal(err)
	}
	owned, cleanup, cancelFn, done := lifecycleState(waiter)
	if owned || cleanup || cancelFn != nil || done != nil || campaignActive(waiter) {
		t.Fatal("canceled campaign retained lifecycle state")
	}
}

func testConstrainedPoolTimesOutWithoutLeaseOverstay(t *testing.T, db *sql.DB) {
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { db.SetMaxOpenConns(0) })
	e := lifecycleElector(t, db, "constrained-pool", "member", 300*time.Millisecond, 60*time.Millisecond)
	if err := e.Campaign(context.Background()); err != nil {
		t.Fatal(err)
	}
	conn, err := db.Conn(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	before := db.Stats()
	bttesting.Eventually(t, time.Second, func() bool { return !e.IsLeader() })
	after := db.Stats()
	if after.WaitCount <= before.WaitCount || after.WaitDuration <= before.WaitDuration {
		t.Fatalf("pool wait was not recorded: before=%+v after=%+v", before, after)
	}
	if err := conn.Close(); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := e.Resign(ctx); err != nil {
		t.Fatal(err)
	}
}

func testSharedPoolMultiElectorShutdown(t *testing.T, db *sql.DB) {
	const count = 3
	electors := make([]*Elector, 0, count)
	var renews atomic.Int64
	for i := range count {
		e := lifecycleElector(t, db, "shared-pool-"+string(rune('a'+i)), "member", 400*time.Millisecond, 70*time.Millisecond)
		e.testHook = func(operation, phase string) error {
			if operation == "renew" && phase == "after" {
				renews.Add(1)
			}
			return nil
		}
		if err := e.Campaign(context.Background()); err != nil {
			t.Fatal(err)
		}
		electors = append(electors, e)
	}
	bttesting.Eventually(t, time.Second, func() bool { return renews.Load() >= count })
	var wg sync.WaitGroup
	for _, e := range electors {
		wg.Add(1)
		go func(e *Elector) {
			defer wg.Done()
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			if err := e.Resign(ctx); err != nil {
				t.Errorf("Resign(): %v", err)
			}
		}(e)
	}
	wg.Wait()
	stopped := renews.Load()
	bttesting.Consistently(t, 180*time.Millisecond, func() bool { return renews.Load() == stopped })
	for _, e := range electors {
		owned, cleanup, cancelFn, done := lifecycleState(e)
		if owned || cleanup || cancelFn != nil || done != nil {
			t.Fatal("shared elector retained cleanup inventory")
		}
	}
	bttesting.Eventually(t, time.Second, func() bool { return db.Stats().InUse == 0 })
}

func lifecycleElector(t *testing.T, db *sql.DB, group, member string, lease, renew time.Duration) *Elector {
	t.Helper()
	e, err := New(db, leader.Options{
		Group: group, MemberID: member, KeyPrefix: "sqlleader-lifecycle", Lease: lease, RenewInterval: renew,
	})
	if err != nil {
		t.Fatal(err)
	}
	return e
}

func leaseUntil(t *testing.T, db *sql.DB, key string) time.Time {
	t.Helper()
	var value time.Time
	if err := db.QueryRow(`select lease_until from public.bluetape_leader_leases where leader_key=$1`, key).Scan(&value); err != nil {
		t.Fatal(err)
	}
	return value
}

func campaignActive(e *Elector) bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.campaigning
}

func lifecycleState(e *Elector) (bool, bool, context.CancelFunc, chan struct{}) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.owned, e.cleanup, e.cancel, e.done
}
