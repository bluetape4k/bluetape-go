package audit

import (
	"encoding/json"
	"errors"
	"math"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"
)

func TestAggregateIDAndRevisionValidation(t *testing.T) {
	aggregate, err := NewAggregateID(" account ", " 42 ")
	if err != nil {
		t.Fatalf("NewAggregateID: %v", err)
	}
	if aggregate.Type != "account" || aggregate.ID != "42" {
		t.Fatalf("aggregate should be trimmed, got %#v", aggregate)
	}
	if aggregate.String() != "account:42" {
		t.Fatalf("aggregate String() = %q", aggregate.String())
	}

	if _, err := NewAggregateID("", "42"); !errors.Is(err, ErrInvalidAggregateID) {
		t.Fatalf("blank type err = %v", err)
	}
	if err := Revision(0).Validate(); !errors.Is(err, ErrInvalidRevision) {
		t.Fatalf("zero revision err = %v", err)
	}
	next, err := InitialRevision().Next()
	if err != nil {
		t.Fatalf("InitialRevision.Next: %v", err)
	}
	if next != Revision(2) {
		t.Fatalf("next revision = %d", next)
	}
	if _, err := Revision(math.MaxUint64).Next(); !errors.Is(err, ErrInvalidRevision) {
		t.Fatalf("max revision next err = %v", err)
	}
}

func TestDomainEventJSONValidationAndCopies(t *testing.T) {
	aggregate := mustAggregateID(t)
	payload := json.RawMessage(`{"name":"debit"}`)
	metadata := Metadata{"actor": "system"}
	event, err := NewDomainEvent(EventOptions{
		EventID:        EventID("event-1"),
		EventType:      EventType("AccountDebited"),
		AggregateID:    aggregate,
		Revision:       InitialRevision(),
		OccurredAt:     fixedTime(),
		RecordedAt:     fixedTime().Add(time.Millisecond),
		IdempotencyKey: "idem-1",
		Metadata:       metadata,
		Payload:        payload,
	})
	if err != nil {
		t.Fatalf("NewDomainEvent: %v", err)
	}

	metadata["actor"] = "mutated"
	payload[2] = 'x'
	if event.Metadata["actor"] != "system" {
		t.Fatalf("metadata was not copied: %#v", event.Metadata)
	}
	if string(event.Payload) != `{"name":"debit"}` {
		t.Fatalf("payload was not copied: %s", event.Payload)
	}

	encoded, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var decoded DomainEvent
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("unmarshal valid event: %v", err)
	}
	if decoded.EventID != event.EventID || decoded.Aggregate != aggregate {
		t.Fatalf("decoded mismatch: %#v", decoded)
	}

	invalid := []byte(`{"event_id":"","event_type":"AccountDebited","aggregate":{"type":"account","id":"42"},"revision":1,"occurred_at":"2026-01-02T03:04:05Z","recorded_at":"2026-01-02T03:04:06Z","idempotency_key":"idem-1","payload":{}}`)
	if err := json.Unmarshal(invalid, &decoded); !errors.Is(err, ErrInvalidEvent) {
		t.Fatalf("invalid event unmarshal err = %v", err)
	}

	canonical := []byte(`{"event_id":" event-2 ","event_type":" AccountCredited ","aggregate":{"type":" account ","id":" 42 "},"revision":1,"occurred_at":"2026-01-02T03:04:05Z","recorded_at":"2026-01-02T03:04:06Z","idempotency_key":" idem-2 ","payload":{}}`)
	if err := json.Unmarshal(canonical, &decoded); err != nil {
		t.Fatalf("unmarshal canonical event: %v", err)
	}
	if decoded.EventID != "event-2" || decoded.EventType != "AccountCredited" || decoded.Aggregate.String() != "account:42" || decoded.IdempotencyKey != "idem-2" {
		t.Fatalf("decoded event was not canonicalized: %#v", decoded)
	}

	_, err = NewDomainEvent(EventOptions{
		EventID:        EventID("event-3"),
		EventType:      EventType("AccountChanged"),
		AggregateID:    aggregate,
		Revision:       InitialRevision(),
		OccurredAt:     fixedTime(),
		RecordedAt:     fixedTime().Add(time.Millisecond),
		IdempotencyKey: "idem-3",
		Metadata:       Metadata{" ": "blank-key"},
		Payload:        json.RawMessage(`{}`),
	})
	if !errors.Is(err, ErrInvalidEvent) {
		t.Fatalf("metadata err should be invalid event: %v", err)
	}
	if errors.Is(err, ErrInvalidEntry) {
		t.Fatalf("metadata err should not be invalid entry: %v", err)
	}
}

func TestEntryDecodeValidatesNestedContracts(t *testing.T) {
	entry := mustEntry(t, 1, "event-1", "idem-1")
	encoded, err := json.Marshal(entry)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	decoded, err := DecodeEntryJSON(encoded)
	if err != nil {
		t.Fatalf("DecodeEntryJSON: %v", err)
	}
	if decoded.SchemaVersion != SchemaVersion || decoded.Event.EventID != EventID("event-1") {
		t.Fatalf("decoded mismatch: %#v", decoded)
	}

	var direct Entry
	invalid := []byte(`{"schema_version":2,"aggregate":{"type":"account","id":"42"},"revision":1,"author":"tester","event":{"event_id":"event-1","event_type":"AccountChanged","aggregate":{"type":"account","id":"42"},"revision":1,"occurred_at":"2026-01-02T03:04:05Z","recorded_at":"2026-01-02T03:04:06Z","idempotency_key":"idem-1","payload":{}}}`)
	if err := json.Unmarshal(invalid, &direct); !errors.Is(err, ErrInvalidEntry) {
		t.Fatalf("unsupported schema version err = %v", err)
	}

	mismatched := []byte(`{"schema_version":1,"aggregate":{"type":"account","id":"42"},"revision":2,"author":"tester","event":{"event_id":"event-1","event_type":"AccountChanged","aggregate":{"type":"account","id":"42"},"revision":1,"occurred_at":"2026-01-02T03:04:05Z","recorded_at":"2026-01-02T03:04:06Z","idempotency_key":"idem-1","payload":{}}}`)
	if _, err := DecodeEntryJSON(mismatched); !errors.Is(err, ErrInvalidEntry) {
		t.Fatalf("mismatched revision err = %v", err)
	}

	canonical := []byte(`{"schema_version":1,"aggregate":{"type":" account ","id":" 42 "},"revision":1,"author":" tester ","event":{"event_id":" event-1 ","event_type":" AccountChanged ","aggregate":{"type":" account ","id":" 42 "},"revision":1,"occurred_at":"2026-01-02T03:04:05Z","recorded_at":"2026-01-02T03:04:06Z","idempotency_key":" idem-1 ","payload":{}},"snapshot":{"format":" json ","schema_version":" v1 ","payload":{}}}`)
	decoded, err = DecodeEntryJSON(canonical)
	if err != nil {
		t.Fatalf("DecodeEntryJSON canonical: %v", err)
	}
	if decoded.Author != "tester" || decoded.Aggregate.String() != "account:42" || decoded.Event.EventID != "event-1" || decoded.Snapshot.Format != "json" {
		t.Fatalf("decoded entry was not canonicalized: %#v", decoded)
	}

	missingSnapshotPayload := []byte(`{"schema_version":1,"aggregate":{"type":"account","id":"42"},"revision":1,"author":"tester","event":{"event_id":"event-1","event_type":"AccountChanged","aggregate":{"type":"account","id":"42"},"revision":1,"occurred_at":"2026-01-02T03:04:05Z","recorded_at":"2026-01-02T03:04:06Z","idempotency_key":"idem-1","payload":{}},"snapshot":{"format":"json","schema_version":"v1"}}`)
	if _, err := DecodeEntryJSON(missingSnapshotPayload); !errors.Is(err, ErrInvalidEntry) {
		t.Fatalf("missing snapshot payload err = %v", err)
	}

	_, err = NewEntry(EntryOptions{
		Author: "tester",
		Event:  entry.Event,
		Snapshot: &SnapshotMetadata{
			Format:        "json",
			SchemaVersion: "v1",
		},
	})
	if !errors.Is(err, ErrInvalidEntry) {
		t.Fatalf("constructor missing snapshot payload err = %v", err)
	}
}

func TestAggregateRecorderPendingAckAndNoMutationOnInvalidRecord(t *testing.T) {
	aggregate := mustAggregateID(t)
	recorder, err := NewAggregateRecorderFromHead(aggregate, Revision(math.MaxUint64))
	if err != nil {
		t.Fatalf("NewAggregateRecorderFromHead: %v", err)
	}
	if _, err := recorder.Record(EventRecord{
		EventID:        EventID("overflow"),
		EventType:      EventType("AccountChanged"),
		OccurredAt:     fixedTime(),
		IdempotencyKey: "idem-overflow",
		Payload:        json.RawMessage(`{}`),
	}); !errors.Is(err, ErrInvalidRevision) {
		t.Fatalf("overflow record err = %v", err)
	}
	if got := recorder.PendingEvents(); len(got) != 0 {
		t.Fatalf("overflow mutated pending: %#v", got)
	}

	recorder, err = NewAggregateRecorder(aggregate)
	if err != nil {
		t.Fatalf("NewAggregateRecorder: %v", err)
	}
	first := mustRecord(t, recorder, "event-1", "idem-1")
	second := mustRecord(t, recorder, "event-2", "idem-2")
	if first.Revision != InitialRevision() || second.Revision != Revision(2) {
		t.Fatalf("recorded revisions = %d, %d", first.Revision, second.Revision)
	}

	pending := recorder.PendingEvents()
	pending[0].Payload[0] = '['
	pending[0].Metadata["changed"] = "yes"
	again := recorder.PendingEvents()
	if string(again[0].Payload) != `{}` || again[0].Metadata["changed"] != "" {
		t.Fatalf("pending snapshot was mutable: %#v", again[0])
	}

	if err := recorder.AckThrough(Revision(1)); err != nil {
		t.Fatalf("AckThrough(1): %v", err)
	}
	remaining := recorder.PendingEvents()
	if len(remaining) != 1 || remaining[0].Revision != Revision(2) {
		t.Fatalf("remaining after ack = %#v", remaining)
	}
	if err := recorder.AckThrough(Revision(1)); err != nil {
		t.Fatalf("idempotent AckThrough(1): %v", err)
	}
	if err := recorder.AckThrough(Revision(3)); !errors.Is(err, ErrInvalidRevision) {
		t.Fatalf("future AckThrough err = %v", err)
	}
	if got := recorder.PendingEvents(); len(got) != 1 || got[0].Revision != Revision(2) {
		t.Fatalf("future ack mutated pending: %#v", got)
	}
}

func TestHistoryReconstructsAndRejectsBrokenStreams(t *testing.T) {
	second := mustEntry(t, 2, "event-2", "idem-2")
	first := mustEntry(t, 1, "event-1", "idem-1")
	history, err := NewHistory([]Entry{second, first})
	if err != nil {
		t.Fatalf("NewHistory: %v", err)
	}
	if history.AggregateID() != mustAggregateID(t) || history.HeadRevision() != Revision(2) {
		t.Fatalf("history state mismatch: aggregate=%#v head=%d", history.AggregateID(), history.HeadRevision())
	}
	entries := history.Entries()
	if len(entries) != 2 || entries[0].Revision != InitialRevision() || entries[1].Revision != Revision(2) {
		t.Fatalf("entries not ordered: %#v", entries)
	}
	entries[0].Event.Payload[0] = '['
	if string(history.Entries()[0].Event.Payload) != `{}` {
		t.Fatalf("history entries are mutable")
	}

	cases := []struct {
		name    string
		entries []Entry
		want    error
	}{
		{name: "nil", entries: nil, want: ErrInvalidEntry},
		{name: "empty", entries: []Entry{}, want: ErrInvalidEntry},
		{name: "gap at start", entries: []Entry{second}, want: ErrRevisionConflict},
		{name: "duplicate revision", entries: []Entry{first, first}, want: ErrRevisionConflict},
		{name: "duplicate event id", entries: []Entry{first, mustEntry(t, 2, "event-1", "idem-2")}, want: ErrRevisionConflict},
		{name: "duplicate idempotency key", entries: []Entry{first, mustEntry(t, 2, "event-2", "idem-1")}, want: ErrRevisionConflict},
		{name: "mixed aggregate", entries: []Entry{first, mustEntryForAggregate(t, mustOtherAggregateID(t), 2, "event-2", "idem-2")}, want: ErrMixedAggregate},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := NewHistory(tc.entries); !errors.Is(err, tc.want) {
				t.Fatalf("NewHistory err = %v, want %v", err, tc.want)
			}
		})
	}
}

func TestChangeMetadataNormalizesFields(t *testing.T) {
	change, err := NewChangeMetadata([]string{" name ", "status", "name"}, "updated", Metadata{"actor": "tester"})
	if err != nil {
		t.Fatalf("NewChangeMetadata: %v", err)
	}
	sort.Strings(change.ChangedFields)
	if !reflect.DeepEqual(change.ChangedFields, []string{"name", "status"}) {
		t.Fatalf("changed fields = %#v", change.ChangedFields)
	}
}

func TestValidationErrorsDoNotExposeSensitiveValues(t *testing.T) {
	_, err := NewDomainEvent(EventOptions{
		EventID:        EventID("event-1"),
		EventType:      EventType("AccountChanged"),
		AggregateID:    mustAggregateID(t),
		Revision:       InitialRevision(),
		OccurredAt:     fixedTime(),
		RecordedAt:     fixedTime().Add(time.Second),
		IdempotencyKey: "secret-idem-key",
		Metadata:       Metadata{"secret-token": "secret-value"},
		Payload:        json.RawMessage(`{"secret":"payload-secret"`),
	})
	if err == nil {
		t.Fatal("expected payload validation error")
	}
	message := err.Error()
	for _, sensitive := range []string{"secret-idem-key", "secret-token", "secret-value", "payload-secret"} {
		if strings.Contains(message, sensitive) {
			t.Fatalf("validation error leaked %q: %s", sensitive, message)
		}
	}

	_, err = DecodeEntryJSON([]byte(`{"schema_version":1,"aggregate":{"type":"account","id":"42"},"revision":2,"author":"secret-author","event":{"event_id":"event-1","event_type":"AccountChanged","aggregate":{"type":"account","id":"42"},"revision":1,"occurred_at":"2026-01-02T03:04:05Z","recorded_at":"2026-01-02T03:04:06Z","idempotency_key":"secret-idem-key","metadata":{"secret-token":"secret-value"},"payload":{"secret":"payload-secret"}}}`))
	if err == nil {
		t.Fatal("expected mismatched revision error")
	}
	message = err.Error()
	for _, sensitive := range []string{"secret-author", "secret-idem-key", "secret-token", "secret-value", "payload-secret"} {
		if strings.Contains(message, sensitive) {
			t.Fatalf("decode validation error leaked %q: %s", sensitive, message)
		}
	}
}

func mustAggregateID(t *testing.T) AggregateID {
	t.Helper()
	id, err := NewAggregateID("account", "42")
	if err != nil {
		t.Fatalf("NewAggregateID: %v", err)
	}
	return id
}

func mustOtherAggregateID(t *testing.T) AggregateID {
	t.Helper()
	id, err := NewAggregateID("account", "43")
	if err != nil {
		t.Fatalf("NewAggregateID: %v", err)
	}
	return id
}

func fixedTime() time.Time {
	return time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
}

func mustRecord(t *testing.T, recorder *AggregateRecorder, eventID string, idempotencyKey string) DomainEvent {
	t.Helper()
	event, err := recorder.Record(EventRecord{
		EventID:        EventID(eventID),
		EventType:      EventType("AccountChanged"),
		OccurredAt:     fixedTime(),
		IdempotencyKey: idempotencyKey,
		Metadata:       Metadata{"actor": "tester"},
		Payload:        json.RawMessage(`{}`),
	})
	if err != nil {
		t.Fatalf("Record: %v", err)
	}
	return event
}

func mustEntry(t *testing.T, revision uint64, eventID string, idempotencyKey string) Entry {
	t.Helper()
	return mustEntryForAggregate(t, mustAggregateID(t), revision, eventID, idempotencyKey)
}

func mustEntryForAggregate(t *testing.T, aggregate AggregateID, revision uint64, eventID string, idempotencyKey string) Entry {
	t.Helper()
	event, err := NewDomainEvent(EventOptions{
		EventID:        EventID(eventID),
		EventType:      EventType("AccountChanged"),
		AggregateID:    aggregate,
		Revision:       Revision(revision),
		OccurredAt:     fixedTime(),
		RecordedAt:     fixedTime().Add(time.Second),
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
