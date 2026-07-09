package redisstreams

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/bluetape4k/bluetape-go/audit"
	"github.com/bluetape4k/bluetape-go/audit/sqloutbox"
	postgrestestcontainer "github.com/bluetape4k/bluetape-go/testcontainers/postgres"
	redistestcontainer "github.com/bluetape4k/bluetape-go/testcontainers/redis"
	bttesting "github.com/bluetape4k/bluetape-go/testing"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/redis/go-redis/v9"
)

func TestNewRejectsInvalidOptions(t *testing.T) {
	client := &fakeClient{}
	tests := []struct {
		name    string
		options Options
	}{
		{name: "missing client", options: Options{}},
		{name: "typed nil client", options: Options{Client: (*fakeClient)(nil)}},
		{name: "blank stream", options: Options{Client: client, Stream: "  "}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := New(tt.options); !errors.Is(err, sqloutbox.ErrInvalidArgument) {
				t.Fatalf("New error = %v, want ErrInvalidArgument", err)
			}
		})
	}
}

func TestNewAppliesDefaultStream(t *testing.T) {
	publisher, err := New(Options{Client: &fakeClient{}})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if got := publisher.Stream(); got != defaultStream {
		t.Fatalf("Stream = %q, want %q", got, defaultStream)
	}
}

func TestNewPreservesCallerOwnedStreamKey(t *testing.T) {
	const stream = " audit:sqloutbox:tenant "
	publisher, err := New(Options{Client: &fakeClient{}, Stream: stream})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if got := publisher.Stream(); got != stream {
		t.Fatalf("Stream = %q, want exact caller-owned key %q", got, stream)
	}
}

func TestPublishAppendsStableRecordValues(t *testing.T) {
	client := &fakeClient{}
	publisher, err := New(Options{Client: client, Stream: "audit:test"})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	record := testRecord(t, 7, "event-7", "idem-7", 2)

	if err := publisher.Publish(context.Background(), record); err != nil {
		t.Fatalf("Publish: %v", err)
	}

	if client.calls != 1 {
		t.Fatalf("XAdd calls = %d, want 1", client.calls)
	}
	if client.args.Stream != "audit:test" {
		t.Fatalf("stream = %q, want audit:test", client.args.Stream)
	}
	values := client.args.Values.(map[string]any)
	assertValue(t, values, "record_id", "7")
	assertValue(t, values, "event_id", "event-7")
	assertValue(t, values, "idempotency_key", "idem-7")
	assertValue(t, values, "aggregate_type", "account")
	assertValue(t, values, "aggregate_id", "42")
	assertValue(t, values, "revision", "7")
	assertValue(t, values, "event_type", "AccountCredited")
	assertValue(t, values, "schema_version", "1")
	assertValue(t, values, "attempts", "2")

	var entry audit.Entry
	if err := json.Unmarshal([]byte(valueString(t, values, "entry_json")), &entry); err != nil {
		t.Fatalf("entry_json decode: %v", err)
	}
	if entry.Event.EventID != record.EventID || entry.Event.IdempotencyKey != record.IdempotencyKey {
		t.Fatalf("entry_json did not preserve event identity: %#v", entry.Event)
	}
}

func TestPublishPreservesContextCancellation(t *testing.T) {
	client := &fakeClient{}
	publisher, err := New(Options{Client: client, Stream: "audit:test"})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if err := publisher.Publish(ctx, testRecord(t, 1, "event-cancel", "idem-cancel", 1)); !errors.Is(err, context.Canceled) {
		t.Fatalf("Publish error = %v, want context.Canceled", err)
	}
	if client.calls != 0 {
		t.Fatalf("XAdd calls after canceled context = %d, want 0", client.calls)
	}
}

func TestPublishSurfacesRedisError(t *testing.T) {
	injected := errors.New("redis unavailable")
	client := &fakeClient{err: injected}
	publisher, err := New(Options{Client: client, Stream: "audit:test"})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	if err := publisher.Publish(context.Background(), testRecord(t, 1, "event-error", "idem-error", 1)); !errors.Is(err, injected) {
		t.Fatalf("Publish error = %v, want injected error", err)
	}
}

func TestPublishRejectsUninitializedPublisher(t *testing.T) {
	var publisher *Publisher
	if err := publisher.Publish(context.Background(), testRecord(t, 1, "event-zero", "idem-zero", 1)); !errors.Is(err, sqloutbox.ErrInvalidArgument) {
		t.Fatalf("Publish error = %v, want ErrInvalidArgument", err)
	}

	publisher = &Publisher{client: (*fakeClient)(nil), stream: "audit:test"}
	if err := publisher.Publish(context.Background(), testRecord(t, 1, "event-typed-nil", "idem-typed-nil", 1)); !errors.Is(err, sqloutbox.ErrInvalidArgument) {
		t.Fatalf("Publish typed nil client error = %v, want ErrInvalidArgument", err)
	}
}

func TestPublishAppendsToRedisStream(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	t.Cleanup(cancel)

	client := redisClient(ctx, t)
	stream := testStream(t, "append")
	publisher := newPublisher(t, client, stream)
	record := testRecord(t, 1, "event-append", "idem-append", 1)

	if err := publisher.Publish(ctx, record); err != nil {
		t.Fatalf("Publish: %v", err)
	}

	messages := streamMessages(ctx, t, client, stream)
	if len(messages) != 1 {
		t.Fatalf("stream messages = %d, want 1", len(messages))
	}
	values := messages[0].Values
	assertValue(t, values, "event_id", "event-append")
	assertValue(t, values, "idempotency_key", "idem-append")
	assertValue(t, values, "attempts", "1")
	assertValue(t, values, "entry_json", string(mustEntryJSON(t, record.Entry)))
}

func TestPublishAllowsDuplicateAttemptsWithStableEnvelope(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	t.Cleanup(cancel)

	client := redisClient(ctx, t)
	stream := testStream(t, "duplicate")
	publisher := newPublisher(t, client, stream)
	first := testRecord(t, 1, "event-duplicate", "idem-duplicate", 1)
	second := first
	second.Attempts = 2

	if err := publisher.Publish(ctx, first); err != nil {
		t.Fatalf("Publish first: %v", err)
	}
	if err := publisher.Publish(ctx, second); err != nil {
		t.Fatalf("Publish second: %v", err)
	}

	messages := streamMessages(ctx, t, client, stream)
	if len(messages) != 2 {
		t.Fatalf("stream messages = %d, want 2", len(messages))
	}
	assertValue(t, messages[0].Values, "event_id", "event-duplicate")
	assertValue(t, messages[1].Values, "event_id", "event-duplicate")
	assertValue(t, messages[0].Values, "idempotency_key", "idem-duplicate")
	assertValue(t, messages[1].Values, "idempotency_key", "idem-duplicate")
	assertValue(t, messages[0].Values, "attempts", "1")
	assertValue(t, messages[1].Values, "attempts", "2")
}

func TestRelayRetriesPublishErrorThenAppendsRedisStreamAttempt(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	t.Cleanup(cancel)

	client := redisClient(ctx, t)
	stream := testStream(t, "relay-retry")
	db := openPostgresDB(ctx, t)
	store := newTestStore(ctx, t, db)
	if err := store.Enqueue(ctx, db, testEntry(t, 1, "event-relay-retry", "idem-relay-retry")); err != nil {
		t.Fatalf("Enqueue: %v", err)
	}

	publisher := &failOncePublisher{delegate: newPublisher(t, client, stream)}
	now := testClock.Add(time.Minute)
	relay, err := sqloutbox.NewRelay(store, publisher, sqloutbox.RelayOptions{
		ClaimLimit:  1,
		MaxAttempts: 3,
		RetryDelay:  time.Minute,
		Now:         func() time.Time { return now },
	})
	if err != nil {
		t.Fatalf("NewRelay: %v", err)
	}

	result, err := relay.RunOnce(ctx, db)
	if err != nil {
		t.Fatalf("RunOnce first: %v", err)
	}
	if result.Claimed != 1 || result.Published != 0 || result.Failed != 1 || result.DeadLettered != 0 {
		t.Fatalf("first RunOnce result = %#v, want retry failure", result)
	}
	if got := countStatus(ctx, t, db, sqloutbox.StatusPending); got != 1 {
		t.Fatalf("pending count after failure = %d, want 1", got)
	}
	if got := len(streamMessages(ctx, t, client, stream)); got != 0 {
		t.Fatalf("stream messages after failed publish = %d, want 0", got)
	}

	now = now.Add(time.Minute)
	result, err = relay.RunOnce(ctx, db)
	if err != nil {
		t.Fatalf("RunOnce retry: %v", err)
	}
	if result.Claimed != 1 || result.Published != 1 || result.Failed != 0 || result.DeadLettered != 0 {
		t.Fatalf("retry RunOnce result = %#v, want successful retry", result)
	}
	if got := countStatus(ctx, t, db, sqloutbox.StatusPublished); got != 1 {
		t.Fatalf("published count after retry = %d, want 1", got)
	}

	messages := streamMessages(ctx, t, client, stream)
	if len(messages) != 1 {
		t.Fatalf("stream messages after retry = %d, want 1", len(messages))
	}
	assertValue(t, messages[0].Values, "event_id", "event-relay-retry")
	assertValue(t, messages[0].Values, "idempotency_key", "idem-relay-retry")
	assertValue(t, messages[0].Values, "attempts", "2")
}

type fakeClient struct {
	args  *redis.XAddArgs
	calls int
	err   error
}

func (f *fakeClient) XAdd(_ context.Context, args *redis.XAddArgs) *redis.StringCmd {
	f.calls++
	f.args = args
	return redis.NewStringResult("1-0", f.err)
}

type failOncePublisher struct {
	delegate *Publisher
	failed   bool
}

func (p *failOncePublisher) Publish(ctx context.Context, record sqloutbox.Record) error {
	if !p.failed {
		p.failed = true
		return errors.New("temporary redis publish failure")
	}
	return p.delegate.Publish(ctx, record)
}

func newPublisher(t *testing.T, client Client, stream string) *Publisher {
	t.Helper()

	publisher, err := New(Options{Client: client, Stream: stream})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return publisher
}

func redisClient(ctx context.Context, t *testing.T) *redis.Client {
	t.Helper()

	client := redis.NewClient(&redis.Options{Addr: redistestcontainer.Start(ctx, t)})
	t.Cleanup(func() {
		_ = client.Close()
	})
	bttesting.Eventually(t, 5*time.Second, func() bool {
		return client.Ping(ctx).Err() == nil
	})
	return client
}

func openPostgresDB(ctx context.Context, t *testing.T) *sql.DB {
	t.Helper()

	db, err := sql.Open("pgx", postgrestestcontainer.Start(ctx, t))
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

func streamMessages(ctx context.Context, t *testing.T, client *redis.Client, stream string) []redis.XMessage {
	t.Helper()

	messages, err := client.XRange(ctx, stream, "-", "+").Result()
	if err != nil {
		t.Fatalf("XRange: %v", err)
	}
	return messages
}

func countStatus(ctx context.Context, t *testing.T, db *sql.DB, status sqloutbox.Status) int {
	t.Helper()

	var count int
	if err := db.QueryRowContext(ctx, `select count(*) from audit_outbox where status = $1`, string(status)).Scan(&count); err != nil {
		t.Fatalf("count status %q: %v", status, err)
	}
	return count
}

func testRecord(t *testing.T, revision uint64, eventID string, idempotencyKey string, attempts int) sqloutbox.Record {
	t.Helper()

	entry := testEntry(t, revision, eventID, idempotencyKey)
	return sqloutbox.Record{
		ID:             sqloutbox.RecordID(revision),
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

func testEntry(t *testing.T, revision uint64, eventID string, idempotencyKey string) audit.Entry {
	t.Helper()

	aggregate, err := audit.NewAggregateID("account", "42")
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
		Metadata:       audit.Metadata{"source": "redisstreams-test"},
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

func mustEntryJSON(t *testing.T, entry audit.Entry) []byte {
	t.Helper()

	data, err := json.Marshal(entry)
	if err != nil {
		t.Fatalf("Marshal entry: %v", err)
	}
	return data
}

func testStream(t *testing.T, suffix string) string {
	t.Helper()
	return "audit:sqloutbox:test:" + strings.NewReplacer("/", ":", " ", "-", "_", "-").Replace(t.Name()) + ":" + suffix
}

func assertValue(t *testing.T, values map[string]any, field string, want string) {
	t.Helper()

	if got := valueString(t, values, field); got != want {
		t.Fatalf("%s = %q, want %q", field, got, want)
	}
}

func valueString(t *testing.T, values map[string]any, field string) string {
	t.Helper()

	value, ok := values[field]
	if !ok {
		t.Fatalf("missing field %q in %v", field, values)
	}
	switch v := value.(type) {
	case string:
		return v
	case []byte:
		return string(v)
	default:
		return fmt.Sprint(v)
	}
}

var testClock = time.Date(2026, 7, 10, 12, 0, 0, 0, time.UTC)
