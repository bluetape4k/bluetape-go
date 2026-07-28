// Package audittest는 bluetape-go의 audittest audit/outbox 기능을 제공한다.
// 공개 API 주석은 transaction, idempotency, repository ownership, delivery, 오류 계약을 한국어로 확인할 수 있도록 유지한다.
package audittest

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/bluetape4k/bluetape-go/audit"
)

// RepositoryFactory는 func 공개 타입이며 audit conformance harness의 repository/recorder 계약을 보존한다.
// 필드와 zero value, nil 허용 여부, 동시성/transaction 소유권은 생성자와 메서드의 한국어 주석 및 테스트 계약을 따른다.
type RepositoryFactory func(testing.TB) audit.Repository

// RunRepositoryConformance는 RunRepositoryConformance 공개 API의 동작을 수행하며 audit conformance harness의 repository/recorder 계약을 보존한다.
//
// 매개변수:
//   - t: RunRepositoryConformance 동작에 필요한 t 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//   - factory: 감사 대상 actor, action, resource, metadata 값이다. 빈 값과 민감정보 처리는 audit 계약을 따른다.
func RunRepositoryConformance(t *testing.T, factory RepositoryFactory) {
	t.Helper()
	t.Run("append load and query", func(t *testing.T) {
		repo := factory(t)
		ctx := context.Background()
		aggregate := mustAggregate(t, "account", "42")
		entries := []audit.Entry{
			mustEntry(t, aggregate, 1, "event-1", "idem-1", false),
			mustEntry(t, aggregate, 2, "event-2", "idem-2", true),
			mustEntry(t, aggregate, 3, "event-3", "idem-3", false),
		}
		if err := repo.Append(ctx, entries...); err != nil {
			t.Fatalf("Append: %v", err)
		}
		history, ok, err := repo.LoadHistory(ctx, aggregate)
		if err != nil || !ok {
			t.Fatalf("LoadHistory ok=%v err=%v", ok, err)
		}
		if history.HeadRevision() != audit.Revision(3) {
			t.Fatalf("head revision = %d", history.HeadRevision())
		}
		found, err := repo.Find(ctx, audit.Query{Aggregate: &aggregate, FromRevision: audit.Revision(2), ToRevision: audit.Revision(3)})
		if err != nil {
			t.Fatalf("Find: %v", err)
		}
		if got := eventIDs(found); !reflect.DeepEqual(got, []audit.EventID{"event-2", "event-3"}) {
			t.Fatalf("found IDs = %#v", got)
		}
		newest, err := repo.Find(ctx, audit.Query{AggregateType: " account ", NewestFirst: true, Limit: 2})
		if err != nil {
			t.Fatalf("Find newest by type: %v", err)
		}
		if got := eventIDs(newest); !reflect.DeepEqual(got, []audit.EventID{"event-3", "event-2"}) {
			t.Fatalf("newest IDs = %#v", got)
		}
		window, err := repo.Find(ctx, audit.Query{
			FromRecordedAt: fixedTime().Add(1500 * time.Millisecond),
			ToRecordedAt:   fixedTime().Add(2500 * time.Millisecond),
		})
		if err != nil {
			t.Fatalf("Find recorded window: %v", err)
		}
		if got := eventIDs(window); !reflect.DeepEqual(got, []audit.EventID{"event-2"}) {
			t.Fatalf("window IDs = %#v", got)
		}
		latest, ok, err := repo.Latest(ctx, aggregate)
		if err != nil || !ok {
			t.Fatalf("Latest ok=%v err=%v", ok, err)
		}
		if latest.Event.EventID != "event-3" {
			t.Fatalf("latest = %#v", latest.Event.EventID)
		}
		snapshot, ok, err := repo.LatestSnapshot(ctx, aggregate)
		if err != nil || !ok {
			t.Fatalf("LatestSnapshot ok=%v err=%v", ok, err)
		}
		if snapshot.Event.EventID != "event-2" || snapshot.Snapshot == nil {
			t.Fatalf("snapshot = %#v", snapshot)
		}
		previous, ok, err := repo.PreviousSnapshot(ctx, aggregate, audit.Revision(3))
		if err != nil || !ok {
			t.Fatalf("PreviousSnapshot ok=%v err=%v", ok, err)
		}
		if previous.Event.EventID != "event-2" {
			t.Fatalf("previous snapshot = %#v", previous.Event.EventID)
		}

		found[0].Event.Payload[0] = '['
		historyAgain, ok, err := repo.LoadHistory(ctx, aggregate)
		if err != nil || !ok {
			t.Fatalf("LoadHistory after mutation ok=%v err=%v", ok, err)
		}
		if string(historyAgain.Entries()[1].Event.Payload) != `{}` {
			t.Fatalf("repository returned mutable entries")
		}
	})

	t.Run("missing aggregate is not exceptional", func(t *testing.T) {
		repo := factory(t)
		ctx := context.Background()
		aggregate := mustAggregate(t, "account", "missing")
		if _, ok, err := repo.LoadHistory(ctx, aggregate); err != nil || ok {
			t.Fatalf("LoadHistory missing ok=%v err=%v", ok, err)
		}
		if _, ok, err := repo.Latest(ctx, aggregate); err != nil || ok {
			t.Fatalf("Latest missing ok=%v err=%v", ok, err)
		}
	})

	t.Run("append validation is all or nothing", func(t *testing.T) {
		repo := factory(t)
		ctx := context.Background()
		aggregate := mustAggregate(t, "account", "42")
		if err := repo.Append(ctx, mustEntry(t, aggregate, 1, "event-1", "idem-1", false)); err != nil {
			t.Fatalf("Append first: %v", err)
		}
		err := repo.Append(ctx, mustEntry(t, aggregate, 3, "event-3", "idem-3", false))
		if !errors.Is(err, audit.ErrRevisionConflict) {
			t.Fatalf("Append gap err=%v, want ErrRevisionConflict", err)
		}
		history, ok, err := repo.LoadHistory(ctx, aggregate)
		if err != nil || !ok {
			t.Fatalf("LoadHistory ok=%v err=%v", ok, err)
		}
		if history.HeadRevision() != audit.InitialRevision() {
			t.Fatalf("failed append mutated head=%d", history.HeadRevision())
		}
		if err := repo.Append(ctx, mustEntry(t, aggregate, 2, "event-1", "idem-2", false)); !errors.Is(err, audit.ErrRevisionConflict) {
			t.Fatalf("duplicate event id err=%v, want ErrRevisionConflict", err)
		}
		if err := repo.Append(ctx, mustEntry(t, aggregate, 2, "event-2", "idem-1", false)); !errors.Is(err, audit.ErrRevisionConflict) {
			t.Fatalf("duplicate idempotency key err=%v, want ErrRevisionConflict", err)
		}
	})
}

func mustAggregate(t *testing.T, typ string, id string) audit.AggregateID {
	t.Helper()
	aggregate, err := audit.NewAggregateID(typ, id)
	if err != nil {
		t.Fatalf("NewAggregateID: %v", err)
	}
	return aggregate
}

func mustEntry(t *testing.T, aggregate audit.AggregateID, revision uint64, eventID string, idempotencyKey string, snapshot bool) audit.Entry {
	t.Helper()
	event, err := audit.NewDomainEvent(audit.EventOptions{
		EventID:        audit.EventID(eventID),
		EventType:      audit.EventType("AccountChanged"),
		AggregateID:    aggregate,
		Revision:       audit.Revision(revision),
		OccurredAt:     fixedTime(),
		RecordedAt:     fixedTime().Add(time.Duration(revision) * time.Second),
		IdempotencyKey: idempotencyKey,
		Payload:        json.RawMessage(`{}`),
	})
	if err != nil {
		t.Fatalf("NewDomainEvent: %v", err)
	}
	options := audit.EntryOptions{
		Author: "audittest",
		Event:  event,
	}
	if snapshot {
		options.Snapshot = &audit.SnapshotMetadata{
			Format:        "json",
			SchemaVersion: "v1",
			Payload:       json.RawMessage(`{"ok":true}`),
		}
	}
	entry, err := audit.NewEntry(options)
	if err != nil {
		t.Fatalf("NewEntry: %v", err)
	}
	return entry
}

func eventIDs(entries []audit.Entry) []audit.EventID {
	ids := make([]audit.EventID, len(entries))
	for i, entry := range entries {
		ids[i] = entry.Event.EventID
	}
	return ids
}

func fixedTime() time.Time {
	return time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
}
