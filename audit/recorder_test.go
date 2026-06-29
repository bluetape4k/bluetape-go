package audit

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"sync/atomic"
	"testing"

	concurrencytest "github.com/bluetape4k/bluetape-go/testing/concurrency"
)

func TestAggregateRecorderConcurrentRecordPendingAndAck(t *testing.T) {
	aggregate := mustAggregateID(t)
	recorder, err := NewAggregateRecorder(aggregate)
	if err != nil {
		t.Fatalf("NewAggregateRecorder: %v", err)
	}

	stress := concurrencytest.NewGoroutineStressTester(concurrencytest.Options{
		Workers:       8,
		RoundsPerTask: 32,
	})
	var nextID int64

	stress.RunT(t,
		func(context.Context) error {
			id := atomic.AddInt64(&nextID, 1)
			_, err := recorder.Record(EventRecord{
				EventID:        EventID(fmt.Sprintf("event-%d", id)),
				EventType:      EventType("AccountChanged"),
				OccurredAt:     fixedTime(),
				IdempotencyKey: fmt.Sprintf("idem-%d", id),
				Metadata:       Metadata{"actor": "tester"},
				Payload:        json.RawMessage(`{"ok":true}`),
			})
			if err != nil {
				return err
			}
			return nil
		},
		func(context.Context) error {
			for _, event := range recorder.PendingEvents() {
				if event.Aggregate != aggregate {
					return fmt.Errorf("pending aggregate mismatch: %#v", event)
				}
			}
			return nil
		},
		func(context.Context) error {
			head := recorder.HeadRevision()
			if head > 1 {
				return recorder.AckThrough(head - 1)
			}
			return nil
		},
	)

	pending := recorder.PendingEvents()
	if !sort.SliceIsSorted(pending, func(i, j int) bool {
		return pending[i].Revision < pending[j].Revision
	}) {
		t.Fatalf("pending events are not sorted by revision: %#v", pending)
	}
	for i := 1; i < len(pending); i++ {
		if pending[i].Revision <= pending[i-1].Revision {
			t.Fatalf("duplicate or regressed revision in pending: %#v", pending)
		}
	}
}

func BenchmarkAggregateRecorderRecord(b *testing.B) {
	aggregate, err := NewAggregateID("account", "42")
	if err != nil {
		b.Fatal(err)
	}
	recorder, err := NewAggregateRecorder(aggregate)
	if err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if _, err := recorder.Record(EventRecord{
			EventID:        EventID(fmt.Sprintf("event-%d", i)),
			EventType:      EventType("AccountChanged"),
			OccurredAt:     fixedTime(),
			IdempotencyKey: fmt.Sprintf("idem-%d", i),
			Payload:        json.RawMessage(`{}`),
		}); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkAggregateRecorderPendingEvents(b *testing.B) {
	recorder := seededRecorder(b, 128)
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = recorder.PendingEvents()
	}
}

func BenchmarkAggregateRecorderAckThroughCompactsSurvivors(b *testing.B) {
	for i := 0; i < b.N; i++ {
		recorder := seededRecorder(b, 128)
		if err := recorder.AckThrough(Revision(64)); err != nil {
			b.Fatal(err)
		}
	}
}

func seededRecorder(tb testing.TB, count int) *AggregateRecorder {
	tb.Helper()
	aggregate, err := NewAggregateID("account", "42")
	if err != nil {
		tb.Fatal(err)
	}
	recorder, err := NewAggregateRecorder(aggregate)
	if err != nil {
		tb.Fatal(err)
	}
	for i := 0; i < count; i++ {
		if _, err := recorder.Record(EventRecord{
			EventID:        EventID(fmt.Sprintf("event-%d", i)),
			EventType:      EventType("AccountChanged"),
			OccurredAt:     fixedTime(),
			IdempotencyKey: fmt.Sprintf("idem-%d", i),
			Payload:        json.RawMessage(`{}`),
		}); err != nil {
			tb.Fatal(err)
		}
	}
	return recorder
}
