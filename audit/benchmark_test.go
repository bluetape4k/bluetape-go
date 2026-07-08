package audit

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"
)

var auditBenchmarkClock = time.Date(2026, 7, 9, 0, 0, 0, 0, time.UTC)

func BenchmarkMemoryRepositoryAppend(b *testing.B) {
	ctx := context.Background()
	cases := []struct {
		name        string
		historySize int
		batchSize   int
		payloadSize int
	}{
		{name: "History16/Batch1/Payload256", historySize: 16, batchSize: 1, payloadSize: 256},
		{name: "History256/Batch8/Payload2048", historySize: 256, batchSize: 8, payloadSize: 2048},
	}

	for _, tc := range cases {
		b.Run(tc.name, func(b *testing.B) {
			aggregate := benchmarkAggregate(0)
			seed := benchmarkEntries(b, aggregate, 1, tc.historySize, tc.payloadSize, "seed")
			candidate := benchmarkEntries(b, aggregate, tc.historySize+1, tc.batchSize, tc.payloadSize, "candidate")

			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				b.StopTimer()
				repo := NewMemoryRepository()
				if len(seed) > 0 {
					if err := repo.Append(ctx, seed...); err != nil {
						b.Fatalf("seed append: %v", err)
					}
				}

				b.StartTimer()
				err := repo.Append(ctx, candidate...)
				b.StopTimer()
				if err != nil {
					b.Fatalf("Append: %v", err)
				}
			}
		})
	}
}

func BenchmarkMemoryRepositoryFind(b *testing.B) {
	ctx := context.Background()
	cases := []struct {
		name           string
		aggregateCount int
		historySize    int
		payloadSize    int
		query          func(AggregateID) Query
		want           int
	}{
		{
			name:           "SingleAggregateHistory16",
			aggregateCount: 1,
			historySize:    16,
			payloadSize:    256,
			query: func(aggregate AggregateID) Query {
				return Query{Aggregate: &aggregate}
			},
			want: 16,
		},
		{
			name:           "TypeScan64AggregatesLimit32",
			aggregateCount: 64,
			historySize:    16,
			payloadSize:    256,
			query: func(AggregateID) Query {
				return Query{AggregateType: "account", NewestFirst: true, Limit: 32}
			},
			want: 32,
		},
	}

	for _, tc := range cases {
		b.Run(tc.name, func(b *testing.B) {
			repo := NewMemoryRepository()
			targetAggregate := seedBenchmarkRepository(b, repo, tc.aggregateCount, tc.historySize, tc.payloadSize)
			query := tc.query(targetAggregate)

			b.ReportAllocs()
			for b.Loop() {
				entries, err := repo.Find(ctx, query)
				if err != nil {
					b.Fatalf("Find: %v", err)
				}
				if len(entries) != tc.want {
					b.Fatalf("Find returned %d entries, want %d", len(entries), tc.want)
				}
			}
		})
	}
}

func BenchmarkMemoryRepositoryLoadHistory(b *testing.B) {
	ctx := context.Background()
	cases := []struct {
		name        string
		historySize int
		payloadSize int
	}{
		{name: "Small16/Payload256", historySize: 16, payloadSize: 256},
		{name: "Medium256/Payload2048", historySize: 256, payloadSize: 2048},
	}

	for _, tc := range cases {
		b.Run(tc.name, func(b *testing.B) {
			repo := NewMemoryRepository()
			aggregate := seedBenchmarkRepository(b, repo, 1, tc.historySize, tc.payloadSize)

			b.ReportAllocs()
			for b.Loop() {
				history, ok, err := repo.LoadHistory(ctx, aggregate)
				if err != nil {
					b.Fatalf("LoadHistory: %v", err)
				}
				if !ok || len(history.Entries()) != tc.historySize {
					b.Fatalf("LoadHistory ok=%v len=%d, want %d", ok, len(history.Entries()), tc.historySize)
				}
			}
		})
	}
}

func BenchmarkAuditEntryJSONRoundTrip(b *testing.B) {
	cases := []struct {
		name        string
		payloadSize int
	}{
		{name: "Payload256", payloadSize: 256},
		{name: "Payload2048", payloadSize: 2048},
	}

	for _, tc := range cases {
		b.Run(tc.name, func(b *testing.B) {
			entry := benchmarkEntry(b, benchmarkAggregate(0), 1, tc.payloadSize, "json")

			b.ReportAllocs()
			for b.Loop() {
				encoded, err := json.Marshal(entry)
				if err != nil {
					b.Fatalf("marshal: %v", err)
				}
				decoded, err := DecodeEntryJSON(encoded)
				if err != nil {
					b.Fatalf("DecodeEntryJSON: %v", err)
				}
				if decoded.Event.EventID != entry.Event.EventID {
					b.Fatalf("decoded event ID = %q, want %q", decoded.Event.EventID, entry.Event.EventID)
				}
			}
		})
	}
}

func seedBenchmarkRepository(tb testing.TB, repo *MemoryRepository, aggregateCount int, historySize int, payloadSize int) AggregateID {
	tb.Helper()

	var target AggregateID
	for aggregateIndex := 0; aggregateIndex < aggregateCount; aggregateIndex++ {
		aggregate := benchmarkAggregate(aggregateIndex)
		if aggregateIndex == 0 {
			target = aggregate
		}
		entries := benchmarkEntries(tb, aggregate, 1, historySize, payloadSize, "repo")
		if err := repo.Append(context.Background(), entries...); err != nil {
			tb.Fatalf("Append seed aggregate %d: %v", aggregateIndex, err)
		}
	}
	return target
}

func benchmarkEntries(tb testing.TB, aggregate AggregateID, startRevision int, count int, payloadSize int, prefix string) []Entry {
	tb.Helper()

	entries := make([]Entry, count)
	for i := range entries {
		entries[i] = benchmarkEntry(tb, aggregate, startRevision+i, payloadSize, fmt.Sprintf("%s-%d", prefix, i))
	}
	return entries
}

func benchmarkEntry(tb testing.TB, aggregate AggregateID, revision int, payloadSize int, suffix string) Entry {
	tb.Helper()

	event, err := NewDomainEvent(EventOptions{
		EventID:        EventID(fmt.Sprintf("%s-event-%s-%d", aggregate.ID, suffix, revision)),
		EventType:      EventType("AccountChanged"),
		AggregateID:    aggregate,
		Revision:       Revision(revision),
		OccurredAt:     auditBenchmarkClock.Add(time.Duration(revision) * time.Second),
		RecordedAt:     auditBenchmarkClock.Add(time.Duration(revision) * time.Second),
		IdempotencyKey: fmt.Sprintf("%s-idem-%s-%d", aggregate.ID, suffix, revision),
		Metadata:       Metadata{"source": "benchmark"},
		Payload:        benchmarkPayload(payloadSize),
	})
	if err != nil {
		tb.Fatalf("NewDomainEvent: %v", err)
	}
	entry, err := NewEntry(EntryOptions{
		Author: "benchmark",
		Event:  event,
	})
	if err != nil {
		tb.Fatalf("NewEntry: %v", err)
	}
	return entry
}

func benchmarkAggregate(index int) AggregateID {
	return AggregateID{Type: "account", ID: fmt.Sprintf("account-%04d", index)}
}

func benchmarkPayload(size int) json.RawMessage {
	if size <= len(`{"blob":""}`) {
		return json.RawMessage(`{}`)
	}
	return json.RawMessage(fmt.Sprintf(`{"blob":"%s"}`, strings.Repeat("x", size-len(`{"blob":""}`))))
}
