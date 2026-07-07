package sqloutbox

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/bluetape4k/bluetape-go/audit"
	postgrestestcontainer "github.com/bluetape4k/bluetape-go/testcontainers/postgres"
	concurrencytest "github.com/bluetape4k/bluetape-go/testing/concurrency"
	_ "github.com/jackc/pgx/v5/stdlib"
)

func TestStoreEnqueueClaimAndMarkPublished(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	t.Cleanup(cancel)

	db := openPostgresDB(ctx, t)
	store := newTestStore(ctx, t, db)

	entries := []audit.Entry{
		mustEntry(t, 1, "event-1", "idem-1"),
		mustEntry(t, 2, "event-2", "idem-2"),
	}
	if err := store.Enqueue(ctx, db, entries...); err != nil {
		t.Fatalf("Enqueue: %v", err)
	}

	records, err := store.Claim(ctx, db, ClaimOptions{Limit: 10, Now: testClock.Add(time.Minute)})
	if err != nil {
		t.Fatalf("Claim: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("Claim returned %d records, want 1", len(records))
	}

	if records[0].Entry.Event.EventID != "event-1" {
		t.Fatalf("claimed event ID = %q, want event-1", records[0].Entry.Event.EventID)
	}
	for _, record := range records {
		if record.Status != StatusClaimed {
			t.Fatalf("claimed record status = %q, want %q", record.Status, StatusClaimed)
		}
		if record.Aggregate != record.Entry.Aggregate || record.Revision != record.Entry.Revision {
			t.Fatalf("record metadata does not match entry: %#v", record)
		}
	}

	if err := store.MarkPublished(ctx, db, records[0]); err != nil {
		t.Fatalf("MarkPublished: %v", err)
	}
	records, err = store.Claim(ctx, db, ClaimOptions{Limit: 10, Now: testClock.Add(2 * time.Minute)})
	if err != nil {
		t.Fatalf("Claim second revision: %v", err)
	}
	if len(records) != 1 || records[0].Entry.Event.EventID != "event-2" {
		t.Fatalf("second claim = %#v, want event-2", records)
	}
	if err := store.MarkPublished(ctx, db, records[0]); err != nil {
		t.Fatalf("MarkPublished second revision: %v", err)
	}
	if got := countStatus(ctx, t, db, StatusPublished); got != 2 {
		t.Fatalf("published count = %d, want 2", got)
	}
}

func TestStoreRejectsDuplicateEventAndIdempotency(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	t.Cleanup(cancel)

	db := openPostgresDB(ctx, t)
	store := newTestStore(ctx, t, db)

	first := mustEntry(t, 1, "event-duplicate", "idem-duplicate")
	if err := store.Enqueue(ctx, db, first); err != nil {
		t.Fatalf("Enqueue first: %v", err)
	}
	if err := store.Enqueue(ctx, db, first); err == nil {
		t.Fatal("Enqueue duplicate event and idempotency key succeeded")
	}

	second := mustEntry(t, 2, "event-second", "idem-duplicate")
	if err := store.Enqueue(ctx, db, second); err == nil {
		t.Fatal("Enqueue duplicate idempotency key succeeded")
	}
}

func TestStoreMarkFailedRetriesAndDeadLetters(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	t.Cleanup(cancel)

	db := openPostgresDB(ctx, t)
	store := newTestStore(ctx, t, db)

	if err := store.Enqueue(ctx, db, mustEntry(t, 1, "event-poison", "idem-poison")); err != nil {
		t.Fatalf("Enqueue: %v", err)
	}

	claimed := claimOne(ctx, t, store, db, testClock.Add(time.Minute))
	retryAt := testClock.Add(2 * time.Minute)
	if err := store.MarkFailed(ctx, db, Failure{
		ID:          claimed.ID,
		Attempt:     claimed.Attempts,
		Err:         errors.New("publisher rejected payload"),
		RetryAt:     retryAt,
		MaxAttempts: 2,
		Now:         testClock.Add(time.Minute),
	}); err != nil {
		t.Fatalf("MarkFailed retry: %v", err)
	}
	if got := countStatus(ctx, t, db, StatusPending); got != 1 {
		t.Fatalf("pending count after retry = %d, want 1", got)
	}
	if records, err := store.Claim(ctx, db, ClaimOptions{Limit: 1, Now: retryAt.Add(-time.Nanosecond)}); err != nil {
		t.Fatalf("Claim before retry: %v", err)
	} else if len(records) != 0 {
		t.Fatalf("Claim before retry returned %d records, want 0", len(records))
	}

	claimed = claimOne(ctx, t, store, db, retryAt)
	if err := store.MarkFailed(ctx, db, Failure{
		ID:          claimed.ID,
		Attempt:     claimed.Attempts,
		Err:         errors.New("publisher rejected payload again"),
		RetryAt:     retryAt.Add(time.Minute),
		MaxAttempts: 2,
		Now:         retryAt,
	}); err != nil {
		t.Fatalf("MarkFailed dead-letter: %v", err)
	}
	if got := countStatus(ctx, t, db, StatusDeadLetter); got != 1 {
		t.Fatalf("dead-letter count = %d, want 1", got)
	}
}

func TestStoreReclaimsExpiredClaims(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	t.Cleanup(cancel)

	db := openPostgresDB(ctx, t)
	store := newTestStore(ctx, t, db)

	if err := store.Enqueue(ctx, db, mustEntry(t, 1, "event-expired-claim", "idem-expired-claim")); err != nil {
		t.Fatalf("Enqueue: %v", err)
	}

	claimAt := testClock.Add(time.Second)
	claimed := claimOneWithOptions(ctx, t, store, db, ClaimOptions{
		Limit:         1,
		Now:           claimAt,
		LeaseDuration: time.Minute,
	})
	if claimed.Attempts != 1 {
		t.Fatalf("initial attempts = %d, want 1", claimed.Attempts)
	}

	records, err := store.Claim(ctx, db, ClaimOptions{
		Limit:         1,
		Now:           claimAt.Add(time.Minute - time.Nanosecond),
		LeaseDuration: time.Minute,
	})
	if err != nil {
		t.Fatalf("Claim before lease expiry: %v", err)
	}
	if len(records) != 0 {
		t.Fatalf("Claim before lease expiry returned %d records, want 0", len(records))
	}

	reclaimed := claimOneWithOptions(ctx, t, store, db, ClaimOptions{
		Limit:         1,
		Now:           claimAt.Add(time.Minute),
		LeaseDuration: time.Minute,
	})
	if reclaimed.ID != claimed.ID {
		t.Fatalf("reclaimed ID = %d, want %d", reclaimed.ID, claimed.ID)
	}
	if reclaimed.Attempts != 2 {
		t.Fatalf("reclaimed attempts = %d, want 2", reclaimed.Attempts)
	}
	if err := store.MarkPublished(ctx, db, claimed); !errors.Is(err, ErrRecordNotFound) {
		t.Fatalf("MarkPublished stale claim error = %v, want ErrRecordNotFound", err)
	}
	if err := store.MarkFailed(ctx, db, Failure{
		ID:          claimed.ID,
		Attempt:     claimed.Attempts,
		Err:         errors.New("stale publisher failed"),
		RetryAt:     claimAt.Add(2 * time.Minute),
		MaxAttempts: 3,
		Now:         claimAt.Add(time.Minute),
	}); !errors.Is(err, ErrRecordNotFound) {
		t.Fatalf("MarkFailed stale claim error = %v, want ErrRecordNotFound", err)
	}
}

func TestRelayRunOncePublishesAndRetriesFailures(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	t.Cleanup(cancel)

	db := openPostgresDB(ctx, t)
	store := newTestStore(ctx, t, db)
	if err := store.Enqueue(ctx, db,
		mustEntry(t, 1, "event-ok", "idem-ok"),
		mustEntryForAggregate(t, "43", 1, "event-retry", "idem-retry"),
	); err != nil {
		t.Fatalf("Enqueue: %v", err)
	}

	publisher := &recordingPublisher{failures: map[audit.EventID]int{"event-retry": 1}}
	relay, err := NewRelay(store, publisher, RelayOptions{
		ClaimLimit:  10,
		MaxAttempts: 2,
		RetryDelay:  time.Minute,
		Now:         func() time.Time { return testClock.Add(10 * time.Minute) },
	})
	if err != nil {
		t.Fatalf("NewRelay: %v", err)
	}

	result, err := relay.RunOnce(ctx, db)
	if err != nil {
		t.Fatalf("RunOnce: %v", err)
	}
	if result.Claimed != 2 || result.Published != 1 || result.Failed != 1 || result.DeadLettered != 0 {
		t.Fatalf("RunOnce result = %#v", result)
	}
	if got := publisher.eventIDs(); !reflect.DeepEqual(got, []audit.EventID{"event-ok", "event-retry"}) {
		t.Fatalf("published event IDs = %v", got)
	}
	if got := countStatus(ctx, t, db, StatusPublished); got != 1 {
		t.Fatalf("published count = %d, want 1", got)
	}
	if got := countStatus(ctx, t, db, StatusPending); got != 1 {
		t.Fatalf("pending count = %d, want 1", got)
	}
}

func TestRelayRunOncePublisherContextCancellationDoesNotRetry(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	t.Cleanup(cancel)

	db := openPostgresDB(ctx, t)
	store := newTestStore(ctx, t, db)
	if err := store.Enqueue(ctx, db, mustEntry(t, 1, "event-cancel", "idem-cancel")); err != nil {
		t.Fatalf("Enqueue: %v", err)
	}

	runCtx, cancelRun := context.WithCancel(ctx)
	publisher := publisherFunc(func(context.Context, Record) error {
		cancelRun()
		return fmt.Errorf("publisher shutdown: %w", context.Canceled)
	})
	relay, err := NewRelay(store, publisher, RelayOptions{
		ClaimLimit:  1,
		MaxAttempts: 2,
		RetryDelay:  time.Minute,
		Now:         func() time.Time { return testClock.Add(20 * time.Minute) },
	})
	if err != nil {
		t.Fatalf("NewRelay: %v", err)
	}

	result, err := relay.RunOnce(runCtx, db)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("RunOnce error = %v, want context.Canceled", err)
	}
	if result.Claimed != 1 || result.Published != 0 || result.Failed != 0 || result.DeadLettered != 0 {
		t.Fatalf("RunOnce result = %#v, want claimed-only cancellation", result)
	}
	if got := statusForEvent(ctx, t, db, "event-cancel"); got != StatusClaimed {
		t.Fatalf("status after cancellation = %q, want %q", got, StatusClaimed)
	}
	if got := lastErrorForEvent(ctx, t, db, "event-cancel"); got != "" {
		t.Fatalf("last_error after cancellation = %q, want empty", got)
	}
}

func TestRelayRunOnceRetriesDuplicatePublishWithStableEnvelope(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	t.Cleanup(cancel)

	db := openPostgresDB(ctx, t)
	store := newTestStore(ctx, t, db)
	if err := store.Enqueue(ctx, db, mustEntry(t, 1, "event-duplicate-publish", "idem-duplicate-publish")); err != nil {
		t.Fatalf("Enqueue: %v", err)
	}

	publisher := &recordingPublisher{
		failures: map[audit.EventID]int{"event-duplicate-publish": 1},
	}
	relay, err := NewRelay(store, publisher, RelayOptions{
		ClaimLimit:  1,
		MaxAttempts: 3,
		RetryDelay:  time.Minute,
		Now:         func() time.Time { return testClock.Add(30 * time.Minute) },
	})
	if err != nil {
		t.Fatalf("NewRelay: %v", err)
	}

	result, err := relay.RunOnce(ctx, db)
	if err != nil {
		t.Fatalf("RunOnce first attempt: %v", err)
	}
	if result.Claimed != 1 || result.Published != 0 || result.Failed != 1 || result.DeadLettered != 0 {
		t.Fatalf("first RunOnce result = %#v, want one retry failure", result)
	}

	relay.now = func() time.Time { return testClock.Add(31 * time.Minute) }
	result, err = relay.RunOnce(ctx, db)
	if err != nil {
		t.Fatalf("RunOnce retry: %v", err)
	}
	if result.Claimed != 1 || result.Published != 1 || result.Failed != 0 || result.DeadLettered != 0 {
		t.Fatalf("retry RunOnce result = %#v, want one publish", result)
	}

	records := publisher.records()
	if len(records) != 2 {
		t.Fatalf("publish attempts = %d, want 2", len(records))
	}
	first, second := records[0], records[1]
	if first.Entry.Event.EventID != second.Entry.Event.EventID {
		t.Fatalf("event ID changed across retry: %q vs %q", first.Entry.Event.EventID, second.Entry.Event.EventID)
	}
	if first.Entry.Event.IdempotencyKey != second.Entry.Event.IdempotencyKey {
		t.Fatalf("idempotency key changed across retry: %q vs %q", first.Entry.Event.IdempotencyKey, second.Entry.Event.IdempotencyKey)
	}
	if first.Attempts != 1 || second.Attempts != 2 {
		t.Fatalf("attempts = %d, %d; want 1, 2", first.Attempts, second.Attempts)
	}
	if got := statusForEvent(ctx, t, db, "event-duplicate-publish"); got != StatusPublished {
		t.Fatalf("status after retry = %q, want %q", got, StatusPublished)
	}
}

func TestRelayRunStopsOnContextCancellation(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	t.Cleanup(cancel)

	db := openPostgresDB(ctx, t)
	store := newTestStore(ctx, t, db)
	relay, err := NewRelay(store, &recordingPublisher{}, RelayOptions{
		ClaimLimit: 1,
		IdleDelay:  5 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("NewRelay: %v", err)
	}

	tester := concurrencytest.NewAsyncJobTester(concurrencytest.Options{
		Workers:       2,
		RoundsPerTask: 3,
		Timeout:       5 * time.Second,
	})
	tester.RunT(t, func(ctx context.Context) error {
		runCtx, cancel := context.WithTimeout(ctx, 20*time.Millisecond)
		defer cancel()
		err := relay.Run(runCtx, db)
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
			return nil
		}
		if err == nil {
			return errors.New("Run returned nil, want context cancellation")
		}
		return fmt.Errorf("Run returned unexpected error, want context cancellation: %w", err)
	})
}

func TestRelayRunOnceConcurrentStressPublishesEachRecordOnce(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	t.Cleanup(cancel)

	db := openPostgresDB(ctx, t)
	store := newTestStore(ctx, t, db)

	const entries = 12
	toEnqueue := make([]audit.Entry, 0, entries)
	for i := 1; i <= entries; i++ {
		toEnqueue = append(toEnqueue, mustEntryForAggregate(t, fmt.Sprintf("stress-%02d", i), 1, fmt.Sprintf("event-stress-%02d", i), fmt.Sprintf("idem-stress-%02d", i)))
	}
	if err := store.Enqueue(ctx, db, toEnqueue...); err != nil {
		t.Fatalf("Enqueue: %v", err)
	}

	var seen sync.Map
	publisher := publisherFunc(func(ctx context.Context, record Record) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		eventID := record.Entry.Event.EventID
		if _, loaded := seen.LoadOrStore(eventID, record.Attempts); loaded {
			return fmt.Errorf("duplicate publish attempt for %s", eventID)
		}
		return nil
	})
	relay, err := NewRelay(store, publisher, RelayOptions{
		ClaimLimit: 1,
		Now:        func() time.Time { return testClock.Add(40 * time.Minute) },
	})
	if err != nil {
		t.Fatalf("NewRelay: %v", err)
	}

	tester := concurrencytest.NewGoroutineStressTester(concurrencytest.Options{
		Workers:       4,
		RoundsPerTask: entries * 2,
		Timeout:       10 * time.Second,
	})
	tester.RunT(t, func(ctx context.Context) error {
		_, err := relay.RunOnce(ctx, db)
		return err
	})

	if got := countStatus(ctx, t, db, StatusPublished); got != entries {
		t.Fatalf("published count = %d, want %d", got, entries)
	}
}

func TestStoreConcurrentClaimsDoNotDuplicateRecords(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	t.Cleanup(cancel)

	db := openPostgresDB(ctx, t)
	store := newTestStore(ctx, t, db)

	const entries = 16
	toEnqueue := make([]audit.Entry, 0, entries)
	for i := 1; i <= entries; i++ {
		toEnqueue = append(toEnqueue, mustEntryForAggregate(t, fmt.Sprintf("%02d", i), 1, fmt.Sprintf("event-%02d", i), fmt.Sprintf("idem-%02d", i)))
	}
	if err := store.Enqueue(ctx, db, toEnqueue...); err != nil {
		t.Fatalf("Enqueue: %v", err)
	}

	var seen sync.Map
	tester := concurrencytest.NewGoroutineStressTester(concurrencytest.Options{
		Workers:       4,
		RoundsPerTask: entries * 2,
		Timeout:       10 * time.Second,
	})
	tester.RunT(t, func(ctx context.Context) error {
		records, err := store.Claim(ctx, db, ClaimOptions{Limit: 1, Now: testClock.Add(time.Hour)})
		if err != nil {
			return err
		}
		if len(records) == 0 {
			return nil
		}
		record := records[0]
		if _, loaded := seen.LoadOrStore(record.ID, record.Entry.Event.EventID); loaded {
			return fmt.Errorf("record %d claimed more than once", record.ID)
		}
		return store.MarkPublished(ctx, db, record)
	})

	if got := countStatus(ctx, t, db, StatusPublished); got != entries {
		t.Fatalf("published count = %d, want %d", got, entries)
	}
}

func TestZeroValueValidation(t *testing.T) {
	if _, err := NewStore(Options{Table: "bad table"}); !errors.Is(err, ErrInvalidArgument) {
		t.Fatalf("NewStore invalid table error = %v, want ErrInvalidArgument", err)
	}
	if _, err := NewStore(Options{MaxEntryBytes: -1}); !errors.Is(err, ErrInvalidArgument) {
		t.Fatalf("NewStore invalid max entry bytes error = %v, want ErrInvalidArgument", err)
	}

	store, err := NewStore(Options{})
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	if _, err := NewRelay(nil, &recordingPublisher{}, RelayOptions{}); !errors.Is(err, ErrInvalidArgument) {
		t.Fatalf("NewRelay nil store error = %v, want ErrInvalidArgument", err)
	}
	if _, err := NewRelay(store, nil, RelayOptions{}); !errors.Is(err, ErrInvalidArgument) {
		t.Fatalf("NewRelay nil publisher error = %v, want ErrInvalidArgument", err)
	}
	if _, err := NewRelay(store, &recordingPublisher{}, RelayOptions{ClaimLimit: -1}); !errors.Is(err, ErrInvalidArgument) {
		t.Fatalf("NewRelay invalid claim limit error = %v, want ErrInvalidArgument", err)
	}
	if _, err := store.Claim(context.Background(), stubSession{}, ClaimOptions{}); !errors.Is(err, ErrInvalidArgument) {
		t.Fatalf("Claim zero limit error = %v, want ErrInvalidArgument", err)
	}
	if err := store.MarkFailed(context.Background(), stubSession{}, Failure{}); !errors.Is(err, ErrInvalidArgument) {
		t.Fatalf("MarkFailed zero failure error = %v, want ErrInvalidArgument", err)
	}
}

var testClock = time.Date(2026, 6, 28, 12, 0, 0, 0, time.UTC)

func openPostgresDB(ctx context.Context, t *testing.T) *sql.DB {
	t.Helper()

	dsn := postgrestestcontainer.Start(ctx, t)
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatalf("open postgres: %v", err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Fatalf("close postgres: %v", err)
		}
	})
	if err := db.PingContext(ctx); err != nil {
		t.Fatalf("ping postgres: %v", err)
	}
	return db
}

func newTestStore(ctx context.Context, t *testing.T, db *sql.DB) *Store {
	t.Helper()

	store, err := NewStore(Options{})
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	if err := store.CreateSchema(ctx, db); err != nil {
		t.Fatalf("CreateSchema: %v", err)
	}
	return store
}

func claimOne(ctx context.Context, t *testing.T, store *Store, db *sql.DB, now time.Time) Record {
	t.Helper()

	return claimOneWithOptions(ctx, t, store, db, ClaimOptions{Limit: 1, Now: now})
}

func claimOneWithOptions(ctx context.Context, t *testing.T, store *Store, db *sql.DB, options ClaimOptions) Record {
	t.Helper()

	records, err := store.Claim(ctx, db, options)
	if err != nil {
		t.Fatalf("Claim: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("Claim returned %d records, want 1", len(records))
	}
	return records[0]
}

func countStatus(ctx context.Context, t *testing.T, db *sql.DB, status Status) int {
	t.Helper()

	var count int
	if err := db.QueryRowContext(ctx, `select count(*) from audit_outbox where status = $1`, string(status)).Scan(&count); err != nil {
		t.Fatalf("count status %q: %v", status, err)
	}
	return count
}

func statusForEvent(ctx context.Context, t *testing.T, db *sql.DB, eventID audit.EventID) Status {
	t.Helper()

	var status string
	if err := db.QueryRowContext(ctx, `select status from audit_outbox where event_id = $1`, string(eventID)).Scan(&status); err != nil {
		t.Fatalf("status for event %q: %v", eventID, err)
	}
	return Status(status)
}

func lastErrorForEvent(ctx context.Context, t *testing.T, db *sql.DB, eventID audit.EventID) string {
	t.Helper()

	var lastError sql.NullString
	if err := db.QueryRowContext(ctx, `select last_error from audit_outbox where event_id = $1`, string(eventID)).Scan(&lastError); err != nil {
		t.Fatalf("last_error for event %q: %v", eventID, err)
	}
	if !lastError.Valid {
		return ""
	}
	return lastError.String
}

func mustEntry(t *testing.T, revision uint64, eventID string, idempotencyKey string) audit.Entry {
	t.Helper()
	return mustEntryForAggregate(t, "42", revision, eventID, idempotencyKey)
}

func mustEntryForAggregate(t *testing.T, aggregateID string, revision uint64, eventID string, idempotencyKey string) audit.Entry {
	t.Helper()

	aggregate, err := audit.NewAggregateID("account", aggregateID)
	if err != nil {
		t.Fatalf("NewAggregateID: %v", err)
	}
	event, err := audit.NewDomainEvent(audit.EventOptions{
		EventID:        audit.EventID(eventID),
		EventType:      audit.EventType("AccountCredited"),
		AggregateID:    aggregate,
		Revision:       audit.Revision(revision),
		OccurredAt:     testClock.Add(time.Duration(revision) * time.Second),
		RecordedAt:     testClock.Add(time.Duration(revision) * time.Second),
		IdempotencyKey: idempotencyKey,
		Metadata:       audit.Metadata{"source": "sqloutbox-test"},
		Payload:        json.RawMessage(`{"amount":100}`),
	})
	if err != nil {
		t.Fatalf("NewDomainEvent: %v", err)
	}
	entry, err := audit.NewEntry(audit.EntryOptions{
		Author: "tester",
		Event:  event,
	})
	if err != nil {
		t.Fatalf("NewEntry: %v", err)
	}
	return entry
}

type recordingPublisher struct {
	mu        sync.Mutex
	failures  map[audit.EventID]int
	published []Record
}

type publisherFunc func(context.Context, Record) error

func (fn publisherFunc) Publish(ctx context.Context, record Record) error {
	return fn(ctx, record)
}

type stubSession struct{}

func (stubSession) ExecContext(context.Context, string, ...any) (sql.Result, error) {
	return nil, nil
}

func (stubSession) QueryContext(context.Context, string, ...any) (*sql.Rows, error) {
	return nil, nil
}

func (stubSession) QueryRowContext(context.Context, string, ...any) *sql.Row {
	return &sql.Row{}
}

func (p *recordingPublisher) Publish(ctx context.Context, record Record) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	p.published = append(p.published, record)
	if p.failures != nil && p.failures[record.Entry.Event.EventID] > 0 {
		p.failures[record.Entry.Event.EventID]--
		return errors.New("temporary publisher failure")
	}
	return nil
}

func (p *recordingPublisher) eventIDs() []audit.EventID {
	p.mu.Lock()
	defer p.mu.Unlock()

	ids := make([]audit.EventID, len(p.published))
	for i, record := range p.published {
		ids[i] = record.Entry.Event.EventID
	}
	return ids
}

func (p *recordingPublisher) records() []Record {
	p.mu.Lock()
	defer p.mu.Unlock()

	records := make([]Record, len(p.published))
	copy(records, p.published)
	return records
}
