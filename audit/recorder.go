package audit

import (
	"encoding/json"
	"strings"
	"sync"
	"time"
)

// EventRecord contains caller inputs for AggregateRecorder.Record.
type EventRecord struct {
	EventID        EventID
	EventType      EventType
	OccurredAt     time.Time
	IdempotencyKey string
	Metadata       Metadata
	Payload        json.RawMessage
}

// AggregateRecorder records pending events for one aggregate root.
type AggregateRecorder struct {
	mu        sync.Mutex
	aggregate AggregateID
	head      Revision
	pending   []DomainEvent
}

// NewAggregateRecorder creates a recorder for an aggregate with no history.
func NewAggregateRecorder(aggregate AggregateID) (*AggregateRecorder, error) {
	return NewAggregateRecorderFromHead(aggregate, 0)
}

// NewAggregateRecorderFromHead creates a recorder restored from a durable head.
func NewAggregateRecorderFromHead(aggregate AggregateID, head Revision) (*AggregateRecorder, error) {
	if err := aggregate.Validate(); err != nil {
		return nil, err
	}
	if head != 0 {
		if err := head.Validate(); err != nil {
			return nil, err
		}
	}
	return &AggregateRecorder{aggregate: aggregate, head: head}, nil
}

// Record validates and records a pending event with the next revision.
func (r *AggregateRecorder) Record(record EventRecord) (DomainEvent, error) {
	if r == nil {
		return DomainEvent{}, validationError(ErrInvalidEvent, "recorder", nil)
	}
	eventID := EventID(strings.TrimSpace(string(record.EventID)))
	eventType := EventType(strings.TrimSpace(string(record.EventType)))
	idempotencyKey := strings.TrimSpace(record.IdempotencyKey)
	metadata := record.Metadata.Clone()
	payload, err := normalizePayload("payload", record.Payload, ErrInvalidEvent)
	if err != nil {
		return DomainEvent{}, err
	}
	if eventID == "" {
		return DomainEvent{}, validationError(ErrInvalidEvent, "event_id", record.EventID)
	}
	if eventType == "" {
		return DomainEvent{}, validationError(ErrInvalidEvent, "event_type", record.EventType)
	}
	if record.OccurredAt.IsZero() {
		return DomainEvent{}, validationError(ErrInvalidEvent, "occurred_at", record.OccurredAt)
	}
	if idempotencyKey == "" {
		return DomainEvent{}, validationError(ErrInvalidEvent, "idempotency_key", record.IdempotencyKey)
	}
	if err := validateMetadata(ErrInvalidEvent, "metadata", metadata); err != nil {
		return DomainEvent{}, validationCause(ErrInvalidEvent, "metadata", metadata, err)
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	next := InitialRevision()
	if r.head != 0 {
		var err error
		next, err = r.head.Next()
		if err != nil {
			return DomainEvent{}, err
		}
	}
	event, err := NewDomainEvent(EventOptions{
		EventID:        eventID,
		EventType:      eventType,
		AggregateID:    r.aggregate,
		Revision:       next,
		OccurredAt:     record.OccurredAt,
		RecordedAt:     time.Now().UTC(),
		IdempotencyKey: idempotencyKey,
		Metadata:       metadata,
		Payload:        payload,
	})
	if err != nil {
		return DomainEvent{}, err
	}
	r.head = next
	r.pending = append(r.pending, event.Clone())
	return event.Clone(), nil
}

// PendingEvents returns a defensive snapshot of unacknowledged events.
func (r *AggregateRecorder) PendingEvents() []DomainEvent {
	if r == nil {
		return nil
	}
	r.mu.Lock()
	snapshot := append([]DomainEvent(nil), r.pending...)
	r.mu.Unlock()
	for i := range snapshot {
		snapshot[i] = snapshot[i].Clone()
	}
	return snapshot
}

// AckThrough acknowledges all pending events at or below revision.
func (r *AggregateRecorder) AckThrough(revision Revision) error {
	if r == nil {
		return validationError(ErrInvalidRevision, "recorder", nil)
	}
	if err := revision.Validate(); err != nil {
		return err
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if revision > r.head {
		return validationError(ErrInvalidRevision, "revision", revision)
	}
	keep := 0
	for keep < len(r.pending) && r.pending[keep].Revision <= revision {
		r.pending[keep] = DomainEvent{}
		keep++
	}
	if keep == 0 {
		return nil
	}
	survivors := make([]DomainEvent, len(r.pending)-keep)
	copy(survivors, r.pending[keep:])
	r.pending = survivors
	return nil
}

// HeadRevision returns the current in-memory head revision.
func (r *AggregateRecorder) HeadRevision() Revision {
	if r == nil {
		return 0
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.head
}
