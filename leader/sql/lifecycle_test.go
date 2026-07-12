package sqlleader

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/bluetape4k/bluetape-go/leader"
	bttesting "github.com/bluetape4k/bluetape-go/testing"
)

func TestPostgresFaultRecovery(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	t.Cleanup(cancel)
	db := openPostgresDB(ctx, t)

	t.Run("acquire-lost-response", func(t *testing.T) { testAcquireLostResponseReconcilesOwnToken(t, db) })
	t.Run("acquire-probe-failure", func(t *testing.T) { testAcquireProbeFailureReturnsCommitUnknown(t, db) })
	t.Run("attempt-timeout", func(t *testing.T) { testInternalAttemptTimeoutWithOtherOwnerRetries(t, db) })
	t.Run("renew-lost-response", func(t *testing.T) { testRenewLostResponseClearsOwnedAndKeepsCleanup(t, db) })
	t.Run("resign-lost-response", func(t *testing.T) { testResignLostResponseIsCommitUnknownThenRetryable(t, db) })
	t.Run("redaction", func(t *testing.T) { testPostgresOperationErrorRedactsMarkers(t, db) })
	t.Run("fault-matrix", func(t *testing.T) { testMutationFaultMatrix(t, db) })
	t.Run("backend-termination", func(t *testing.T) { testBackendTerminationRecoveryAndTakeover(t, db) })
}

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

func testAcquireLostResponseReconcilesOwnToken(t *testing.T, db *sql.DB) {
	e := lifecycleElector(t, db, "fault-acquire-after", "member", time.Second, 100*time.Millisecond)
	faults := newFaultController()
	marker := errors.New("lost acquire response")
	faults.failNext("campaign", "after", marker)
	e.testHook = faults.hook
	if err := e.Campaign(context.Background()); err != nil {
		t.Fatalf("Campaign() did not reconcile own token: %v", err)
	}
	if !e.IsLeader() || faults.count("campaign", "reconcile") != 1 {
		t.Fatalf("owned=%v reconcile=%d", e.IsLeader(), faults.count("campaign", "reconcile"))
	}
	if err := e.Resign(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func testAcquireProbeFailureReturnsCommitUnknown(t *testing.T, db *sql.DB) {
	e := lifecycleElector(t, db, "fault-acquire-reconcile", "member", time.Second, 100*time.Millisecond)
	faults := newFaultController()
	afterErr := errors.New("lost campaign response")
	faults.failNext("campaign", "after", afterErr)
	faults.failNext("campaign", "reconcile", errors.New("primary probe unavailable"))
	e.testHook = faults.hook
	err := e.Campaign(context.Background())
	if !errors.Is(err, leader.ErrCommitUnknown) || !errors.Is(err, afterErr) {
		t.Fatalf("Campaign() error=%v, want cause and ErrCommitUnknown", err)
	}
	owned, cleanup, _, _ := lifecycleState(e)
	if owned || !cleanup {
		t.Fatalf("owned=%v cleanup=%v", owned, cleanup)
	}
	if err := e.Campaign(context.Background()); !errors.Is(err, leader.ErrCleanupPending) {
		t.Fatalf("Campaign() error=%v, want ErrCleanupPending", err)
	}
	if err := e.Resign(context.Background()); err != nil {
		t.Fatalf("cleanup Resign(): %v", err)
	}
}

func testInternalAttemptTimeoutWithOtherOwnerRetries(t *testing.T, db *sql.DB) {
	owner := lifecycleElector(t, db, "fault-attempt-timeout", "owner", time.Second, 100*time.Millisecond)
	contender := lifecycleElector(t, db, "fault-attempt-timeout", "contender", time.Second, 100*time.Millisecond)
	if acquired, err := owner.tryAcquire(context.Background()); err != nil || !acquired {
		t.Fatalf("owner acquire=%v err=%v", acquired, err)
	}
	tx, err := db.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	rolledBack := false
	t.Cleanup(func() {
		if !rolledBack {
			_ = tx.Rollback()
		}
	})
	if _, err := tx.Exec(`update public.bluetape_leader_leases set updated_at=updated_at where leader_key=$1`, owner.key); err != nil {
		t.Fatal(err)
	}
	faults := newFaultController()
	contender.testHook = faults.hook
	campaignCtx, cancel := context.WithTimeout(context.Background(), 360*time.Millisecond)
	done := make(chan error, 1)
	started := time.Now()
	go func() { done <- contender.Campaign(campaignCtx) }()
	waitForBlockedQuery(t, db, "insert into public.bluetape_leader_leases")
	bttesting.Eventually(t, time.Second, func() bool {
		return faults.count("campaign", "reconcile") >= 1
	})
	if err := tx.Rollback(); err != nil {
		t.Fatal(err)
	}
	rolledBack = true
	err = <-done
	cancel()
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Campaign() error=%v, want deadline", err)
	}
	if time.Since(started) < 150*time.Millisecond {
		t.Fatalf("Campaign() retried without bounded backoff")
	}
	if cleanupPending(contender) {
		t.Fatal("confirmed other owner left cleanup pending")
	}
}

func testRenewLostResponseClearsOwnedAndKeepsCleanup(t *testing.T, db *sql.DB) {
	e := lifecycleElector(t, db, "fault-renew-after", "member", 500*time.Millisecond, 60*time.Millisecond)
	faults := newFaultController()
	faults.failNext("renew", "after", errors.New("lost renewal response"))
	e.testHook = faults.hook
	if err := e.Campaign(context.Background()); err != nil {
		t.Fatal(err)
	}
	bttesting.Eventually(t, time.Second, func() bool { return !e.IsLeader() })
	if !cleanupPending(e) {
		t.Fatal("indeterminate renewal did not retain cleanup")
	}
	count := faults.count("renew", "before")
	bttesting.Consistently(t, 150*time.Millisecond, func() bool {
		return faults.count("renew", "before") == count
	})
	if err := e.Resign(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func testResignLostResponseIsCommitUnknownThenRetryable(t *testing.T, db *sql.DB) {
	e := lifecycleElector(t, db, "fault-resign-after", "member", time.Second, 100*time.Millisecond)
	faults := newFaultController()
	faults.failNext("resign", "after", errors.New("lost delete response"))
	e.testHook = faults.hook
	if err := e.Campaign(context.Background()); err != nil {
		t.Fatal(err)
	}
	err := e.Resign(context.Background())
	if !errors.Is(err, leader.ErrCommitUnknown) || !cleanupPending(e) {
		t.Fatalf("Resign() error=%v cleanup=%v", err, cleanupPending(e))
	}
	if err := e.Resign(context.Background()); err != nil {
		t.Fatalf("retry Resign(): %v", err)
	}
	if cleanupPending(e) {
		t.Fatal("retry did not clear cleanup")
	}
}

func testPostgresOperationErrorRedactsMarkers(t *testing.T, db *sql.DB) {
	markers := []string{
		"postgres://user:secret@db.internal:5432/app",
		"db.internal:5432",
		"public.bluetape_leader_leases",
		"bluetape_leader_leases_pkey",
		"redaction-group",
		"redaction-member",
		"sqlleader-lifecycle:redaction-group",
		"redaction-member:owner-token",
	}
	cause := errors.New(strings.Join(markers, " | "))
	e := lifecycleElector(t, db, "redaction-group", "redaction-member", time.Second, 100*time.Millisecond)
	faults := newFaultController()
	faults.failNext("campaign", "before", cause)
	e.testHook = faults.hook
	err := e.Campaign(context.Background())
	if !errors.Is(err, cause) {
		t.Fatalf("errors.Is did not preserve cause: %v", err)
	}
	var operationErr *leader.OperationError
	if !errors.As(err, &operationErr) || operationErr.Backend() != "postgres" || operationErr.Operation() != "campaign" {
		t.Fatalf("operation error=%#v", operationErr)
	}
	for _, marker := range markers {
		if strings.Contains(err.Error(), marker) {
			t.Fatalf("rendered error leaked %q: %v", marker, err)
		}
	}
}

func testMutationFaultMatrix(t *testing.T, db *sql.DB) {
	tests := []struct {
		name           string
		operation      string
		phase          string
		wantUnknown    bool
		wantCleanup    bool
		wantCampaignOK bool
	}{
		{"campaign-before", "campaign", "before", false, false, false},
		{"campaign-after", "campaign", "after", false, false, true},
		{"renew-before", "renew", "before", false, false, true},
		{"renew-after", "renew", "after", false, true, true},
		{"resign-before", "resign", "before", false, true, true},
		{"resign-after", "resign", "after", true, true, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := lifecycleElector(t, db, "matrix-"+tt.name, "member", 500*time.Millisecond, 60*time.Millisecond)
			faults := newFaultController()
			marker := fmt.Errorf("%s-%s marker", tt.operation, tt.phase)
			faults.failNext(tt.operation, tt.phase, marker)
			e.testHook = faults.hook
			err := e.Campaign(context.Background())
			if tt.wantCampaignOK && err != nil {
				t.Fatalf("Campaign(): %v", err)
			}
			if !tt.wantCampaignOK && !errors.Is(err, marker) {
				t.Fatalf("Campaign() error=%v, want marker", err)
			}
			switch tt.operation {
			case "renew":
				bttesting.Eventually(t, time.Second, func() bool { return !e.IsLeader() })
			case "resign":
				err = e.Resign(context.Background())
			}
			if errors.Is(err, leader.ErrCommitUnknown) != tt.wantUnknown {
				t.Fatalf("error=%v wantUnknown=%v", err, tt.wantUnknown)
			}
			if cleanupPending(e) != tt.wantCleanup {
				t.Fatalf("cleanup=%v want=%v", cleanupPending(e), tt.wantCleanup)
			}
			if cleanupPending(e) {
				if err := e.Campaign(context.Background()); !errors.Is(err, leader.ErrCleanupPending) {
					t.Fatalf("Campaign() error=%v, want cleanup pending", err)
				}
			}
			if err := e.Resign(context.Background()); err != nil {
				t.Fatalf("final Resign(): %v", err)
			}
		})
	}
}

func testBackendTerminationRecoveryAndTakeover(t *testing.T, db *sql.DB) {
	e := lifecycleElector(t, db, "backend-termination", "owner", 450*time.Millisecond, 100*time.Millisecond)
	if err := e.Campaign(context.Background()); err != nil {
		t.Fatal(err)
	}
	tx, err := db.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	rolledBack := false
	t.Cleanup(func() {
		if !rolledBack {
			_ = tx.Rollback()
		}
	})
	if _, err := tx.Exec(`update public.bluetape_leader_leases set updated_at=updated_at where leader_key=$1`, e.key); err != nil {
		t.Fatal(err)
	}
	pid := waitForBlockedQuery(t, db, "update public.bluetape_leader_leases")
	var terminated bool
	if err := db.QueryRow(`select pg_catalog.pg_terminate_backend($1)`, pid).Scan(&terminated); err != nil || !terminated {
		t.Fatalf("terminate backend pid=%d terminated=%v err=%v", pid, terminated, err)
	}
	bttesting.Eventually(t, time.Second, func() bool { return !e.IsLeader() && cleanupPending(e) })
	if err := tx.Rollback(); err != nil {
		t.Fatal(err)
	}
	rolledBack = true
	if err := db.PingContext(context.Background()); err != nil {
		t.Fatalf("pool did not reconnect: %v", err)
	}
	cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), time.Second)
	defer cleanupCancel()
	if err := e.Resign(cleanupCtx); err != nil {
		t.Fatalf("cleanup Resign(): %v", err)
	}
	contender := lifecycleElector(t, db, "backend-termination", "contender", 450*time.Millisecond, 100*time.Millisecond)
	if err := contender.Campaign(context.Background()); err != nil {
		t.Fatalf("takeover Campaign(): %v", err)
	}
	if err := contender.Resign(context.Background()); err != nil {
		t.Fatal(err)
	}
}

type faultController struct {
	mu     sync.Mutex
	faults map[string][]error
	counts map[string]int
}

func newFaultController() *faultController {
	return &faultController{faults: make(map[string][]error), counts: make(map[string]int)}
}

func (f *faultController) failNext(operation, phase string, err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	key := operation + "/" + phase
	f.faults[key] = append(f.faults[key], err)
}

func (f *faultController) hook(operation, phase string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	key := operation + "/" + phase
	f.counts[key]++
	if len(f.faults[key]) == 0 {
		return nil
	}
	err := f.faults[key][0]
	f.faults[key] = f.faults[key][1:]
	return err
}

func (f *faultController) count(operation, phase string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.counts[operation+"/"+phase]
}

func waitForBlockedQuery(t *testing.T, db *sql.DB, prefix string) int {
	t.Helper()
	var pid int
	bttesting.Eventually(t, time.Second, func() bool {
		err := db.QueryRow(`select pid from pg_catalog.pg_stat_activity
where pid <> pg_catalog.pg_backend_pid() and state='active'
  and wait_event_type='Lock' and lower(query) like $1
order by query_start desc limit 1`, strings.ToLower(prefix)+"%").Scan(&pid)
		return err == nil
	})
	return pid
}

func cleanupPending(e *Elector) bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.cleanup
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
