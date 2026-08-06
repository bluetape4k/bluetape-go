package audit

import (
	"encoding/json"
	"strings"
	"time"
)

// EventID audit entry, event, repository, recorder, history에서 사용하는 문자열 타입이다.
type EventID string

// EventType audit entry, event, repository, recorder, history에서 사용하는 문자열 타입이다.
type EventType string

// EventOptions audit entry, event, repository, recorder, history에서 사용하는 구조체다.
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

// DomainEvent audit entry, event, repository, recorder, history에서 사용하는 구조체다.
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

// NewDomainEvent audit entry, event, repository, recorder, history에 사용할 값을 생성한다.
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

// Validate 값이 audit entry, event, repository, recorder, history 규칙을 만족하는지 검사한다.
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

// Clone 값을 복사해 caller가 독립적으로 수정할 수 있게 한다.
func (e DomainEvent) Clone() DomainEvent {
	clone := e
	clone.Metadata = e.Metadata.Clone()
	clone.Payload = cloneRawMessage(e.Payload)
	return clone
}

// UnmarshalJSON JSON 표현을 현재 값으로 복원한다.
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
