package audit

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"testing"
	"time"

	concurrencytest "github.com/bluetape4k/bluetape-go/testing/concurrency"
)

func TestQueryValidateNormalizesAndRejectsInvalidRanges(t *testing.T) {
	aggregate := mustAggregateID(t)
	query := Query{
		Aggregate:      &aggregate,
		AggregateType:  " account ",
		FromRevision:   InitialRevision(),
		ToRevision:     Revision(3),
		FromRecordedAt: fixedTime(),
		ToRecordedAt:   fixedTime().Add(time.Second),
		Limit:          10,
	}
	normalized, err := query.Validate()
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if normalized.Aggregate == nil || *normalized.Aggregate != aggregate {
		t.Fatalf("aggregate was not preserved: %#v", normalized.Aggregate)
	}
	if normalized.Aggregate == query.Aggregate {
		t.Fatalf("aggregate pointer should be copied")
	}
	if normalized.AggregateType != "account" {
		t.Fatalf("aggregate type was not trimmed: %q", normalized.AggregateType)
	}

	cases := []struct {
		name  string
		query Query
	}{
		{name: "invalid aggregate", query: Query{Aggregate: &AggregateID{Type: "account"}}},
		{name: "to revision before from revision", query: Query{FromRevision: Revision(2), ToRevision: Revision(1)}},
		{name: "time range inverted", query: Query{FromRecordedAt: fixedTime().Add(time.Second), ToRecordedAt: fixedTime()}},
		{name: "negative limit", query: Query{Limit: -1}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := tc.query.Validate(); !errors.Is(err, ErrInvalidQuery) {
				t.Fatalf("Validate err = %v, want ErrInvalidQuery", err)
			}
		})
	}
}

func TestMemoryRepositoryAppendLoadHistoryAndDefensiveCopies(t *testing.T) {
	ctx := context.Background()
	repo := NewMemoryRepository()
	first := mustEntry(t, 1, "event-1", "idem-1")
	second := mustSnapshotEntry(t, mustAggregateID(t), 2, "event-2", "idem-2")

	if err := repo.Append(ctx, first, second); err != nil {
		t.Fatalf("Append: %v", err)
	}
	history, ok, err := repo.LoadHistory(ctx, mustAggregateID(t))
	if err != nil {
		t.Fatalf("LoadHistory: %v", err)
	}
	if !ok {
		t.Fatal("LoadHistory should find appended aggregate")
	}
	if history.HeadRevision() != Revision(2) {
		t.Fatalf("head revision = %d", history.HeadRevision())
	}

	entries := history.Entries()
	entries[0].Event.Payload[0] = '['
	entries[0].Event.Metadata["actor"] = "mutated"
	again, ok, err := repo.LoadHistory(ctx, mustAggregateID(t))
	if err != nil || !ok {
		t.Fatalf("LoadHistory again ok=%v err=%v", ok, err)
	}
	if string(again.Entries()[0].Event.Payload) != `{}` || again.Entries()[0].Event.Metadata["actor"] != "tester" {
		t.Fatalf("repository returned mutable entries: %#v", again.Entries()[0])
	}

	_, ok, err = repo.LoadHistory(ctx, mustOtherAggregateID(t))
	if err != nil {
		t.Fatalf("missing LoadHistory err = %v", err)
	}
	if ok {
		t.Fatal("missing aggregate should return ok=false")
	}
}

func TestMemoryRepositoryAppendValidationIsAllOrNothing(t *testing.T) {
	ctx := context.Background()
	repo := NewMemoryRepository()
	first := mustEntry(t, 1, "event-1", "idem-1")
	if err := repo.Append(ctx, first); err != nil {
		t.Fatalf("Append first: %v", err)
	}

	cases := []struct {
		name    string
		entries []Entry
		want    error
	}{
		{name: "mixed aggregate batch", entries: []Entry{mustEntry(t, 2, "event-2", "idem-2"), mustEntryForAggregate(t, mustOtherAggregateID(t), 1, "other-1", "other-idem-1")}, want: ErrMixedAggregate},
		{name: "non contiguous continuation", entries: []Entry{mustEntry(t, 3, "event-3", "idem-3")}, want: ErrRevisionConflict},
		{name: "duplicate event id", entries: []Entry{mustEntry(t, 2, "event-1", "idem-2")}, want: ErrRevisionConflict},
		{name: "duplicate idempotency key", entries: []Entry{mustEntry(t, 2, "event-2", "idem-1")}, want: ErrRevisionConflict},
		{name: "invalid entry", entries: []Entry{{SchemaVersion: SchemaVersion}}, want: ErrInvalidEntry},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := repo.Append(ctx, tc.entries...); !errors.Is(err, tc.want) {
				t.Fatalf("Append err = %v, want %v", err, tc.want)
			}
			history, ok, err := repo.LoadHistory(ctx, mustAggregateID(t))
			if err != nil || !ok {
				t.Fatalf("LoadHistory after failed append ok=%v err=%v", ok, err)
			}
			if got := history.HeadRevision(); got != InitialRevision() {
				t.Fatalf("failed append mutated history head=%d", got)
			}
		})
	}

	if err := repo.Append(ctx, mustEntryForAggregate(t, mustOtherAggregateID(t), 1, "other-event-1", "idem-1")); !errors.Is(err, ErrRevisionConflict) {
		t.Fatalf("cross aggregate duplicate idempotency key err = %v, want ErrRevisionConflict", err)
	}
}

func TestMemoryRepositoryFindFiltersOrdersAndLimits(t *testing.T) {
	ctx := context.Background()
	repo := NewMemoryRepository()
	account := mustAggregateID(t)
	otherAccount := mustOtherAggregateID(t)
	order, err := NewAggregateID("order", "900")
	if err != nil {
		t.Fatalf("NewAggregateID: %v", err)
	}
	entries := []Entry{
		mustEntryAt(t, account, 1, "account-1", "account-idem-1", fixedTime().Add(time.Second)),
		mustEntryAt(t, otherAccount, 1, "other-account-1", "other-account-idem-1", fixedTime().Add(2*time.Second)),
		mustEntryAt(t, account, 2, "account-2", "account-idem-2", fixedTime().Add(3*time.Second)),
		mustEntryAt(t, order, 1, "order-1", "order-idem-1", fixedTime().Add(4*time.Second)),
		mustSnapshotEntryAt(t, account, 3, "account-3", "account-idem-3", fixedTime().Add(5*time.Second)),
	}
	if err := repo.Append(ctx, entries[0]); err != nil {
		t.Fatalf("Append account first: %v", err)
	}
	if err := repo.Append(ctx, entries[1]); err != nil {
		t.Fatalf("Append other account: %v", err)
	}
	if err := repo.Append(ctx, entries[2]); err != nil {
		t.Fatalf("Append account second: %v", err)
	}
	if err := repo.Append(ctx, entries[3]); err != nil {
		t.Fatalf("Append order: %v", err)
	}
	if err := repo.Append(ctx, entries[4]); err != nil {
		t.Fatalf("Append account snapshot: %v", err)
	}

	all, err := repo.Find(ctx, Query{})
	if err != nil {
		t.Fatalf("Find all: %v", err)
	}
	if got := eventIDs(all); !reflect.DeepEqual(got, []EventID{"account-1", "other-account-1", "account-2", "order-1", "account-3"}) {
		t.Fatalf("append order IDs = %#v", got)
	}

	accountEntries, err := repo.Find(ctx, Query{Aggregate: &account, FromRevision: Revision(2), ToRevision: Revision(3)})
	if err != nil {
		t.Fatalf("Find account revisions: %v", err)
	}
	if got := eventIDs(accountEntries); !reflect.DeepEqual(got, []EventID{"account-2", "account-3"}) {
		t.Fatalf("account revision IDs = %#v", got)
	}

	accountsNewest, err := repo.Find(ctx, Query{AggregateType: " account ", NewestFirst: true, Limit: 2})
	if err != nil {
		t.Fatalf("Find account newest: %v", err)
	}
	if got := eventIDs(accountsNewest); !reflect.DeepEqual(got, []EventID{"account-3", "account-2"}) {
		t.Fatalf("newest account IDs = %#v", got)
	}

	window, err := repo.Find(ctx, Query{FromRecordedAt: fixedTime().Add(2500 * time.Millisecond), ToRecordedAt: fixedTime().Add(4500 * time.Millisecond)})
	if err != nil {
		t.Fatalf("Find recorded window: %v", err)
	}
	if got := eventIDs(window); !reflect.DeepEqual(got, []EventID{"account-2", "order-1"}) {
		t.Fatalf("recorded window IDs = %#v", got)
	}
}

func TestMemoryRepositoryLatestAndSnapshots(t *testing.T) {
	ctx := context.Background()
	repo := NewMemoryRepository()
	aggregate := mustAggregateID(t)
	entries := []Entry{
		mustEntry(t, 1, "event-1", "idem-1"),
		mustSnapshotEntry(t, aggregate, 2, "event-2", "idem-2"),
		mustEntry(t, 3, "event-3", "idem-3"),
		mustSnapshotEntry(t, aggregate, 4, "event-4", "idem-4"),
	}
	if err := repo.Append(ctx, entries...); err != nil {
		t.Fatalf("Append: %v", err)
	}

	latest, ok, err := repo.Latest(ctx, aggregate)
	if err != nil || !ok {
		t.Fatalf("Latest ok=%v err=%v", ok, err)
	}
	if latest.Revision != Revision(4) {
		t.Fatalf("latest revision = %d", latest.Revision)
	}
	snapshot, ok, err := repo.LatestSnapshot(ctx, aggregate)
	if err != nil || !ok {
		t.Fatalf("LatestSnapshot ok=%v err=%v", ok, err)
	}
	if snapshot.Revision != Revision(4) || snapshot.Snapshot == nil {
		t.Fatalf("latest snapshot = %#v", snapshot)
	}
	previous, ok, err := repo.PreviousSnapshot(ctx, aggregate, Revision(4))
	if err != nil || !ok {
		t.Fatalf("PreviousSnapshot ok=%v err=%v", ok, err)
	}
	if previous.Revision != Revision(2) || previous.Snapshot == nil {
		t.Fatalf("previous snapshot = %#v", previous)
	}

	missing, ok, err := repo.Latest(ctx, mustOtherAggregateID(t))
	if err != nil {
		t.Fatalf("missing Latest err = %v", err)
	}
	if ok || missing.Event.EventID != "" {
		t.Fatalf("missing Latest should return zero entry and ok=false: ok=%v entry=%#v", ok, missing)
	}
}

func TestMemoryRepositoryContextCancellation(t *testing.T) {
	repo := NewMemoryRepository()
	entry := mustEntry(t, 1, "event-1", "idem-1")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if err := repo.Append(ctx, entry); !errors.Is(err, context.Canceled) {
		t.Fatalf("Append canceled err = %v", err)
	}
	if _, err := repo.Find(ctx, Query{}); !errors.Is(err, context.Canceled) {
		t.Fatalf("Find canceled err = %v", err)
	}
	if _, _, err := repo.LoadHistory(ctx, mustAggregateID(t)); !errors.Is(err, context.Canceled) {
		t.Fatalf("LoadHistory canceled err = %v", err)
	}
	if _, _, err := repo.Latest(ctx, mustAggregateID(t)); !errors.Is(err, context.Canceled) {
		t.Fatalf("Latest canceled err = %v", err)
	}
}

func TestMemoryRepositoryConcurrentAppendAndFind(t *testing.T) {
	repo := NewMemoryRepository()
	stress := concurrencytest.NewGoroutineStressTester(concurrencytest.Options{
		Workers:       8,
		RoundsPerTask: 16,
	})
	stress.RunT(t,
		func(ctx context.Context) error {
			aggregate, err := NewAggregateID("account", "concurrent")
			if err != nil {
				return err
			}
			latest, ok, err := repo.Latest(ctx, aggregate)
			if err != nil {
				return err
			}
			next := uint64(1)
			if ok {
				next = uint64(latest.Revision) + 1
			}
			entry := mustEntryForAggregateNoFatal(aggregate, next, fmt.Sprintf("event-%d", next), fmt.Sprintf("idem-%d", next))
			err = repo.Append(ctx, entry)
			if err != nil && !errors.Is(err, ErrRevisionConflict) {
				return err
			}
			return nil
		},
		func(ctx context.Context) error {
			entries, err := repo.Find(ctx, Query{AggregateType: "account", NewestFirst: true, Limit: 4})
			if err != nil {
				return err
			}
			for _, entry := range entries {
				if entry.Aggregate.Type != "account" {
					return fmt.Errorf("unexpected aggregate type: %#v", entry.Aggregate)
				}
			}
			return nil
		},
	)
}

func eventIDs(entries []Entry) []EventID {
	ids := make([]EventID, len(entries))
	for i, entry := range entries {
		ids[i] = entry.Event.EventID
	}
	return ids
}

func mustEntryAt(t *testing.T, aggregate AggregateID, revision uint64, eventID string, idempotencyKey string, recordedAt time.Time) Entry {
	t.Helper()
	event, err := NewDomainEvent(EventOptions{
		EventID:        EventID(eventID),
		EventType:      EventType("AccountChanged"),
		AggregateID:    aggregate,
		Revision:       Revision(revision),
		OccurredAt:     fixedTime(),
		RecordedAt:     recordedAt,
		IdempotencyKey: idempotencyKey,
		Metadata:       Metadata{"actor": "tester"},
		Payload:        json.RawMessage(`{}`),
	})
	if err != nil {
		t.Fatalf("NewDomainEvent: %v", err)
	}
	entry, err := NewEntry(EntryOptions{
		Author: "tester",
		Event:  event,
		Change: &ChangeMetadata{
			ChangedFields: []string{"balance"},
			Summary:       "updated",
			Attributes:    Metadata{"source": "test"},
		},
	})
	if err != nil {
		t.Fatalf("NewEntry: %v", err)
	}
	return entry
}

func mustSnapshotEntry(t *testing.T, aggregate AggregateID, revision uint64, eventID string, idempotencyKey string) Entry {
	t.Helper()
	return mustSnapshotEntryAt(t, aggregate, revision, eventID, idempotencyKey, fixedTime().Add(time.Second))
}

func mustSnapshotEntryAt(t *testing.T, aggregate AggregateID, revision uint64, eventID string, idempotencyKey string, recordedAt time.Time) Entry {
	t.Helper()
	entry := mustEntryAt(t, aggregate, revision, eventID, idempotencyKey, recordedAt)
	snapshot := SnapshotMetadata{
		Format:        "json",
		SchemaVersion: "v1",
		Payload:       json.RawMessage(`{"balance":"10.00"}`),
	}
	entry.Snapshot = &snapshot
	if err := entry.Validate(); err != nil {
		t.Fatalf("snapshot entry Validate: %v", err)
	}
	return entry
}

func mustEntryForAggregateNoFatal(aggregate AggregateID, revision uint64, eventID string, idempotencyKey string) Entry {
	event, err := NewDomainEvent(EventOptions{
		EventID:        EventID(eventID),
		EventType:      EventType("AccountChanged"),
		AggregateID:    aggregate,
		Revision:       Revision(revision),
		OccurredAt:     fixedTime(),
		RecordedAt:     fixedTime().Add(time.Duration(revision) * time.Second),
		IdempotencyKey: idempotencyKey,
		Metadata:       Metadata{"actor": "tester"},
		Payload:        json.RawMessage(`{}`),
	})
	if err != nil {
		panic(err)
	}
	entry, err := NewEntry(EntryOptions{Author: "tester", Event: event})
	if err != nil {
		panic(err)
	}
	return entry
}
