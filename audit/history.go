package audit

import "sort"

// History struct 공개 타입이며 audit entry, event, repository, recorder, history 계약을 보존한다.
// 필드와 zero value, nil 허용 여부, 동시성/transaction 소유권은 생성자와 메서드의 한국어 주석 및 테스트 계약을 따른다.
type History struct {
	aggregate AggregateID
	entries   []Entry
	head      Revision
}

// NewHistory NewHistory 공개 API의 동작을 수행하며 audit entry, event, repository, recorder, history 계약을 보존한다.
//
// 매개변수:
//   - entries: NewHistory 동작에 필요한 entries 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//
// 반환 오류는 입력 검증 실패, 취소, transaction 실패, repository/outbox 실패, 또는 package sentinel/typed error 계약을 보존한다.
func NewHistory(entries []Entry) (History, error) {
	if len(entries) == 0 {
		return History{}, validationError(ErrInvalidEntry, "entries", len(entries))
	}

	ordered := make([]Entry, len(entries))
	for i, entry := range entries {
		ordered[i] = entry.Clone()
	}
	sort.Slice(ordered, func(i, j int) bool {
		return ordered[i].Revision < ordered[j].Revision
	})

	aggregate := ordered[0].Aggregate
	if err := aggregate.Validate(); err != nil {
		return History{}, err
	}
	seenEventIDs := make(map[EventID]struct{}, len(ordered))
	seenIdempotencyKeys := make(map[string]EventID, len(ordered))

	for i, entry := range ordered {
		if err := entry.Validate(); err != nil {
			return History{}, err
		}
		if entry.Aggregate != aggregate {
			return History{}, validationError(ErrMixedAggregate, "aggregate", entry.Aggregate)
		}
		wantRevision := Revision(i + 1)
		if entry.Revision != wantRevision || entry.Revision != entry.Event.Revision {
			return History{}, validationError(ErrRevisionConflict, "revision", entry.Revision)
		}
		if _, ok := seenEventIDs[entry.Event.EventID]; ok {
			return History{}, validationError(ErrRevisionConflict, "event_id", entry.Event.EventID)
		}
		seenEventIDs[entry.Event.EventID] = struct{}{}
		if existingEventID, ok := seenIdempotencyKeys[entry.Event.IdempotencyKey]; ok && existingEventID != entry.Event.EventID {
			return History{}, validationError(ErrRevisionConflict, "idempotency_key", entry.Event.IdempotencyKey)
		}
		seenIdempotencyKeys[entry.Event.IdempotencyKey] = entry.Event.EventID
	}

	return History{
		aggregate: aggregate,
		entries:   ordered,
		head:      ordered[len(ordered)-1].Revision,
	}, nil
}

// AggregateID AggregateID 공개 API의 동작을 수행하며 audit entry, event, repository, recorder, history 계약을 보존한다.
func (h History) AggregateID() AggregateID {
	return h.aggregate
}

// HeadRevision HeadRevision 공개 API의 동작을 수행하며 audit entry, event, repository, recorder, history 계약을 보존한다.
func (h History) HeadRevision() Revision {
	return h.head
}

// Entries Entries 공개 API의 동작을 수행하며 audit entry, event, repository, recorder, history 계약을 보존한다.
func (h History) Entries() []Entry {
	if len(h.entries) == 0 {
		return nil
	}
	entries := make([]Entry, len(h.entries))
	for i, entry := range h.entries {
		entries[i] = entry.Clone()
	}
	return entries
}
