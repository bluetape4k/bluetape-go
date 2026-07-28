package audit

import (
	"encoding/json"
	"strings"
	"time"
)

// EventID는 string 공개 타입이며 audit entry, event, repository, recorder, history 계약을 보존한다.
// 필드와 zero value, nil 허용 여부, 동시성/transaction 소유권은 생성자와 메서드의 한국어 주석 및 테스트 계약을 따른다.
type EventID string

// EventType는 string 공개 타입이며 audit entry, event, repository, recorder, history 계약을 보존한다.
// 필드와 zero value, nil 허용 여부, 동시성/transaction 소유권은 생성자와 메서드의 한국어 주석 및 테스트 계약을 따른다.
type EventType string

// EventOptions는 struct 공개 타입이며 audit entry, event, repository, recorder, history 계약을 보존한다.
// 필드와 zero value, nil 허용 여부, 동시성/transaction 소유권은 생성자와 메서드의 한국어 주석 및 테스트 계약을 따른다.
type EventOptions struct {
	EventID        EventID
	EventType      EventType
	AggregateID    AggregateID
	Revision       Revision
	OccurredAt     time.Time
	RecordedAt     time.Time
	IdempotencyKey string
	Metadata       Metadata
	Payload        json.RawMessage
}

// DomainEvent는 struct 공개 타입이며 audit entry, event, repository, recorder, history 계약을 보존한다.
// 필드와 zero value, nil 허용 여부, 동시성/transaction 소유권은 생성자와 메서드의 한국어 주석 및 테스트 계약을 따른다.
type DomainEvent struct {
	EventID        EventID         `json:"event_id"`
	EventType      EventType       `json:"event_type"`
	Aggregate      AggregateID     `json:"aggregate"`
	Revision       Revision        `json:"revision"`
	OccurredAt     time.Time       `json:"occurred_at"`
	RecordedAt     time.Time       `json:"recorded_at"`
	IdempotencyKey string          `json:"idempotency_key"`
	Metadata       Metadata        `json:"metadata,omitempty"`
	Payload        json.RawMessage `json:"payload"`
}

// NewDomainEvent는 NewDomainEvent 공개 API의 동작을 수행하며 audit entry, event, repository, recorder, history 계약을 보존한다.
//
// 매개변수:
//   - options: NewDomainEvent 동작에 필요한 options 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//
// 반환 오류는 입력 검증 실패, 취소, transaction 실패, repository/outbox 실패, 또는 package sentinel/typed error 계약을 보존한다.
func NewDomainEvent(options EventOptions) (DomainEvent, error) {
	event := DomainEvent{
		EventID:        EventID(strings.TrimSpace(string(options.EventID))),
		EventType:      EventType(strings.TrimSpace(string(options.EventType))),
		Aggregate:      options.AggregateID,
		Revision:       options.Revision,
		OccurredAt:     options.OccurredAt,
		RecordedAt:     options.RecordedAt,
		IdempotencyKey: strings.TrimSpace(options.IdempotencyKey),
		Metadata:       options.Metadata.Clone(),
	}
	payload, err := normalizePayload("payload", options.Payload, ErrInvalidEvent)
	if err != nil {
		return DomainEvent{}, err
	}
	event.Payload = payload
	if err := event.Validate(); err != nil {
		return DomainEvent{}, err
	}
	return event, nil
}

// Validate는 Validate 공개 API의 동작을 수행하며 audit entry, event, repository, recorder, history 계약을 보존한다.
//
// 반환 오류는 입력 검증 실패, 취소, transaction 실패, repository/outbox 실패, 또는 package sentinel/typed error 계약을 보존한다.
func (e DomainEvent) Validate() error {
	if strings.TrimSpace(string(e.EventID)) == "" {
		return validationError(ErrInvalidEvent, "event_id", e.EventID)
	}
	if strings.TrimSpace(string(e.EventType)) == "" {
		return validationError(ErrInvalidEvent, "event_type", e.EventType)
	}
	if err := e.Aggregate.Validate(); err != nil {
		return validationCause(ErrInvalidEvent, "aggregate", e.Aggregate, err)
	}
	if err := e.Revision.Validate(); err != nil {
		return validationCause(ErrInvalidEvent, "revision", e.Revision, err)
	}
	if e.OccurredAt.IsZero() {
		return validationError(ErrInvalidEvent, "occurred_at", e.OccurredAt)
	}
	if e.RecordedAt.IsZero() {
		return validationError(ErrInvalidEvent, "recorded_at", e.RecordedAt)
	}
	if strings.TrimSpace(e.IdempotencyKey) == "" {
		return validationError(ErrInvalidEvent, "idempotency_key", e.IdempotencyKey)
	}
	if err := validateMetadata(ErrInvalidEvent, "metadata", e.Metadata); err != nil {
		return validationCause(ErrInvalidEvent, "metadata", e.Metadata, err)
	}
	if len(e.Payload) == 0 || !json.Valid(e.Payload) {
		return validationError(ErrInvalidEvent, "payload", string(e.Payload))
	}
	return nil
}

// Clone는 Clone 공개 API의 동작을 수행하며 audit entry, event, repository, recorder, history 계약을 보존한다.
func (e DomainEvent) Clone() DomainEvent {
	clone := e
	clone.Metadata = e.Metadata.Clone()
	clone.Payload = cloneRawMessage(e.Payload)
	return clone
}

// UnmarshalJSON는 UnmarshalJSON 공개 API의 동작을 수행하며 audit entry, event, repository, recorder, history 계약을 보존한다.
//
// 매개변수:
//   - data: UnmarshalJSON 동작에 필요한 data 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//
// 반환 오류는 입력 검증 실패, 취소, transaction 실패, repository/outbox 실패, 또는 package sentinel/typed error 계약을 보존한다.
func (e *DomainEvent) UnmarshalJSON(data []byte) error {
	type domainEvent DomainEvent
	var decoded domainEvent
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	value, err := NewDomainEvent(EventOptions{
		EventID:        decoded.EventID,
		EventType:      decoded.EventType,
		AggregateID:    decoded.Aggregate,
		Revision:       decoded.Revision,
		OccurredAt:     decoded.OccurredAt,
		RecordedAt:     decoded.RecordedAt,
		IdempotencyKey: decoded.IdempotencyKey,
		Metadata:       decoded.Metadata,
		Payload:        decoded.Payload,
	})
	if err != nil {
		return err
	}
	*e = value
	return nil
}
