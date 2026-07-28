package audit

import (
	"encoding/json"
	"strings"
	"time"
)

// EventID string 공개 타입이며 audit entry, event, repository, recorder, history 계약을 보존한다.
// 필드와 zero value, nil 허용 여부, 동시성/transaction 소유권은 생성자와 메서드의 한국어 주석 및 테스트 계약을 따른다.
type EventID string

// EventType string 공개 타입이며 audit entry, event, repository, recorder, history 계약을 보존한다.
// 필드와 zero value, nil 허용 여부, 동시성/transaction 소유권은 생성자와 메서드의 한국어 주석 및 테스트 계약을 따른다.
type EventType string

// EventOptions struct 공개 타입이며 audit entry, event, repository, recorder, history 계약을 보존한다.
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

// DomainEvent struct 공개 타입이며 audit entry, event, repository, recorder, history 계약을 보존한다.
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

// NewDomainEvent NewDomainEvent 공개 API의 동작을 수행하며 audit entry, event, repository, recorder, history 계약을 보존한다.
//
// 매개변수:
//   - options: 적용할 옵션 목록이다. nil이면 기본값만 사용한다.
//
// 반환 오류는 입력 검증 실패, context 취소, transaction 실패, repository/outbox 실패, package sentinel error와 typed error를 그대로 드러낸다.
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

// Validate Validate 공개 API의 동작을 수행하며 audit entry, event, repository, recorder, history 계약을 보존한다.
//
// 반환 오류는 입력 검증 실패, context 취소, transaction 실패, repository/outbox 실패, package sentinel error와 typed error를 그대로 드러낸다.
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

// Clone Clone 공개 API의 동작을 수행하며 audit entry, event, repository, recorder, history 계약을 보존한다.
func (e DomainEvent) Clone() DomainEvent {
	clone := e
	clone.Metadata = e.Metadata.Clone()
	clone.Payload = cloneRawMessage(e.Payload)
	return clone
}

// UnmarshalJSON UnmarshalJSON 공개 API의 동작을 수행하며 audit entry, event, repository, recorder, history 계약을 보존한다.
//
// 매개변수:
//   - data: UnmarshalJSON에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
//
// 반환 오류는 입력 검증 실패, context 취소, transaction 실패, repository/outbox 실패, package sentinel error와 typed error를 그대로 드러낸다.
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
