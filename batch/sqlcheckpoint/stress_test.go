package sqlcheckpoint

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/bluetape4k/bluetape-go/batch"
	"github.com/bluetape4k/bluetape-go/sqlkit"
)

func TestPostgresConcurrentConflictHasExactWinnerAndLoser(t *testing.T) {
	fixture := newPostgresFixture(t)
	secondPool := openPostgresPool(fixture.ctx, t, fixture.dsn)
	assertPostgresConcurrentConflictHasExactWinnerAndLoser(
		fixture.ctx, t, fixture.db, secondPool, "conflict", 8,
	)
}

func TestPostgresConcurrentConflictIgnoresRepeatableReadDefault(t *testing.T) {
	fixture := newPostgresFixture(t)
	firstPool := openPostgresPoolWithDefaultIsolation(
		fixture.ctx, t, fixture.dsn, "repeatable read",
	)
	secondPool := openPostgresPoolWithDefaultIsolation(
		fixture.ctx, t, fixture.dsn, "repeatable read",
	)
	assertPostgresConcurrentConflictHasExactWinnerAndLoser(
		fixture.ctx, t, firstPool, secondPool, "repeatable-read-conflict", 4,
	)
}

func assertPostgresConcurrentConflictHasExactWinnerAndLoser(
	ctx context.Context,
	t *testing.T,
	firstPool, secondPool *sql.DB,
	prefix string,
	iterations int,
) {
	t.Helper()

	for iteration := range iterations {
		namespace := fmt.Sprintf("%s-%02d", prefix, iteration)
		arrived := make(chan struct{}, 2)
		release := make(chan struct{})
		callback := func(ctx context.Context, session sqlkit.Session, items []string) error {
			if _, err := session.ExecContext(ctx,
				`insert into public.sqlcheckpoint_business(id,payload) values ($1,$2)`, items[0], namespace); err != nil {
				return err
			}
			arrived <- struct{}{}
			select {
			case <-release:
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		}
		writers := []*Writer[string, string]{
			newPostgresWriter(t, firstPool, namespace, callback),
			newPostgresWriter(t, secondPool, namespace, callback),
		}
		for index, writer := range writers {
			checkpoint, found, err := writer.Load(ctx, "job")
			if err != nil || found || checkpoint.Version != 0 {
				t.Fatalf("iteration %d writer %d Load = %+v, %v, %v; want missing", iteration, index, checkpoint, found, err)
			}
		}

		type outcome struct {
			writerIndex     int
			businessID      string
			checkpointValue string
			revision        uint64
			err             error
		}
		outcomes := make(chan outcome, 2)
		var workers sync.WaitGroup
		for index, writer := range writers {
			businessID := fmt.Sprintf("%s-%02d-%d", prefix, iteration, index)
			checkpointValue := fmt.Sprintf("winner-%d", index)
			workers.Add(1)
			go func() {
				defer workers.Done()
				revision, err := writer.Commit(ctx, "job", 0,
					[]string{businessID}, checkpointValue)
				outcomes <- outcome{
					writerIndex: index, businessID: businessID, checkpointValue: checkpointValue,
					revision: revision, err: err,
				}
			}()
		}
		for range 2 {
			select {
			case <-arrived:
			case <-time.After(5 * time.Second):
				t.Fatalf("iteration %d callbacks did not reach pre-CAS barrier", iteration)
			}
		}
		close(release)
		workers.Wait()
		close(outcomes)

		winners, conflicts := 0, 0
		var winner, loser outcome
		for result := range outcomes {
			switch {
			case result.err == nil && result.revision == 1:
				winners++
				winner = result
			case result.revision == 0 && errors.Is(result.err, batch.ErrCheckpointConflict):
				conflicts++
				loser = result
			default:
				t.Fatalf("iteration %d unexpected outcome revision=%d err=%v", iteration, result.revision, result.err)
			}
		}
		if winners != 1 || conflicts != 1 {
			t.Fatalf("iteration %d winners=%d conflicts=%d; want 1/1", iteration, winners, conflicts)
		}

		for _, expected := range []struct {
			id   string
			want int
		}{{id: winner.businessID, want: 1}, {id: loser.businessID, want: 0}} {
			var count int
			if err := firstPool.QueryRowContext(ctx,
				`select count(*) from public.sqlcheckpoint_business where id=$1`, expected.id).Scan(&count); err != nil {
				t.Fatal(err)
			}
			if count != expected.want {
				t.Fatalf("iteration %d business %q count=%d want=%d", iteration, expected.id, count, expected.want)
			}
		}
		loaded, found, err := writers[winner.writerIndex].Load(ctx, "job")
		if err != nil || !found || loaded.Version != 1 || loaded.Value != winner.checkpointValue {
			t.Fatalf("iteration %d winner=%d Load=%+v found=%v err=%v; want matching payload %q",
				iteration, winner.writerIndex, loaded, found, err, winner.checkpointValue)
		}
	}
}

func TestPostgresCancellationOwnershipStressIsBounded(t *testing.T) {
	fixture := newPostgresFixture(t)
	fixture.db.SetMaxOpenConns(1)
	fixture.db.SetMaxIdleConns(1)

	const iterations = 12
	var callbackCalls atomic.Int64
	proofs := make([]cancellationProof, 0, iterations)
	for iteration := range iterations {
		namespace := fmt.Sprintf("cancel-%02d", iteration)
		key := "job"
		id := fmt.Sprintf("cancel-stress-%02d", iteration)
		var iterationCallbackCalls atomic.Int64
		observation := new(transactionObservation)
		writer := newPostgresWriter(t, fixture.db, namespace,
			func(ctx context.Context, session sqlkit.Session, items []string) error {
				callbackCalls.Add(1)
				iterationCallbackCalls.Add(1)
				if _, err := session.ExecContext(ctx,
					`insert into public.sqlcheckpoint_business(id,payload) values ($1,'cancel-stress')`, items[0]); err != nil {
					return err
				}
				return context.Canceled
			})
		observeWriter(writer, observation)
		revision, err := writer.Commit(fixture.ctx, key, 0, []string{id}, "checkpoint")
		if revision != 0 || !errors.Is(err, context.Canceled) || errors.Is(err, batch.ErrCommitUnknown) {
			t.Fatalf("iteration %d Commit = %d, %v; want known cancellation", iteration, revision, err)
		}
		if observation.scans.Load() != 0 || observation.commits.Load() != 0 || observation.rollbacks.Load() != 1 {
			t.Fatalf("iteration %d CAS/Commit/rollback=%d/%d/%d; want 0/0/1", iteration,
				observation.scans.Load(), observation.commits.Load(), observation.rollbacks.Load())
		}
		if iterationCallbackCalls.Load() != 1 {
			t.Fatalf("iteration %d callback calls=%d want=1", iteration, iterationCallbackCalls.Load())
		}
		assertBusinessIDAbsent(fixture.ctx, t, fixture.db, id)
		proofs = append(proofs, retainCancellationProof(fmt.Sprintf("stress-%02d", iteration), namespace, key,
			[]string{id}, writer, &iterationCallbackCalls, observation))
	}
	callbackSnapshot := callbackCalls.Load()
	if callbackSnapshot != iterations {
		t.Fatalf("callback calls=%d want=%d", callbackSnapshot, iterations)
	}
	var aggregateScansSnapshot, aggregateCommitsSnapshot int64
	for _, proof := range proofs {
		aggregateScansSnapshot += proof.observationSnapshot.scans
		aggregateCommitsSnapshot += proof.observationSnapshot.commits
	}

	reuseCtx, cancel := context.WithTimeout(fixture.ctx, 1500*time.Millisecond)
	defer cancel()
	if err := fixture.db.PingContext(reuseCtx); err != nil {
		t.Fatalf("single connection was not reusable within 1s + 500ms: %v", err)
	}
	deadline := time.Now().Add(1500 * time.Millisecond)
	for fixture.db.Stats().InUse != 0 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if stats := fixture.db.Stats(); stats.InUse != 0 {
		t.Fatalf("pool ownership leak: %+v", stats)
	}
	if got := callbackCalls.Load(); got != callbackSnapshot {
		t.Fatalf("aggregate callback calls changed after drain: before=%d after=%d", callbackSnapshot, got)
	}
	var aggregateScansAfter, aggregateCommitsAfter int64
	for _, proof := range proofs {
		assertCancellationProofStable(fixture.ctx, t, fixture.db, proof)
		observationAfter := proof.observation.snapshot()
		aggregateScansAfter += observationAfter.scans
		aggregateCommitsAfter += observationAfter.commits
	}
	if aggregateScansAfter != aggregateScansSnapshot || aggregateCommitsAfter != aggregateCommitsSnapshot {
		t.Fatalf("aggregate CAS/Commit changed after drain: before=%d/%d after=%d/%d",
			aggregateScansSnapshot, aggregateCommitsSnapshot, aggregateScansAfter, aggregateCommitsAfter)
	}
	var lateRows int
	if err := fixture.db.QueryRowContext(fixture.ctx,
		`select count(*) from public.sqlcheckpoint_business where payload='cancel-stress'`).Scan(&lateRows); err != nil {
		t.Fatal(err)
	}
	if lateRows != 0 {
		t.Fatalf("late business writes=%d", lateRows)
	}
}
