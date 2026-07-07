package sqloutboxtest_test

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"reflect"
	"sync/atomic"
	"testing"
	"time"

	"github.com/bluetape4k/bluetape-go/audit"
	"github.com/bluetape4k/bluetape-go/audit/sqloutbox"
	"github.com/bluetape4k/bluetape-go/audit/sqloutbox/sqloutboxtest"
	postgrestestcontainer "github.com/bluetape4k/bluetape-go/testcontainers/postgres"
	concurrencytest "github.com/bluetape4k/bluetape-go/testing/concurrency"
	_ "github.com/jackc/pgx/v5/stdlib"
)

func TestDiscardPublisherHonorsContextCancellation(t *testing.T) {
	var publisher sqloutboxtest.DiscardPublisher
	if err := publisher.Publish(context.Background(), testRecord(t, "discard-ok", "discard-ok", 1)); err != nil {
		t.Fatalf("Publish background: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := publisher.Publish(ctx, testRecord(t, "discard-cancel", "discard-cancel", 1)); !errors.Is(err, context.Canceled) {
		t.Fatalf("Publish canceled error = %v, want context.Canceled", err)
	}
	var nilContext context.Context
	if err := publisher.Publish(nilContext, testRecord(t, "discard-nil-context", "discard-nil-context", 1)); err == nil {
		t.Fatal("Publish nil context error = nil, want error")
	}
}

func TestPublisherFuncAdaptsFunctionAndRejectsNil(t *testing.T) {
	record := testRecord(t, "func-ok", "func-ok", 1)
	var called atomic.Int32
	publisher := sqloutboxtest.PublisherFunc(func(ctx context.Context, got sqloutbox.Record) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		called.Add(1)
		if got.EventID != record.EventID {
			t.Fatalf("record event ID = %q, want %q", got.EventID, record.EventID)
		}
		return nil
	})

	if err := publisher.Publish(context.Background(), record); err != nil {
		t.Fatalf("Publish: %v", err)
	}
	if got := called.Load(); got != 1 {
		t.Fatalf("called = %d, want 1", got)
	}

	var nilPublisher sqloutboxtest.PublisherFunc
	if err := nilPublisher.Publish(context.Background(), record); !errors.Is(err, sqloutboxtest.ErrNilPublisherFunc) {
		t.Fatalf("nil PublisherFunc error = %v, want ErrNilPublisherFunc", err)
	}
}

func TestRecordingPublisherRejectsNilReceiver(t *testing.T) {
	var publisher *sqloutboxtest.RecordingPublisher
	err := publisher.Publish(context.Background(), testRecord(t, "record-nil-receiver", "idem-nil-receiver", 1))
	if !errors.Is(err, sqloutboxtest.ErrNilRecordingPublisher) {
		t.Fatalf("nil RecordingPublisher error = %v, want ErrNilRecordingPublisher", err)
	}
}

func TestRecordingPublisherRecordsCopiesAndEventIDs(t *testing.T) {
	publisher := sqloutboxtest.NewRecordingPublisher()
	first := testRecord(t, "record-first", "idem-first", 1)
	second := testRecord(t, "record-second", "idem-second", 1)

	if err := publisher.Publish(context.Background(), first); err != nil {
		t.Fatalf("Publish first: %v", err)
	}
	if err := publisher.Publish(context.Background(), second); err != nil {
		t.Fatalf("Publish second: %v", err)
	}
	if got, want := publisher.EventIDs(), []audit.EventID{"record-first", "record-second"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("EventIDs = %v, want %v", got, want)
	}

	records := publisher.Records()
	records[0] = testRecord(t, "mutated", "mutated", 1)
	if got := publisher.EventIDs()[0]; got != "record-first" {
		t.Fatalf("mutating Records result changed publisher state to %q", got)
	}
	publisher.Reset()
	if got := publisher.Count(); got != 0 {
		t.Fatalf("Count after Reset = %d, want 0", got)
	}
}

func TestRecordingPublisherFailureInjectionIsDeterministic(t *testing.T) {
	injected := errors.New("publisher unavailable")
	publisher := sqloutboxtest.NewRecordingPublisher(
		sqloutboxtest.WithFailures(map[audit.EventID]int{"record-retry": 2}, injected),
	)
	record := testRecord(t, "record-retry", "idem-retry", 1)

	for attempt := 1; attempt <= 2; attempt++ {
		if err := publisher.Publish(context.Background(), record); !errors.Is(err, injected) {
			t.Fatalf("attempt %d error = %v, want injected error", attempt, err)
		}
	}
	if err := publisher.Publish(context.Background(), record); err != nil {
		t.Fatalf("third Publish: %v", err)
	}
	if got := publisher.Count(); got != 3 {
		t.Fatalf("Count = %d, want 3 attempts recorded", got)
	}
}

func TestRecordingPublisherConcurrentStress(t *testing.T) {
	publisher := sqloutboxtest.NewRecordingPublisher()
	var sequence atomic.Int64
	stress := concurrencytest.NewGoroutineStressTester(concurrencytest.Options{
		Workers:       4,
		RoundsPerTask: 16,
		Timeout:       5 * time.Second,
	})
	stress.RunT(t, func(ctx context.Context) error {
		next := sequence.Add(1)
		record := testRecord(t, audit.EventID(fmt.Sprintf("stress-%03d", next)), fmt.Sprintf("idem-stress-%03d", next), 1)
		return publisher.Publish(ctx, record)
	})

	if got := publisher.Count(); got != int(sequence.Load()) {
		t.Fatalf("Count = %d, want %d", got, sequence.Load())
	}
}

func TestRecordingPublisherSupportsRelayRetryAndDeadLetterHandoff(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	t.Cleanup(cancel)

	db := openPostgresDB(ctx, t)
	store := newTestStore(ctx, t, db)
	if err := store.Enqueue(ctx, db,
		testEntry(t, "relay-retry", "relay-retry", 1),
		testEntryForAggregate(t, "dead", "relay-dead-letter", "relay-dead-letter", 1),
	); err != nil {
		t.Fatalf("Enqueue: %v", err)
	}

	publisher := sqloutboxtest.NewRecordingPublisher(
		sqloutboxtest.WithFailures(map[audit.EventID]int{
			"relay-retry":       1,
			"relay-dead-letter": 2,
		}, errors.New("temporary sink failure")),
	)
	now := testClock.Add(time.Hour)
	relay, err := sqloutbox.NewRelay(store, publisher, sqloutbox.RelayOptions{
		ClaimLimit:  2,
		MaxAttempts: 2,
		RetryDelay:  time.Millisecond,
		Now:         func() time.Time { return now },
	})
	if err != nil {
		t.Fatalf("NewRelay: %v", err)
	}

	first, err := relay.RunOnce(ctx, db)
	if err != nil {
		t.Fatalf("RunOnce first: %v", err)
	}
	if first.Claimed != 2 || first.Published != 0 || first.Failed != 2 || first.DeadLettered != 0 {
		t.Fatalf("first RunOnce result = %#v", first)
	}
	now = now.Add(time.Millisecond)
	second, err := relay.RunOnce(ctx, db)
	if err != nil {
		t.Fatalf("RunOnce retry: %v", err)
	}
	if second.Claimed != 2 || second.Published != 1 || second.Failed != 0 || second.DeadLettered != 1 {
		t.Fatalf("retry RunOnce result = %#v", second)
	}
	if got, want := publisher.EventIDs(), []audit.EventID{"relay-retry", "relay-dead-letter", "relay-retry", "relay-dead-letter"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("EventIDs = %v, want %v", got, want)
	}
}

var testClock = time.Date(2026, 7, 7, 12, 0, 0, 0, time.UTC)

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

func newTestStore(ctx context.Context, t *testing.T, db *sql.DB) *sqloutbox.Store {
	t.Helper()

	store, err := sqloutbox.NewStore(sqloutbox.Options{})
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	if err := store.CreateSchema(ctx, db); err != nil {
		t.Fatalf("CreateSchema: %v", err)
	}
	return store
}

func testRecord(t *testing.T, eventID audit.EventID, idempotencyKey string, attempts int) sqloutbox.Record {
	t.Helper()

	entry := testEntry(t, eventID, idempotencyKey, 1)
	return sqloutbox.Record{
		ID:             sqloutbox.RecordID(attempts),
		Status:         sqloutbox.StatusClaimed,
		Aggregate:      entry.Aggregate,
		Revision:       entry.Revision,
		EventID:        entry.Event.EventID,
		IdempotencyKey: entry.Event.IdempotencyKey,
		EventType:      entry.Event.EventType,
		OccurredAt:     entry.Event.OccurredAt,
		RecordedAt:     entry.Event.RecordedAt,
		SchemaVersion:  entry.SchemaVersion,
		Attempts:       attempts,
		Entry:          entry,
	}
}

func testEntry(t *testing.T, eventID audit.EventID, idempotencyKey string, revision uint64) audit.Entry {
	t.Helper()
	return testEntryForAggregate(t, "42", eventID, idempotencyKey, revision)
}

func testEntryForAggregate(t *testing.T, aggregateID string, eventID audit.EventID, idempotencyKey string, revision uint64) audit.Entry {
	t.Helper()

	aggregate, err := audit.NewAggregateID("account", aggregateID)
	if err != nil {
		t.Fatalf("NewAggregateID: %v", err)
	}
	event, err := audit.NewDomainEvent(audit.EventOptions{
		EventID:        eventID,
		EventType:      audit.EventType("AccountCredited"),
		AggregateID:    aggregate,
		Revision:       audit.Revision(revision),
		OccurredAt:     testClock.Add(time.Duration(revision) * time.Second),
		RecordedAt:     testClock.Add(time.Duration(revision) * time.Second),
		IdempotencyKey: idempotencyKey,
		Metadata:       audit.Metadata{"source": "sqloutboxtest"},
		Payload:        []byte(`{"amount":100}`),
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
