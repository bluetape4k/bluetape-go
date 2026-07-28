package audit

import (
	"encoding/json"
	"math"
	"strings"
)

// AggregateID는 struct 공개 타입이며 audit entry, event, repository, recorder, history 계약을 보존한다.
// 필드와 zero value, nil 허용 여부, 동시성/transaction 소유권은 생성자와 메서드의 한국어 주석 및 테스트 계약을 따른다.
type AggregateID struct {
	Type string `json:"type"`
	ID   string `json:"id"`
}

// NewAggregateID는 NewAggregateID 공개 API의 동작을 수행하며 audit entry, event, repository, recorder, history 계약을 보존한다.
//
// 매개변수:
//   - typ: NewAggregateID 동작에 필요한 typ 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//   - id: audit event 또는 entry 식별자다. uniqueness와 idempotency 의미는 repository 계약을 따른다.
//
// 반환 오류는 입력 검증 실패, 취소, transaction 실패, repository/outbox 실패, 또는 package sentinel/typed error 계약을 보존한다.
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

// Validate는 Validate 공개 API의 동작을 수행하며 audit entry, event, repository, recorder, history 계약을 보존한다.
//
// 반환 오류는 입력 검증 실패, 취소, transaction 실패, repository/outbox 실패, 또는 package sentinel/typed error 계약을 보존한다.
func (id AggregateID) Validate() error {
	if strings.TrimSpace(id.Type) == "" {
		return validationError(ErrInvalidAggregateID, "type", id.Type)
	}
	if strings.TrimSpace(id.ID) == "" {
		return validationError(ErrInvalidAggregateID, "id", id.ID)
	}
	return nil
}

// String는 String 공개 API의 동작을 수행하며 audit entry, event, repository, recorder, history 계약을 보존한다.
func (id AggregateID) String() string {
	return id.Type + ":" + id.ID
}

// UnmarshalJSON는 UnmarshalJSON 공개 API의 동작을 수행하며 audit entry, event, repository, recorder, history 계약을 보존한다.
//
// 매개변수:
//   - data: UnmarshalJSON 동작에 필요한 data 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//
// 반환 오류는 입력 검증 실패, 취소, transaction 실패, repository/outbox 실패, 또는 package sentinel/typed error 계약을 보존한다.
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

// Revision는 uint64 공개 타입이며 audit entry, event, repository, recorder, history 계약을 보존한다.
// 필드와 zero value, nil 허용 여부, 동시성/transaction 소유권은 생성자와 메서드의 한국어 주석 및 테스트 계약을 따른다.
type Revision uint64

// InitialRevision는 InitialRevision 공개 API의 동작을 수행하며 audit entry, event, repository, recorder, history 계약을 보존한다.
func InitialRevision() Revision {
	return Revision(1)
}

// Validate는 Validate 공개 API의 동작을 수행하며 audit entry, event, repository, recorder, history 계약을 보존한다.
//
// 반환 오류는 입력 검증 실패, 취소, transaction 실패, repository/outbox 실패, 또는 package sentinel/typed error 계약을 보존한다.
func (r Revision) Validate() error {
	if r == 0 {
		return validationError(ErrInvalidRevision, "revision", r)
	}
	return nil
}

// Next는 Next 공개 API의 동작을 수행하며 audit entry, event, repository, recorder, history 계약을 보존한다.
//
// 반환 오류는 입력 검증 실패, 취소, transaction 실패, repository/outbox 실패, 또는 package sentinel/typed error 계약을 보존한다.
func (r Revision) Next() (Revision, error) {
	if err := r.Validate(); err != nil {
		return 0, err
	}
	if uint64(r) == math.MaxUint64 {
		return 0, validationError(ErrInvalidRevision, "revision", r)
	}
	return r + 1, nil
}

// Metadata는 map[string]string 공개 타입이며 audit entry, event, repository, recorder, history 계약을 보존한다.
// 필드와 zero value, nil 허용 여부, 동시성/transaction 소유권은 생성자와 메서드의 한국어 주석 및 테스트 계약을 따른다.
type Metadata map[string]string

// Clone는 Clone 공개 API의 동작을 수행하며 audit entry, event, repository, recorder, history 계약을 보존한다.
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
