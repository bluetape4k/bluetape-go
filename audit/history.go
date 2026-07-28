package audit

import "sort"

// History audit entry, event, repository, recorder, history에서 사용하는 구조체다.
type History struct {
	aggregate AggregateID
	entries   []Entry
	head      Revision
}

// NewHistory audit entry, event, repository, recorder, history에 사용할 값을 생성한다.
//
// 매개변수:
//   - entries: NewHistory에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
//
// 반환 오류는 입력 검증 실패, context 취소, transaction 실패, repository/outbox 실패, package sentinel error와 typed error를 그대로 드러낸다.
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

// AggregateID audit entry, event, repository, recorder, history에서 필요한 값을 조회한다.
func (h History) AggregateID() AggregateID {
	return h.aggregate
}

// HeadRevision audit entry, event, repository, recorder, history에서 필요한 값을 조회한다.
func (h History) HeadRevision() Revision {
	return h.head
}

// Entries audit entry, event, repository, recorder, history에서 필요한 값을 조회한다.
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
