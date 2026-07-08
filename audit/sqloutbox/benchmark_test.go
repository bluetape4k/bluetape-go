package sqloutbox

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/bluetape4k/bluetape-go/audit"
	postgrestestcontainer "github.com/bluetape4k/bluetape-go/testcontainers/postgres"
	_ "github.com/jackc/pgx/v5/stdlib"
)

const (
	auditSQLOutboxBenchEnv       = "BLUETAPE_AUDIT_SQL_OUTBOX_BENCH"
	auditSQLOutboxOperationLimit = 10 * time.Second
)

var auditSQLOutboxBenchmarkClock = time.Date(2026, 7, 9, 0, 0, 0, 0, time.UTC)

func BenchmarkAuditSQLOutboxPostgres(b *testing.B) {
	if os.Getenv(auditSQLOutboxBenchEnv) != "1" {
		b.Skipf("set %s=1 to run serial Testcontainers-backed PostgreSQL benchmarks", auditSQLOutboxBenchEnv)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	b.Cleanup(cancel)
	db := openBenchmarkPostgresDB(ctx, b)
	store := newBenchmarkStore(ctx, b, db)

	b.Run("Enqueue/Batch10/Payload512", func(b *testing.B) {
		runEnqueueBenchmark(ctx, b, store, db, 10, 512)
	})
	b.Run("Claim/Limit10/Pending100/Payload512", func(b *testing.B) {
		runClaimBenchmark(ctx, b, store, db, 100, 10, 512)
	})
	b.Run("RunOnce/Publish10/Payload512", func(b *testing.B) {
		runRelayPublishBenchmark(ctx, b, store, db, 10, 512)
	})
	b.Run("RunOnce/DeadLetter10/Payload512", func(b *testing.B) {
		runRelayDeadLetterBenchmark(ctx, b, store, db, 10, 512)
	})
}

func runEnqueueBenchmark(ctx context.Context, b *testing.B, store *Store, db *sql.DB, batchSize int, payloadSize int) {
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		resetBenchmarkOutbox(ctx, b, db)
		entries := benchmarkOutboxEntries(b, "enqueue", 0, batchSize, payloadSize)

		opCtx, cancel := benchmarkOperationContext(ctx)
		b.StartTimer()
		err := store.Enqueue(opCtx, db, entries...)
		cancel()
		b.StopTimer()
		if err != nil {
			b.Fatalf("Enqueue: %v", err)
		}
	}
}

func runClaimBenchmark(ctx context.Context, b *testing.B, store *Store, db *sql.DB, pendingCount int, claimLimit int, payloadSize int) {
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		resetBenchmarkOutbox(ctx, b, db)
		entries := benchmarkOutboxEntries(b, "claim", 0, pendingCount, payloadSize)
		if err := store.Enqueue(ctx, db, entries...); err != nil {
			b.Fatalf("seed Enqueue: %v", err)
		}

		opCtx, cancel := benchmarkOperationContext(ctx)
		b.StartTimer()
		records, err := store.Claim(opCtx, db, ClaimOptions{Limit: claimLimit, Now: auditSQLOutboxBenchmarkClock.Add(time.Hour)})
		cancel()
		b.StopTimer()
		if err != nil {
			b.Fatalf("Claim: %v", err)
		}
		if len(records) != claimLimit {
			b.Fatalf("Claim returned %d records, want %d", len(records), claimLimit)
		}
	}
}

func runRelayPublishBenchmark(ctx context.Context, b *testing.B, store *Store, db *sql.DB, batchSize int, payloadSize int) {
	publisher := benchmarkPublisher{}
	relay, err := NewRelay(store, publisher, RelayOptions{
		ClaimLimit:  batchSize,
		MaxAttempts: 3,
		RetryDelay:  time.Second,
		Now:         func() time.Time { return auditSQLOutboxBenchmarkClock.Add(2 * time.Hour) },
	})
	if err != nil {
		b.Fatalf("NewRelay: %v", err)
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		resetBenchmarkOutbox(ctx, b, db)
		entries := benchmarkOutboxEntries(b, "relay-publish", 0, batchSize, payloadSize)
		if err := store.Enqueue(ctx, db, entries...); err != nil {
			b.Fatalf("seed Enqueue: %v", err)
		}

		opCtx, cancel := benchmarkOperationContext(ctx)
		b.StartTimer()
		result, err := relay.RunOnce(opCtx, db)
		cancel()
		b.StopTimer()
		if err != nil {
			b.Fatalf("RunOnce: %v", err)
		}
		if result.Claimed != batchSize || result.Published != batchSize || result.Failed != 0 || result.DeadLettered != 0 {
			b.Fatalf("RunOnce result = %#v, want all published", result)
		}
	}
}

func runRelayDeadLetterBenchmark(ctx context.Context, b *testing.B, store *Store, db *sql.DB, batchSize int, payloadSize int) {
	relay, err := NewRelay(store, benchmarkPublisher{err: errors.New("publisher rejected benchmark payload")}, RelayOptions{
		ClaimLimit:  batchSize,
		MaxAttempts: 1,
		RetryDelay:  time.Second,
		Now:         func() time.Time { return auditSQLOutboxBenchmarkClock.Add(3 * time.Hour) },
	})
	if err != nil {
		b.Fatalf("NewRelay: %v", err)
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		resetBenchmarkOutbox(ctx, b, db)
		entries := benchmarkOutboxEntries(b, "relay-dead-letter", 0, batchSize, payloadSize)
		if err := store.Enqueue(ctx, db, entries...); err != nil {
			b.Fatalf("seed Enqueue: %v", err)
		}

		opCtx, cancel := benchmarkOperationContext(ctx)
		b.StartTimer()
		result, err := relay.RunOnce(opCtx, db)
		cancel()
		b.StopTimer()
		if err != nil {
			b.Fatalf("RunOnce: %v", err)
		}
		if result.Claimed != batchSize || result.Published != 0 || result.Failed != 0 || result.DeadLettered != batchSize {
			b.Fatalf("RunOnce result = %#v, want all dead-lettered", result)
		}
	}
}

func openBenchmarkPostgresDB(ctx context.Context, b *testing.B) *sql.DB {
	b.Helper()

	dsn := postgrestestcontainer.Start(ctx, b)
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		b.Fatalf("open postgres: %v", err)
	}
	b.Cleanup(func() {
		if err := db.Close(); err != nil {
			b.Fatalf("close postgres: %v", err)
		}
	})
	if err := db.PingContext(ctx); err != nil {
		b.Fatalf("ping postgres: %v", err)
	}
	return db
}

func newBenchmarkStore(ctx context.Context, b *testing.B, db *sql.DB) *Store {
	b.Helper()

	store, err := NewStore(Options{})
	if err != nil {
		b.Fatalf("NewStore: %v", err)
	}
	if err := store.CreateSchema(ctx, db); err != nil {
		b.Fatalf("CreateSchema: %v", err)
	}
	return store
}

func resetBenchmarkOutbox(ctx context.Context, b *testing.B, db *sql.DB) {
	b.Helper()

	if _, err := db.ExecContext(ctx, `truncate table audit_outbox restart identity`); err != nil {
		b.Fatalf("reset outbox: %v", err)
	}
}

func benchmarkOutboxEntries(tb testing.TB, prefix string, aggregateOffset int, count int, payloadSize int) []audit.Entry {
	tb.Helper()

	entries := make([]audit.Entry, count)
	for i := range entries {
		aggregate, err := audit.NewAggregateID("account", fmt.Sprintf("%s-%04d", prefix, aggregateOffset+i))
		if err != nil {
			tb.Fatalf("NewAggregateID: %v", err)
		}
		event, err := audit.NewDomainEvent(audit.EventOptions{
			EventID:        audit.EventID(fmt.Sprintf("%s-event-%d", prefix, aggregateOffset+i)),
			EventType:      audit.EventType("AccountChanged"),
			AggregateID:    aggregate,
			Revision:       audit.InitialRevision(),
			OccurredAt:     auditSQLOutboxBenchmarkClock.Add(time.Duration(i) * time.Second),
			RecordedAt:     auditSQLOutboxBenchmarkClock.Add(time.Duration(i) * time.Second),
			IdempotencyKey: fmt.Sprintf("%s-idem-%d", prefix, aggregateOffset+i),
			Metadata:       audit.Metadata{"source": "sqloutbox-benchmark"},
			Payload:        benchmarkOutboxPayload(payloadSize),
		})
		if err != nil {
			tb.Fatalf("NewDomainEvent: %v", err)
		}
		entry, err := audit.NewEntry(audit.EntryOptions{
			Author: "benchmark",
			Event:  event,
		})
		if err != nil {
			tb.Fatalf("NewEntry: %v", err)
		}
		entries[i] = entry
	}
	return entries
}

func benchmarkOperationContext(ctx context.Context) (context.Context, context.CancelFunc) {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithTimeout(ctx, auditSQLOutboxOperationLimit)
}

func benchmarkOutboxPayload(size int) json.RawMessage {
	if size <= len(`{"blob":""}`) {
		return json.RawMessage(`{}`)
	}
	return json.RawMessage(fmt.Sprintf(`{"blob":"%s"}`, strings.Repeat("x", size-len(`{"blob":""}`))))
}

type benchmarkPublisher struct {
	err error
}

func (p benchmarkPublisher) Publish(ctx context.Context, _ Record) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return p.err
}
