package sqlcheckpoint

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/bluetape4k/bluetape-go/batch"
	"github.com/bluetape4k/bluetape-go/sqlkit"
	postgrestestcontainer "github.com/bluetape4k/bluetape-go/testcontainers/postgres"
	_ "github.com/jackc/pgx/v5/stdlib"
)

const postgresTestTimeout = 90 * time.Second

type postgresFixture struct {
	ctx context.Context
	dsn string
	db  *sql.DB
}

type idempotentServerCleanupTB struct {
	testing.TB
	terminated atomic.Bool
}

func (tb *idempotentServerCleanupTB) Cleanup(cleanup func()) {
	tb.TB.Cleanup(func() {
		if !tb.terminated.Load() {
			cleanup()
		}
	})
}

type transactionObservation struct {
	begins        atomic.Int64
	savepoints    atomic.Int64
	releases      atomic.Int64
	rollbackTos   atomic.Int64
	scans         atomic.Int64
	commits       atomic.Int64
	rollbacks     atomic.Int64
	beforeRelease func(*observedTransaction)
	afterRelease  func(*observedTransaction)
	beforeCAS     func(*observedTransaction)
	afterCAS      func(*observedTransaction)
	beforeCommit  func(*observedTransaction)
}

type transactionObservationSnapshot struct {
	begins      int64
	savepoints  int64
	releases    int64
	rollbackTos int64
	scans       int64
	commits     int64
	rollbacks   int64
}

func (observation *transactionObservation) snapshot() transactionObservationSnapshot {
	return transactionObservationSnapshot{
		begins:      observation.begins.Load(),
		savepoints:  observation.savepoints.Load(),
		releases:    observation.releases.Load(),
		rollbackTos: observation.rollbackTos.Load(),
		scans:       observation.scans.Load(),
		commits:     observation.commits.Load(),
		rollbacks:   observation.rollbacks.Load(),
	}
}

type cancellationProof struct {
	name                string
	namespace           string
	key                 string
	businessIDs         []string
	writer              *Writer[string, string]
	callbackCalls       *atomic.Int64
	callbackSnapshot    int64
	observation         *transactionObservation
	observationSnapshot transactionObservationSnapshot
}

func retainCancellationProof(
	name, namespace, key string,
	businessIDs []string,
	writer *Writer[string, string],
	callbackCalls *atomic.Int64,
	observation *transactionObservation,
) cancellationProof {
	return cancellationProof{
		name: name, namespace: namespace, key: key, businessIDs: businessIDs,
		writer: writer, callbackCalls: callbackCalls, callbackSnapshot: callbackCalls.Load(),
		observation: observation, observationSnapshot: observation.snapshot(),
	}
}

func countedPostgresWrite(callbackCalls *atomic.Int64) WriteTxFunc[string] {
	return func(ctx context.Context, session sqlkit.Session, items []string) error {
		callbackCalls.Add(1)
		for _, item := range items {
			if _, err := session.ExecContext(ctx,
				`insert into public.sqlcheckpoint_business(id,payload) values ($1,$2)`, item, "payload:"+item); err != nil {
				return err
			}
		}
		return nil
	}
}

func assertCancellationProofStable(ctx context.Context, t *testing.T, db *sql.DB, proof cancellationProof) {
	t.Helper()
	if got := string(proof.writer.options.namespace); got != proof.namespace {
		t.Fatalf("%s writer namespace=%q want=%q", proof.name, got, proof.namespace)
	}
	if got := proof.callbackCalls.Load(); got != proof.callbackSnapshot {
		t.Fatalf("%s callback calls changed after drain: before=%d after=%d",
			proof.name, proof.callbackSnapshot, got)
	}
	if got := proof.observation.snapshot(); got != proof.observationSnapshot {
		t.Fatalf("%s transaction observation changed after drain: before=%+v after=%+v",
			proof.name, proof.observationSnapshot, got)
	}
	if checkpoint, found, err := proof.writer.Load(ctx, proof.key); err != nil || found || checkpoint.Version != 0 {
		t.Fatalf("%s namespace=%q key=%q late checkpoint=%+v found=%v err=%v",
			proof.name, proof.namespace, proof.key, checkpoint, found, err)
	}
	for _, businessID := range proof.businessIDs {
		assertBusinessIDAbsent(ctx, t, db, businessID)
	}
}

type observedTransaction struct {
	transaction
	sqlTx       *sql.Tx
	observation *transactionObservation
}

func (tx *observedTransaction) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	switch query {
	case savepointSQL:
		tx.observation.savepoints.Add(1)
	case releaseSavepointSQL:
		tx.observation.releases.Add(1)
		if tx.observation.beforeRelease != nil {
			tx.observation.beforeRelease(tx)
		}
		result, err := tx.transaction.ExecContext(ctx, query, args...)
		if tx.observation.afterRelease != nil {
			tx.observation.afterRelease(tx)
		}
		return result, err
	case rollbackToSavepointSQL:
		tx.observation.rollbackTos.Add(1)
	}
	return tx.transaction.ExecContext(ctx, query, args...)
}

func (tx *observedTransaction) ScanRevision(ctx context.Context, query string, args ...any) (int64, error) {
	tx.observation.scans.Add(1)
	if tx.observation.beforeCAS != nil {
		tx.observation.beforeCAS(tx)
	}
	revision, err := tx.transaction.ScanRevision(ctx, query, args...)
	if tx.observation.afterCAS != nil {
		tx.observation.afterCAS(tx)
	}
	return revision, err
}

func (tx *observedTransaction) Commit() error {
	tx.observation.commits.Add(1)
	if tx.observation.beforeCommit != nil {
		tx.observation.beforeCommit(tx)
	}
	return tx.transaction.Commit()
}

func (tx *observedTransaction) Rollback() error {
	tx.observation.rollbacks.Add(1)
	return tx.transaction.Rollback()
}

func observeWriter[T, C any](writer *Writer[T, C], observation *transactionObservation) {
	writer.beginTx = func(ctx context.Context) (transaction, error) {
		observation.begins.Add(1)
		tx, err := beginCheckpointTransaction(ctx, writer.db)
		if err != nil {
			return nil, err
		}
		base := &sqlTransaction{tx: tx}
		return &observedTransaction{transaction: base, sqlTx: tx, observation: observation}, nil
	}
}

type commitOutcome struct {
	revision uint64
	err      error
}

func runObservedCommit(
	ctx context.Context,
	writer *Writer[string, string],
	key string,
	expected uint64,
	items []string,
	checkpoint string,
) <-chan commitOutcome {
	result := make(chan commitOutcome, 1)
	go func() {
		revision, err := writer.Commit(ctx, key, expected, items, checkpoint)
		result <- commitOutcome{revision: revision, err: err}
	}()
	return result
}

func observedPhaseHook(reached chan<- *observedTransaction, release <-chan struct{}) func(*observedTransaction) {
	return func(tx *observedTransaction) {
		select {
		case reached <- tx:
		default:
		}
		select {
		case <-release:
		case <-time.After(5 * time.Second):
		}
	}
}

func awaitObservedPhase(t *testing.T, reached <-chan *observedTransaction, name string) *observedTransaction {
	t.Helper()
	select {
	case tx := <-reached:
		return tx
	case <-time.After(5 * time.Second):
		t.Fatalf("%s phase was not reached", name)
		return nil
	}
}

func awaitCommitOutcome(t *testing.T, result <-chan commitOutcome) commitOutcome {
	t.Helper()
	select {
	case outcome := <-result:
		return outcome
	case <-time.After(5 * time.Second):
		t.Fatal("Commit did not return within bounded wait")
		return commitOutcome{}
	}
}

func newPostgresFixture(t *testing.T) *postgresFixture {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), postgresTestTimeout)
	t.Cleanup(cancel)
	dsn, db := startReadyPostgresPool(ctx, t, "PostgreSQL")
	t.Cleanup(func() { _ = db.Close() })
	for _, statement := range []string{
		SchemaSQL,
		`create table public.sqlcheckpoint_business (
			id text primary key,
			payload text not null
		)`,
	} {
		if _, err := db.ExecContext(ctx, statement); err != nil {
			t.Fatalf("fixture statement %q: %v", statement, err)
		}
	}
	return &postgresFixture{ctx: ctx, dsn: dsn, db: db}
}

func startReadyPostgresPool(ctx context.Context, t *testing.T, label string) (string, *sql.DB) {
	t.Helper()
	const maxStarts = 2
	var readinessErr error
	activeServers := 0
	for starts := 1; starts <= maxStarts; starts++ {
		if activeServers != 0 {
			t.Fatalf("%s retry would overlap %d active PostgreSQL server(s)", label, activeServers)
		}
		cleanupTB := &idempotentServerCleanupTB{TB: t}
		server := postgrestestcontainer.StartServer(ctx, cleanupTB)
		activeServers++
		terminateFailedAttempt := func() {
			terminateCtx, terminateCancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer terminateCancel()
			if err := server.Terminate(terminateCtx); err != nil {
				t.Fatalf("%s readiness failed and PostgreSQL termination failed: %v", label, err)
			}
			cleanupTB.terminated.Store(true)
			activeServers--
		}

		details, err := server.ConnectionDetails(ctx)
		if err != nil {
			terminateFailedAttempt()
			t.Fatalf("%s connection details: %v", label, err)
		}
		dsn, err := details.Require(postgrestestcontainer.ConnectionStringKey)
		if err != nil {
			terminateFailedAttempt()
			t.Fatalf("%s connection string: %v", label, err)
		}
		db, err := sql.Open("pgx", dsn)
		if err == nil {
			readinessErr = postgresReadiness(ctx, db, 15*time.Second)
		} else {
			readinessErr = err
		}
		if readinessErr == nil {
			if activeServers != 1 || starts > maxStarts {
				t.Fatalf("%s active servers=%d starts=%d; want one active server within two starts",
					label, activeServers, starts)
			}
			return dsn, db
		}
		if db != nil {
			if err := db.Close(); err != nil {
				terminateFailedAttempt()
				t.Fatalf("%s close failed readiness pool: %v", label, err)
			}
		}
		terminateFailedAttempt()
	}
	t.Fatalf("%s readiness after at most two sequential starts: %v", label, readinessErr)
	return "", nil
}

func openPostgresPool(ctx context.Context, t *testing.T, dsn string) *sql.DB {
	t.Helper()
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	waitPostgresReady(ctx, t, db, "PostgreSQL pool")
	return db
}

func openPostgresPoolWithDefaultIsolation(
	ctx context.Context,
	t *testing.T,
	dsn string,
	isolation string,
) *sql.DB {
	t.Helper()
	parsed, err := url.Parse(dsn)
	if err != nil {
		t.Fatal(err)
	}
	query := parsed.Query()
	query.Set("default_transaction_isolation", isolation)
	parsed.RawQuery = query.Encode()

	db := openPostgresPool(ctx, t, parsed.String())
	var got string
	if err := db.QueryRowContext(ctx, `show default_transaction_isolation`).Scan(&got); err != nil {
		t.Fatal(err)
	}
	if got != isolation {
		t.Fatalf("default_transaction_isolation=%q want=%q", got, isolation)
	}
	return db
}

func postgresStringCodec() Codec[string] {
	return Codec[string]{
		Encode: func(value string) ([]byte, error) { return []byte(value), nil },
		Decode: func(value []byte) (string, error) { return string(value), nil },
	}
}

func newPostgresWriter(
	t *testing.T,
	db *sql.DB,
	namespace string,
	write WriteTxFunc[string],
) *Writer[string, string] {
	t.Helper()
	if write == nil {
		write = func(ctx context.Context, session sqlkit.Session, items []string) error {
			for _, item := range items {
				if _, err := session.ExecContext(ctx,
					`insert into public.sqlcheckpoint_business(id,payload) values ($1,$2)`, item, "payload:"+item); err != nil {
					return err
				}
			}
			return nil
		}
	}
	writer, err := New(db, Options{Namespace: namespace}, postgresStringCodec(), write)
	if err != nil {
		t.Fatal(err)
	}
	return writer
}

func TestPostgresSuccessRestartRollbackAndIsolation(t *testing.T) {
	fixture := newPostgresFixture(t)
	writer := newPostgresWriter(t, fixture.db, "success", nil)

	if checkpoint, found, err := writer.Load(fixture.ctx, "job"); err != nil || found || checkpoint.Version != 0 {
		t.Fatalf("initial Load = %+v, %v, %v; want missing", checkpoint, found, err)
	}
	revision, err := writer.Commit(fixture.ctx, "job", 0, []string{"business-1"}, "checkpoint-1")
	if err != nil || revision != 1 {
		t.Fatalf("missing Commit = %d, %v; want revision 1", revision, err)
	}
	revision, err = writer.Commit(fixture.ctx, "job", 1, []string{"business-2"}, "checkpoint-2")
	if err != nil || revision != 2 {
		t.Fatalf("exact Commit = %d, %v; want revision 2", revision, err)
	}

	restarted := newPostgresWriter(t, fixture.db, "success", nil)
	checkpoint, found, err := restarted.Load(fixture.ctx, "job")
	if err != nil || !found || checkpoint.Version != 2 || checkpoint.Value != "checkpoint-2" {
		t.Fatalf("restart Load = %+v, %v, %v", checkpoint, found, err)
	}
	assertBusinessRows(fixture.ctx, t, fixture.db, 2)

	callbackCause := errors.New("known callback rollback")
	failing := newPostgresWriter(t, fixture.db, "rollback", func(ctx context.Context, session sqlkit.Session, items []string) error {
		if _, err := session.ExecContext(ctx,
			`insert into public.sqlcheckpoint_business(id,payload) values ($1,$2)`, items[0], "rollback"); err != nil {
			return err
		}
		return callbackCause
	})
	if revision, err = failing.Commit(fixture.ctx, "job", 0, []string{"callback-rollback"}, "checkpoint"); revision != 0 || !errors.Is(err, callbackCause) || errors.Is(err, batch.ErrCommitUnknown) {
		t.Fatalf("callback failure = %d, %v; want known rollback", revision, err)
	}
	assertBusinessIDAbsent(fixture.ctx, t, fixture.db, "callback-rollback")
	if _, found, err := failing.Load(fixture.ctx, "job"); err != nil || found {
		t.Fatalf("rollback Load found=%v err=%v", found, err)
	}

	replay := newPostgresWriter(t, fixture.db, "rollback", nil)
	if revision, err = replay.Commit(fixture.ctx, "job", 0, []string{"callback-rollback"}, "replayed"); err != nil || revision != 1 {
		t.Fatalf("known rollback replay = %d, %v", revision, err)
	}

	checkpointFailure := newPostgresWriter(t, fixture.db, "checkpoint-failure", func(ctx context.Context, session sqlkit.Session, items []string) error {
		if _, err := session.ExecContext(ctx,
			`insert into public.sqlcheckpoint_business(id,payload) values ($1,$2)`, items[0], "rollback"); err != nil {
			return err
		}
		_, err := session.ExecContext(ctx,
			`alter table public.bluetape_batch_checkpoints rename to bluetape_batch_checkpoints_hidden`)
		return err
	})
	if revision, err = checkpointFailure.Commit(fixture.ctx, "job", 0, []string{"checkpoint-rollback"}, "checkpoint"); revision != 0 || err == nil || errors.Is(err, batch.ErrCommitUnknown) {
		t.Fatalf("checkpoint failure = %d, %v; want known rollback", revision, err)
	}
	assertBusinessIDAbsent(fixture.ctx, t, fixture.db, "checkpoint-rollback")
	if _, err := fixture.db.ExecContext(fixture.ctx, `select 1 from public.bluetape_batch_checkpoints limit 1`); err != nil {
		t.Fatalf("checkpoint relation rename was not rolled back: %v", err)
	}

	identities := []struct {
		namespace string
		key       string
		value     string
	}{
		{namespace: "tenant\x00a", key: "key\x00one", value: "nul"},
		{namespace: string([]byte{'t', 0xff, 'a'}), key: string([]byte{'k', 0xfe, 'y'}), value: "invalid-utf8"},
	}
	for _, identity := range identities {
		isolated := newPostgresWriter(t, fixture.db, identity.namespace, nil)
		if revision, err = isolated.Commit(fixture.ctx, identity.key, 0, nil, identity.value); err != nil || revision != 1 {
			t.Fatalf("isolated Commit(%q) = %d, %v", identity.value, revision, err)
		}
		loaded, found, err := isolated.Load(fixture.ctx, identity.key)
		if err != nil || !found || loaded.Value != identity.value || loaded.Version != 1 {
			t.Fatalf("isolated Load(%q) = %+v, %v, %v", identity.value, loaded, found, err)
		}
	}

	callbackCalls := 0
	checkpointOnly := newPostgresWriter(t, fixture.db, "checkpoint-only", func(context.Context, sqlkit.Session, []string) error {
		callbackCalls++
		return nil
	})
	if revision, err = checkpointOnly.Commit(fixture.ctx, "job", 0, nil, "only"); err != nil || revision != 1 || callbackCalls != 0 {
		t.Fatalf("checkpoint-only Commit = %d, %v callback=%d", revision, err, callbackCalls)
	}
	if err := fixture.db.PingContext(fixture.ctx); err != nil {
		t.Fatalf("writer closed caller pool: %v", err)
	}
}

func TestPostgresRawTransactionControlAndPanics(t *testing.T) {
	fixture := newPostgresFixture(t)
	tests := []struct {
		name             string
		callback         func(context.Context, sqlkit.Session) error
		wantViolation    bool
		wantCancellation bool
	}{
		{
			name: "raw-commit",
			callback: func(ctx context.Context, session sqlkit.Session) error {
				_, err := session.ExecContext(ctx, `commit`)
				return err
			},
			wantViolation: true,
		},
		{
			name: "raw-rollback",
			callback: func(ctx context.Context, session sqlkit.Session) error {
				_, err := session.ExecContext(ctx, `rollback`)
				return err
			},
			wantViolation: true,
		},
		{
			name: "commit-begin-failed-statement",
			callback: func(ctx context.Context, session sqlkit.Session) error {
				if _, err := session.ExecContext(ctx, `commit`); err != nil {
					return err
				}
				if _, err := session.ExecContext(ctx, `begin`); err != nil {
					return err
				}
				_, err := session.ExecContext(ctx, `select 1/0`)
				return err
			},
			wantViolation: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := openPostgresPool(fixture.ctx, t, fixture.dsn)
			observation := new(transactionObservation)
			writer := newPostgresWriter(t, db, "raw-"+tt.name, func(ctx context.Context, session sqlkit.Session, items []string) error {
				if _, err := session.ExecContext(ctx,
					`insert into public.sqlcheckpoint_business(id,payload) values ($1,'raw')`, items[0]); err != nil {
					return err
				}
				return tt.callback(ctx, session)
			})
			observeWriter(writer, observation)
			revision, err := writer.Commit(fixture.ctx, "job", 0, []string{tt.name}, "checkpoint")
			if revision != 0 || !errors.Is(err, batch.ErrAtomicityUnknown) || !errors.Is(err, batch.ErrCommitUnknown) {
				t.Fatalf("raw control Commit = %d, %v; want atomicity unknown", revision, err)
			}
			if got := errors.Is(err, ErrCallbackContractViolation); got != tt.wantViolation {
				t.Fatalf("contract violation=%v want=%v: %v", got, tt.wantViolation, err)
			}
			if _, found, loadErr := writer.Load(fixture.ctx, "job"); loadErr != nil || found {
				t.Fatalf("raw control dispatched checkpoint: found=%v err=%v", found, loadErr)
			}
			assertNoCheckpointOrProviderCommit(t, observation)
		})
	}

	t.Run("raw-commit-cancellation", func(t *testing.T) {
		db := openPostgresPool(fixture.ctx, t, fixture.dsn)
		ctx, cancel := context.WithCancel(fixture.ctx)
		observation := new(transactionObservation)
		writer := newPostgresWriter(t, db, "raw-cancel", func(ctx context.Context, session sqlkit.Session, _ []string) error {
			if _, err := session.ExecContext(ctx, `commit`); err != nil {
				return err
			}
			cancel()
			return ctx.Err()
		})
		observeWriter(writer, observation)
		revision, err := writer.Commit(ctx, "job", 0, []string{"raw-cancel"}, "checkpoint")
		if revision != 0 || !errors.Is(err, batch.ErrAtomicityUnknown) || !errors.Is(err, context.Canceled) {
			t.Fatalf("raw commit cancellation = %d, %v", revision, err)
		}
		if errors.Is(err, ErrCallbackContractViolation) {
			code := postgresCode(err)
			if code != "25P01" && code != "3B001" {
				t.Fatalf("contract violation lacked positive lifecycle evidence code=%q: %v", code, err)
			}
		}
		assertNoCheckpointOrProviderCommit(t, observation)
		if _, found, loadErr := writer.Load(fixture.ctx, "job"); loadErr != nil || found {
			t.Fatalf("raw cancellation dispatched checkpoint: found=%v err=%v", found, loadErr)
		}
	})

	t.Run("normal-panic", func(t *testing.T) {
		db := openPostgresPool(fixture.ctx, t, fixture.dsn)
		panicValue := &struct{ marker string }{marker: "original"}
		observation := new(transactionObservation)
		writer := newPostgresWriter(t, db, "normal-panic", func(ctx context.Context, session sqlkit.Session, items []string) error {
			if _, err := session.ExecContext(ctx,
				`insert into public.sqlcheckpoint_business(id,payload) values ($1,'panic')`, items[0]); err != nil {
				return err
			}
			panic(panicValue)
		})
		observeWriter(writer, observation)
		returned, recovered := capturePanic(func() {
			_, _ = writer.Commit(fixture.ctx, "job", 0, []string{"normal-panic"}, "checkpoint")
		})
		if returned || recovered != panicValue {
			t.Fatalf("normal panic = %#v, want original %#v", recovered, panicValue)
		}
		assertBusinessIDAbsent(fixture.ctx, t, fixture.db, "normal-panic")
		if _, found, err := writer.Load(fixture.ctx, "job"); err != nil || found {
			t.Fatalf("normal panic checkpoint found=%v err=%v", found, err)
		}
		assertNoCheckpointOrProviderCommit(t, observation)
	})

	t.Run("raw-commit-panic-redaction", func(t *testing.T) {
		db := openPostgresPool(fixture.ctx, t, fixture.dsn)
		secret := errors.New("panic-secret postgres://user:dsn-secret@host/database")
		observation := new(transactionObservation)
		writer := newPostgresWriter(t, db, "raw-panic", func(ctx context.Context, session sqlkit.Session, _ []string) error {
			if _, err := session.ExecContext(ctx, `commit`); err != nil {
				return err
			}
			panic(secret)
		})
		observeWriter(writer, observation)
		returned, recovered := capturePanic(func() {
			_, _ = writer.Commit(fixture.ctx, "job", 0, []string{"raw-panic"}, "checkpoint")
		})
		if returned {
			t.Fatal("raw Commit panic path returned")
		}
		atomicityPanic, ok := recovered.(*AtomicityPanic)
		if !ok {
			t.Fatalf("raw panic = %T %#v, want *AtomicityPanic", recovered, recovered)
		}
		panicErr, panicIsError := atomicityPanic.PanicValue().(error)
		if !panicIsError || !errors.Is(panicErr, secret) || !errors.Is(atomicityPanic, batch.ErrAtomicityUnknown) ||
			!errors.Is(atomicityPanic, batch.ErrCommitUnknown) {
			t.Fatalf("AtomicityPanic contract = %#v", atomicityPanic)
		}
		for _, rendered := range []string{
			atomicityPanic.Error(),
			fmt.Sprintf("%v", atomicityPanic),
			fmt.Sprintf("%+v", atomicityPanic),
			fmt.Sprint(errors.Unwrap(atomicityPanic)),
		} {
			if strings.Contains(rendered, "panic-secret") || strings.Contains(rendered, "dsn-secret") || strings.Contains(rendered, "postgres://") {
				t.Fatalf("AtomicityPanic leaked secret in %q", rendered)
			}
		}
		assertNoCheckpointOrProviderCommit(t, observation)
		if _, found, loadErr := writer.Load(fixture.ctx, "job"); loadErr != nil || found {
			t.Fatalf("raw panic dispatched checkpoint: found=%v err=%v", found, loadErr)
		}
	})
}

func assertNoCheckpointOrProviderCommit(t *testing.T, observation *transactionObservation) {
	t.Helper()
	if scans, commits := observation.scans.Load(), observation.commits.Load(); scans != 0 || commits != 0 {
		t.Fatalf("checkpoint/provider Commit calls=%d/%d; want 0/0", scans, commits)
	}
}

func TestPostgresSmallPoolReleaseAfterKnownCallbackFailure(t *testing.T) {
	fixture := newPostgresFixture(t)
	fixture.db.SetMaxOpenConns(1)
	fixture.db.SetMaxIdleConns(1)
	proofs := make([]cancellationProof, 0, 6)

	t.Run("pre-begin", func(t *testing.T) {
		const namespace, key, businessID = "pre-begin", "job", "pre"
		var callbackCalls atomic.Int64
		observation := new(transactionObservation)
		writer := newPostgresWriter(t, fixture.db, namespace, func(context.Context, sqlkit.Session, []string) error {
			callbackCalls.Add(1)
			return nil
		})
		observeWriter(writer, observation)
		preCanceled, cancel := context.WithCancel(fixture.ctx)
		cancel()
		if revision, err := writer.Commit(preCanceled, key, 0, []string{businessID}, "checkpoint"); revision != 0 || !errors.Is(err, context.Canceled) {
			t.Fatalf("pre-Begin cancellation = %d, %v", revision, err)
		}
		if callbackCalls.Load() != 0 || observation.begins.Load() != 0 || observation.scans.Load() != 0 || observation.commits.Load() != 0 {
			t.Fatalf("pre-Begin calls callback/begin/CAS/Commit=%d/%d/%d/%d; want zero",
				callbackCalls.Load(), observation.begins.Load(), observation.scans.Load(), observation.commits.Load())
		}
		assertBusinessIDAbsent(fixture.ctx, t, fixture.db, businessID)
		proofs = append(proofs, retainCancellationProof("pre-begin", namespace, key,
			[]string{businessID}, writer, &callbackCalls, observation))
	})

	t.Run("callback-barrier-known-rollback", func(t *testing.T) {
		const namespace, key, businessID = "callback-cancel", "job", "callback-cancel"
		callbackReached := make(chan struct{}, 1)
		callbackRelease := make(chan struct{})
		var callbackCalls atomic.Int64
		observation := new(transactionObservation)
		writer := newPostgresWriter(t, fixture.db, namespace, func(ctx context.Context, session sqlkit.Session, items []string) error {
			callbackCalls.Add(1)
			if _, err := session.ExecContext(ctx,
				`insert into public.sqlcheckpoint_business(id,payload) values ($1,'cancel')`, items[0]); err != nil {
				return err
			}
			callbackReached <- struct{}{}
			select {
			case <-callbackRelease:
				return context.Canceled
			case <-time.After(5 * time.Second):
				return errors.New("callback barrier timeout")
			}
		})
		observeWriter(writer, observation)
		result := runObservedCommit(fixture.ctx, writer, key, 0, []string{businessID}, "checkpoint")
		select {
		case <-callbackReached:
		case <-time.After(5 * time.Second):
			t.Fatal("callback cancellation barrier was not reached")
		}
		close(callbackRelease)
		outcome := awaitCommitOutcome(t, result)
		if outcome.revision != 0 || !errors.Is(outcome.err, context.Canceled) || errors.Is(outcome.err, batch.ErrCommitUnknown) {
			t.Fatalf("callback cancellation = %d, %v; want known rollback", outcome.revision, outcome.err)
		}
		if observation.begins.Load() != 1 || observation.releases.Load() != 1 || observation.scans.Load() != 0 ||
			observation.commits.Load() != 0 || observation.rollbacks.Load() != 1 {
			t.Fatalf("callback phase begin/release/CAS/Commit/rollback=%d/%d/%d/%d/%d",
				observation.begins.Load(), observation.releases.Load(), observation.scans.Load(),
				observation.commits.Load(), observation.rollbacks.Load())
		}
		assertBusinessIDAbsent(fixture.ctx, t, fixture.db, businessID)
		proofs = append(proofs, retainCancellationProof("callback-barrier-known-rollback", namespace, key,
			[]string{businessID}, writer, &callbackCalls, observation))
	})

	t.Run("callback-context-cancellation-auto-rollback", func(t *testing.T) {
		const namespace, key, businessID = "callback-context-cancel", "job", "callback-context-cancel"
		ctx, cancel := context.WithCancel(fixture.ctx)
		callbackReached := make(chan *observedTransaction, 1)
		callbackRelease := make(chan struct{})
		var callbackCalls atomic.Int64
		observation := new(transactionObservation)
		writer := newPostgresWriter(t, fixture.db, namespace, func(ctx context.Context, session sqlkit.Session, items []string) error {
			callbackCalls.Add(1)
			if _, err := session.ExecContext(ctx,
				`insert into public.sqlcheckpoint_business(id,payload) values ($1,'cancel')`, items[0]); err != nil {
				return err
			}
			guarded, ok := session.(guardedSession)
			if !ok {
				return errors.New("callback session is not guarded")
			}
			observed, ok := guarded.session.(*observedTransaction)
			if !ok {
				return errors.New("callback session is not observed")
			}
			callbackReached <- observed
			select {
			case <-callbackRelease:
				return ctx.Err()
			case <-time.After(5 * time.Second):
				return errors.New("callback cancellation barrier timeout")
			}
		})
		observeWriter(writer, observation)
		result := runObservedCommit(ctx, writer, key, 0, []string{businessID}, "checkpoint")
		tx := awaitObservedPhase(t, callbackReached, "callback cancellation")
		cancel()
		_ = tx.sqlTx.Rollback()
		close(callbackRelease)
		outcome := awaitCommitOutcome(t, result)
		if outcome.revision != 0 || !errors.Is(outcome.err, context.Canceled) || !errors.Is(outcome.err, sql.ErrTxDone) ||
			!errors.Is(outcome.err, batch.ErrAtomicityUnknown) || errors.Is(outcome.err, ErrCallbackContractViolation) {
			t.Fatalf("callback context cancellation = %d, %v; want canceled TxDone atomicity unknown", outcome.revision, outcome.err)
		}
		assertNoCheckpointOrProviderCommit(t, observation)
		assertBusinessIDAbsent(fixture.ctx, t, fixture.db, businessID)
		proofs = append(proofs, retainCancellationProof("callback-context-cancellation-auto-rollback", namespace, key,
			[]string{businessID}, writer, &callbackCalls, observation))
	})

	t.Run("before-checkpoint-dml", func(t *testing.T) {
		const namespace, key, businessID = "before-cas", "job", "before-cas"
		ctx, cancel := context.WithCancel(fixture.ctx)
		reached := make(chan *observedTransaction, 1)
		release := make(chan struct{})
		var callbackCalls atomic.Int64
		observation := &transactionObservation{afterRelease: observedPhaseHook(reached, release)}
		writer := newPostgresWriter(t, fixture.db, namespace, countedPostgresWrite(&callbackCalls))
		observeWriter(writer, observation)
		result := runObservedCommit(ctx, writer, key, 0, []string{businessID}, "checkpoint")
		_ = awaitObservedPhase(t, reached, "post-ownership/pre-CAS")
		cancel()
		close(release)
		outcome := awaitCommitOutcome(t, result)
		if outcome.revision != 0 || !errors.Is(outcome.err, context.Canceled) || errors.Is(outcome.err, batch.ErrCommitUnknown) {
			t.Fatalf("pre-CAS cancellation = %d, %v; want known rollback", outcome.revision, outcome.err)
		}
		if observation.scans.Load() != 0 || observation.commits.Load() != 0 || observation.rollbacks.Load() != 1 {
			t.Fatalf("pre-CAS calls CAS/Commit/rollback=%d/%d/%d; want 0/0/1",
				observation.scans.Load(), observation.commits.Load(), observation.rollbacks.Load())
		}
		assertBusinessIDAbsent(fixture.ctx, t, fixture.db, businessID)
		proofs = append(proofs, retainCancellationProof("before-checkpoint-dml", namespace, key,
			[]string{businessID}, writer, &callbackCalls, observation))
	})

	t.Run("ownership-probe-canceled-context-tx-done", func(t *testing.T) {
		const namespace, key, businessID = "probe-cancel", "job", "probe-cancel"
		ctx, cancel := context.WithCancel(fixture.ctx)
		reached := make(chan *observedTransaction, 1)
		release := make(chan struct{})
		var callbackCalls atomic.Int64
		observation := &transactionObservation{beforeRelease: observedPhaseHook(reached, release)}
		writer := newPostgresWriter(t, fixture.db, namespace, countedPostgresWrite(&callbackCalls))
		observeWriter(writer, observation)
		result := runObservedCommit(ctx, writer, key, 0, []string{businessID}, "checkpoint")
		tx := awaitObservedPhase(t, reached, "ownership probe")
		cancel()
		_ = tx.sqlTx.Rollback()
		close(release)
		outcome := awaitCommitOutcome(t, result)
		if outcome.revision != 0 || !errors.Is(outcome.err, context.Canceled) || !errors.Is(outcome.err, sql.ErrTxDone) ||
			!errors.Is(outcome.err, batch.ErrAtomicityUnknown) || errors.Is(outcome.err, ErrCallbackContractViolation) {
			t.Fatalf("canceled-context sql.ErrTxDone = %d, %v; want atomicity unknown without guessed violation", outcome.revision, outcome.err)
		}
		assertNoCheckpointOrProviderCommit(t, observation)
		assertBusinessIDAbsent(fixture.ctx, t, fixture.db, businessID)
		proofs = append(proofs, retainCancellationProof("ownership-probe-canceled-context-tx-done", namespace, key,
			[]string{businessID}, writer, &callbackCalls, observation))
	})

	t.Run("post-cas-pre-commit", func(t *testing.T) {
		const namespace, key, businessID = "post-cas", "job", "post-cas"
		ctx, cancel := context.WithCancel(fixture.ctx)
		reached := make(chan *observedTransaction, 1)
		release := make(chan struct{})
		var callbackCalls atomic.Int64
		observation := &transactionObservation{afterCAS: observedPhaseHook(reached, release)}
		writer := newPostgresWriter(t, fixture.db, namespace, countedPostgresWrite(&callbackCalls))
		observeWriter(writer, observation)
		result := runObservedCommit(ctx, writer, key, 0, []string{businessID}, "checkpoint")
		_ = awaitObservedPhase(t, reached, "post-CAS")
		cancel()
		close(release)
		outcome := awaitCommitOutcome(t, result)
		if outcome.revision != 0 || !errors.Is(outcome.err, context.Canceled) || errors.Is(outcome.err, batch.ErrCommitUnknown) {
			t.Fatalf("post-CAS cancellation = %d, %v; want known rollback", outcome.revision, outcome.err)
		}
		if observation.scans.Load() != 1 || observation.commits.Load() != 0 || observation.rollbacks.Load() != 1 {
			t.Fatalf("post-CAS calls CAS/Commit/rollback=%d/%d/%d; want 1/0/1",
				observation.scans.Load(), observation.commits.Load(), observation.rollbacks.Load())
		}
		assertBusinessIDAbsent(fixture.ctx, t, fixture.db, businessID)
		if _, found, err := writer.Load(fixture.ctx, key); err != nil || found {
			t.Fatalf("post-CAS cancellation left checkpoint found=%v err=%v", found, err)
		}
		proofs = append(proofs, retainCancellationProof("post-cas-pre-commit", namespace, key,
			[]string{businessID}, writer, &callbackCalls, observation))
	})

	reuseCtx, reuseCancel := context.WithTimeout(fixture.ctx, 1500*time.Millisecond)
	defer reuseCancel()
	if err := fixture.db.PingContext(reuseCtx); err != nil {
		t.Fatalf("single connection was not reusable within 1s + 500ms: %v", err)
	}
	deadline := time.Now().Add(1500 * time.Millisecond)
	for fixture.db.Stats().InUse != 0 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if stats := fixture.db.Stats(); stats.InUse != 0 {
		t.Fatalf("caller pool did not drain: %+v", stats)
	}
	if len(proofs) != 6 {
		t.Fatalf("retained cancellation proofs=%d want=6", len(proofs))
	}
	for _, proof := range proofs {
		assertCancellationProofStable(fixture.ctx, t, fixture.db, proof)
	}
}

func TestPostgresCommitDispatchUnknownAndFreshLoadReconciles(t *testing.T) {
	fixture := newPostgresFixture(t)
	for _, statement := range []string{
		`create function public.sqlcheckpoint_slow_commit() returns trigger language plpgsql as $$
		begin perform pg_catalog.pg_sleep(5); return new; end $$`,
		`create constraint trigger sqlcheckpoint_slow_commit
		after insert on public.bluetape_batch_checkpoints
		deferrable initially deferred for each row execute function public.sqlcheckpoint_slow_commit()`,
	} {
		if _, err := fixture.db.ExecContext(fixture.ctx, statement); err != nil {
			t.Fatal(err)
		}
	}
	ctx, cancel := context.WithTimeout(fixture.ctx, 300*time.Millisecond)
	defer cancel()
	writer := newPostgresWriter(t, fixture.db, "commit-cancel", nil)
	observation := new(transactionObservation)
	observeWriter(writer, observation)
	revision, err := writer.Commit(ctx, "job", 0, []string{"commit-cancel"}, "checkpoint")
	if revision != 0 || !errors.Is(err, batch.ErrCommitUnknown) || errors.Is(err, batch.ErrAtomicityUnknown) {
		t.Fatalf("post-Commit-dispatch cancellation = %d, %v; want commit unknown", revision, err)
	}
	if observation.scans.Load() != 1 || observation.commits.Load() != 1 {
		t.Fatalf("commit-unknown CAS/provider Commit calls=%d/%d; want 1/1",
			observation.scans.Load(), observation.commits.Load())
	}
	fresh := openPostgresPool(fixture.ctx, t, fixture.dsn)
	restarted := newPostgresWriter(t, fresh, "commit-cancel", nil)
	checkpoint, found, loadErr := restarted.Load(fixture.ctx, "job")
	if loadErr != nil {
		t.Fatalf("fresh Load after commit unknown: %v", loadErr)
	}
	var businessRows int
	if err := fresh.QueryRowContext(fixture.ctx,
		`select count(*) from public.sqlcheckpoint_business where id='commit-cancel'`).Scan(&businessRows); err != nil {
		t.Fatal(err)
	}
	if found {
		if businessRows != 1 || checkpoint.Version != 1 || checkpoint.Value != "checkpoint" {
			t.Fatalf("committed reconciliation checkpoint=%+v businessRows=%d; want revision 1/value/checkpoint and one row",
				checkpoint, businessRows)
		}
	} else if businessRows != 0 || checkpoint != (batch.VersionedCheckpoint{}) {
		t.Fatalf("rolled-back reconciliation checkpoint=%+v found=%v businessRows=%d; want missing and zero rows",
			checkpoint, found, businessRows)
	}
}

func TestPostgresAtomicityUnknownCompetingActorAttribution(t *testing.T) {
	fixture := newPostgresFixture(t)
	originalObservation := new(transactionObservation)
	original := newPostgresWriter(t, fixture.db, "competing-actor", func(ctx context.Context, session sqlkit.Session, items []string) error {
		if _, err := session.ExecContext(ctx,
			`insert into public.sqlcheckpoint_business(id,payload) values ($1,'original-ambiguous')`, items[0]); err != nil {
			return err
		}
		_, err := session.ExecContext(ctx, `commit`)
		return err
	})
	observeWriter(original, originalObservation)
	revision, err := original.Commit(fixture.ctx, "job", 0, []string{"ambiguous-original"}, "original-checkpoint")
	if revision != 0 || !errors.Is(err, batch.ErrAtomicityUnknown) {
		t.Fatalf("original raw-COMMIT attempt = %d, %v; want atomicity unknown", revision, err)
	}
	assertNoCheckpointOrProviderCommit(t, originalObservation)

	competitorPool := openPostgresPool(fixture.ctx, t, fixture.dsn)
	competitor := newPostgresWriter(t, competitorPool, "competing-actor", nil)
	if revision, err = competitor.Commit(fixture.ctx, "job", 0, []string{"authoritative-competitor"}, "competitor-checkpoint"); err != nil || revision != 1 {
		t.Fatalf("competitor Commit = %d, %v; want revision 1", revision, err)
	}
	checkpoint, found, err := competitor.Load(fixture.ctx, "job")
	if err != nil || !found || checkpoint.Version != 1 || checkpoint.Value != "competitor-checkpoint" {
		t.Fatalf("authoritative Load = %+v, %v, %v; want competitor revision/value", checkpoint, found, err)
	}
	for _, id := range []string{"ambiguous-original", "authoritative-competitor"} {
		var count int
		if err := competitorPool.QueryRowContext(fixture.ctx,
			`select count(*) from public.sqlcheckpoint_business where id=$1`, id).Scan(&count); err != nil {
			t.Fatal(err)
		}
		if count != 1 {
			t.Fatalf("business evidence %q count=%d; want 1", id, count)
		}
	}
	// The original raw COMMIT has independent business evidence, while Load
	// exposes only the competitor's current authoritative checkpoint. It cannot
	// attribute the original ambiguous attempt once same-key exclusivity is lost.
}

func assertBusinessRows(ctx context.Context, t *testing.T, db *sql.DB, want int) {
	t.Helper()
	var got int
	if err := db.QueryRowContext(ctx, `select count(*) from public.sqlcheckpoint_business`).Scan(&got); err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("business rows = %d, want %d", got, want)
	}
}

func waitPostgresReady(ctx context.Context, t *testing.T, db *sql.DB, label string) {
	t.Helper()
	if err := postgresReadiness(ctx, db, 15*time.Second); err != nil {
		t.Fatalf("%s readiness: %v", label, err)
	}
}

func postgresReadiness(ctx context.Context, db *sql.DB, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	var lastErr error
	for time.Now().Before(deadline) {
		pingCtx, cancel := context.WithTimeout(ctx, time.Second)
		lastErr = db.PingContext(pingCtx)
		cancel()
		if lastErr == nil {
			return nil
		}
		time.Sleep(50 * time.Millisecond)
	}
	return lastErr
}

func assertBusinessIDAbsent(ctx context.Context, t *testing.T, db *sql.DB, id string) {
	t.Helper()
	var count int
	if err := db.QueryRowContext(ctx,
		`select count(*) from public.sqlcheckpoint_business where id=$1`, id).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("business row %q survived rollback", id)
	}
}
