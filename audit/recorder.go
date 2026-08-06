package audit

import (
	"encoding/json"
	"strings"
	"sync"
	"time"
)

// EventRecord audit entry, event, repository, recorder, history에서 사용하는 구조체다.
type EventRecord struct {
	EventID        EventID
	EventType      EventType
	OccurredAt     time.Time
	IdempotencyKey string
	Metadata       Metadata
	Payload        json.RawMessage
}

// AggregateRecorder audit entry, event, repository, recorder, history에서 사용하는 구조체다.
type AggregateRecorder struct {
	mu        sync.Mutex
	aggregate AggregateID
	head      Revision
	pending   []DomainEvent
}

// NewAggregateRecorder audit entry, event, repository, recorder, history에 사용할 값을 생성한다.
//
// 매개변수:
//   - aggregate: NewAggregateRecorder에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
//
// 반환 오류는 입력 검증 실패, context 취소, transaction 실패, repository/outbox 실패, package sentinel error와 typed error를 그대로 드러낸다.
func NewAggregateRecorder(aggregate AggregateID) (*AggregateRecorder, error) {
	return NewAggregateRecorderFromHead(aggregate, 0)
}

// NewAggregateRecorderFromHead audit entry, event, repository, recorder, history에 사용할 값을 생성한다.
//
// 매개변수:
//   - aggregate: NewAggregateRecorderFromHead에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
//   - head: NewAggregateRecorderFromHead에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
//
// 반환 오류는 입력 검증 실패, context 취소, transaction 실패, repository/outbox 실패, package sentinel error와 typed error를 그대로 드러낸다.
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

// Record audit entry, event, repository, recorder, history의 쓰기 동작을 수행한다.
//
// 매개변수:
//   - record: Record에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
//
// 반환 오류는 입력 검증 실패, context 취소, transaction 실패, repository/outbox 실패, package sentinel error와 typed error를 그대로 드러낸다.
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

// PendingEvents audit entry, event, repository, recorder, history에서 필요한 값을 조회한다.
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

// AckThrough audit entry, event, repository, recorder, history의 상태를 변경한다.
//
// 매개변수:
//   - revision: AckThrough에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
//
// 반환 오류는 입력 검증 실패, context 취소, transaction 실패, repository/outbox 실패, package sentinel error와 typed error를 그대로 드러낸다.
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

// HeadRevision audit entry, event, repository, recorder, history에서 필요한 값을 조회한다.
func (r *AggregateRecorder) HeadRevision() Revision {
	if r == nil {
		return 0
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.head
}
