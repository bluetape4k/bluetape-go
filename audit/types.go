package audit

import (
	"encoding/json"
	"math"
	"strings"
)

// AggregateID audit entry, event, repository, recorder, history에서 사용하는 구조체다.
type AggregateID struct {
	Type string `json:"type"`
	ID   string `json:"id"`
}

// NewAggregateID audit entry, event, repository, recorder, history에 사용할 값을 생성한다.
//
// 매개변수:
//   - typ: NewAggregateID에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
//   - id: audit event 또는 entry 식별자다. uniqueness와 idempotency 의미는 repository 계약을 따른다.
//
// 반환 오류는 입력 검증 실패, context 취소, transaction 실패, repository/outbox 실패, package sentinel error와 typed error를 그대로 드러낸다.
func NewAggregateID(typ string, id string) (AggregateID, error) {
	aggregate := AggregateID{
		Type: strings.TrimSpace(typ),
		ID:   strings.TrimSpace(id),
	}
	if err := aggregate.Validate(); err != nil {
		return AggregateID{}, err
	}
	return aggregate, nil
}

// Validate 값이 audit entry, event, repository, recorder, history 규칙을 만족하는지 검사한다.
//
// 반환 오류는 입력 검증 실패, context 취소, transaction 실패, repository/outbox 실패, package sentinel error와 typed error를 그대로 드러낸다.
func (id AggregateID) Validate() error {
	if strings.TrimSpace(id.Type) == "" {
		return validationError(ErrInvalidAggregateID, "type", id.Type)
	}
	if strings.TrimSpace(id.ID) == "" {
		return validationError(ErrInvalidAggregateID, "id", id.ID)
	}
	return nil
}

// String audit entry, event, repository, recorder, history의 식별 정보를 반환한다.
func (id AggregateID) String() string {
	return id.Type + ":" + id.ID
}

// UnmarshalJSON JSON 표현을 현재 값으로 복원한다.
//
// 매개변수:
//   - data: UnmarshalJSON에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
//
// 반환 오류는 입력 검증 실패, context 취소, transaction 실패, repository/outbox 실패, package sentinel error와 typed error를 그대로 드러낸다.
func (id *AggregateID) UnmarshalJSON(data []byte) error {
	type aggregateID AggregateID
	var decoded aggregateID
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	value, err := NewAggregateID(decoded.Type, decoded.ID)
	if err != nil {
		return err
	}
	*id = value
	return nil
}

// Revision uint64 공개 타입이며 audit entry, event, repository, recorder, history 계약을 보존한다.
type Revision uint64

// InitialRevision audit entry, event, repository, recorder, history 동작을 수행한다.
func InitialRevision() Revision {
	return Revision(1)
}

// Validate 값이 audit entry, event, repository, recorder, history 규칙을 만족하는지 검사한다.
//
// 반환 오류는 입력 검증 실패, context 취소, transaction 실패, repository/outbox 실패, package sentinel error와 typed error를 그대로 드러낸다.
func (r Revision) Validate() error {
	if r == 0 {
		return validationError(ErrInvalidRevision, "revision", r)
	}
	return nil
}

// Next audit entry, event, repository, recorder, history 동작을 수행한다.
//
// 반환 오류는 입력 검증 실패, context 취소, transaction 실패, repository/outbox 실패, package sentinel error와 typed error를 그대로 드러낸다.
func (r Revision) Next() (Revision, error) {
	if err := r.Validate(); err != nil {
		return 0, err
	}
	if uint64(r) == math.MaxUint64 {
		return 0, validationError(ErrInvalidRevision, "revision", r)
	}
	return r + 1, nil
}

// Metadata map[string]string 공개 타입이며 audit entry, event, repository, recorder, history 계약을 보존한다.
type Metadata map[string]string

// Clone 값을 복사해 caller가 독립적으로 수정할 수 있게 한다.
func (m Metadata) Clone() Metadata {
	if len(m) == 0 {
		return nil
	}
	clone := make(Metadata, len(m))
	for key, value := range m {
		clone[key] = value
	}
	return clone
}

func validateMetadata(kind error, field string, metadata Metadata) error {
	for key := range metadata {
		if strings.TrimSpace(key) == "" {
			return validationError(kind, field, key)
		}
	}
	return nil
}

func cloneRawMessage(payload json.RawMessage) json.RawMessage {
	if len(payload) == 0 {
		return nil
	}
	clone := make(json.RawMessage, len(payload))
	copy(clone, payload)
	return clone
}

func normalizePayload(field string, payload json.RawMessage, kind error) (json.RawMessage, error) {
	clone := payload
	if len(clone) == 0 {
		clone = json.RawMessage(`{}`)
	}
	clone = cloneRawMessage(clone)
	if !json.Valid(clone) {
		return nil, validationError(kind, field, string(clone))
	}
	return clone, nil
}
