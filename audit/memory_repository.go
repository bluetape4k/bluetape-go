package audit

import (
	"context"
	"sync"
)

// MemoryRepository struct 공개 타입이며 audit entry, event, repository, recorder, history 계약을 보존한다.
// 필드와 zero value, nil 허용 여부, 동시성/transaction 소유권은 생성자와 메서드의 한국어 주석 및 테스트 계약을 따른다.
type MemoryRepository struct {
	mu      sync.RWMutex
	entries []Entry
}

// NewMemoryRepository NewMemoryRepository 공개 API의 동작을 수행하며 audit entry, event, repository, recorder, history 계약을 보존한다.
func NewMemoryRepository() *MemoryRepository {
	return &MemoryRepository{}
}

// Append Append 공개 API의 동작을 수행하며 audit entry, event, repository, recorder, history 계약을 보존한다.
//
// 매개변수:
//   - ctx: 호출자가 소유한 취소, deadline, 요청 범위를 전달한다.
//   - entries: Append에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
//
// 반환 오류는 입력 검증 실패, context 취소, transaction 실패, repository/outbox 실패, package sentinel error와 typed error를 그대로 드러낸다.
func (r *MemoryRepository) Append(ctx context.Context, entries ...Entry) error {
	ctx = normalizeContext(ctx)
	if err := checkContext(ctx); err != nil {
		return err
	}
	if len(entries) == 0 {
		return nil
	}

	clones := make([]Entry, len(entries))
	for i, entry := range entries {
		clone := entry.Clone()
		if err := clone.Validate(); err != nil {
			return validationCause(ErrInvalidEntry, "entries", i, err)
		}
		clones[i] = clone
	}
	if err := validateAppendBatch(clones); err != nil {
		return err
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	if err := checkContext(ctx); err != nil {
		return err
	}
	if err := r.validateContinuationLocked(clones); err != nil {
		return err
	}
	for _, entry := range clones {
		r.entries = append(r.entries, entry.Clone())
	}
	return nil
}

// Find Find 공개 API의 동작을 수행하며 audit entry, event, repository, recorder, history 계약을 보존한다.
//
// 매개변수:
//   - ctx: 호출자가 소유한 취소, deadline, 요청 범위를 전달한다.
//   - query: Find에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
//
// 반환 오류는 입력 검증 실패, context 취소, transaction 실패, repository/outbox 실패, package sentinel error와 typed error를 그대로 드러낸다.
func (r *MemoryRepository) Find(ctx context.Context, query Query) ([]Entry, error) {
	ctx = normalizeContext(ctx)
	if err := checkContext(ctx); err != nil {
		return nil, err
	}
	normalized, err := query.Validate()
	if err != nil {
		return nil, err
	}

	r.mu.RLock()
	defer r.mu.RUnlock()
	if err := checkContext(ctx); err != nil {
		return nil, err
	}

	matched := make([]Entry, 0)
	for i := range r.entries {
		entry := r.entries[i]
		if !matchesQuery(entry, normalized) {
			continue
		}
		matched = append(matched, entry.Clone())
	}
	if normalized.NewestFirst {
		reverseEntries(matched)
	}
	if normalized.Limit > 0 && len(matched) > normalized.Limit {
		matched = matched[:normalized.Limit]
	}
	return matched, nil
}

// LoadHistory LoadHistory 공개 API의 동작을 수행하며 audit entry, event, repository, recorder, history 계약을 보존한다.
//
// 매개변수:
//   - ctx: 호출자가 소유한 취소, deadline, 요청 범위를 전달한다.
//   - aggregate: LoadHistory에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
//
// 반환 오류는 입력 검증 실패, context 취소, transaction 실패, repository/outbox 실패, package sentinel error와 typed error를 그대로 드러낸다.
func (r *MemoryRepository) LoadHistory(ctx context.Context, aggregate AggregateID) (History, bool, error) {
	if err := aggregate.Validate(); err != nil {
		return History{}, false, validationCause(ErrInvalidQuery, "aggregate", aggregate, err)
	}
	entries, err := r.Find(ctx, Query{Aggregate: &aggregate})
	if err != nil {
		return History{}, false, err
	}
	if len(entries) == 0 {
		return History{}, false, nil
	}
	history, err := NewHistory(entries)
	if err != nil {
		return History{}, false, err
	}
	return history, true, nil
}

// Latest Latest 공개 API의 동작을 수행하며 audit entry, event, repository, recorder, history 계약을 보존한다.
//
// 매개변수:
//   - ctx: 호출자가 소유한 취소, deadline, 요청 범위를 전달한다.
//   - aggregate: Latest에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
//
// 반환 오류는 입력 검증 실패, context 취소, transaction 실패, repository/outbox 실패, package sentinel error와 typed error를 그대로 드러낸다.
func (r *MemoryRepository) Latest(ctx context.Context, aggregate AggregateID) (Entry, bool, error) {
	if err := aggregate.Validate(); err != nil {
		return Entry{}, false, validationCause(ErrInvalidQuery, "aggregate", aggregate, err)
	}
	entries, err := r.Find(ctx, Query{Aggregate: &aggregate, NewestFirst: true, Limit: 1})
	if err != nil {
		return Entry{}, false, err
	}
	if len(entries) == 0 {
		return Entry{}, false, nil
	}
	return entries[0], true, nil
}

// LatestSnapshot LatestSnapshot 공개 API의 동작을 수행하며 audit entry, event, repository, recorder, history 계약을 보존한다.
//
// 매개변수:
//   - ctx: 호출자가 소유한 취소, deadline, 요청 범위를 전달한다.
//   - aggregate: LatestSnapshot에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
//
// 반환 오류는 입력 검증 실패, context 취소, transaction 실패, repository/outbox 실패, package sentinel error와 typed error를 그대로 드러낸다.
func (r *MemoryRepository) LatestSnapshot(ctx context.Context, aggregate AggregateID) (Entry, bool, error) {
	return r.findSnapshot(ctx, aggregate, 0)
}

// PreviousSnapshot PreviousSnapshot 공개 API의 동작을 수행하며 audit entry, event, repository, recorder, history 계약을 보존한다.
//
// 매개변수:
//   - ctx: 호출자가 소유한 취소, deadline, 요청 범위를 전달한다.
//   - aggregate: PreviousSnapshot에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
//   - before: PreviousSnapshot에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
//
// 반환 오류는 입력 검증 실패, context 취소, transaction 실패, repository/outbox 실패, package sentinel error와 typed error를 그대로 드러낸다.
func (r *MemoryRepository) PreviousSnapshot(ctx context.Context, aggregate AggregateID, before Revision) (Entry, bool, error) {
	if err := before.Validate(); err != nil {
		return Entry{}, false, validationCause(ErrInvalidQuery, "before", before, err)
	}
	return r.findSnapshot(ctx, aggregate, before)
}

func (r *MemoryRepository) findSnapshot(ctx context.Context, aggregate AggregateID, before Revision) (Entry, bool, error) {
	if err := aggregate.Validate(); err != nil {
		return Entry{}, false, validationCause(ErrInvalidQuery, "aggregate", aggregate, err)
	}
	entries, err := r.Find(ctx, Query{Aggregate: &aggregate, NewestFirst: true})
	if err != nil {
		return Entry{}, false, err
	}
	for _, entry := range entries {
		if before != 0 && entry.Revision >= before {
			continue
		}
		if entry.Snapshot != nil {
			return entry, true, nil
		}
	}
	return Entry{}, false, nil
}

func validateAppendBatch(entries []Entry) error {
	aggregate := entries[0].Aggregate
	seenEventIDs := make(map[EventID]struct{}, len(entries))
	seenIdempotencyKeys := make(map[string]EventID, len(entries))
	for i, entry := range entries {
		if entry.Aggregate != aggregate {
			return validationError(ErrMixedAggregate, "aggregate", entry.Aggregate)
		}
		if i > 0 && entry.Revision != entries[i-1].Revision+1 {
			return validationError(ErrRevisionConflict, "revision", entry.Revision)
		}
		if _, ok := seenEventIDs[entry.Event.EventID]; ok {
			return validationError(ErrRevisionConflict, "event_id", entry.Event.EventID)
		}
		seenEventIDs[entry.Event.EventID] = struct{}{}
		if existingEventID, ok := seenIdempotencyKeys[entry.Event.IdempotencyKey]; ok && existingEventID != entry.Event.EventID {
			return validationError(ErrRevisionConflict, "idempotency_key", entry.Event.IdempotencyKey)
		}
		seenIdempotencyKeys[entry.Event.IdempotencyKey] = entry.Event.EventID
	}
	return nil
}

func (r *MemoryRepository) validateContinuationLocked(entries []Entry) error {
	aggregate := entries[0].Aggregate
	existing := make([]Entry, 0)
	existingEventIDs := make(map[EventID]struct{}, len(r.entries))
	existingIdempotencyKeys := make(map[string]EventID, len(r.entries))
	for _, entry := range r.entries {
		existingEventIDs[entry.Event.EventID] = struct{}{}
		existingIdempotencyKeys[entry.Event.IdempotencyKey] = entry.Event.EventID
		if entry.Aggregate == aggregate {
			existing = append(existing, entry.Clone())
		}
	}
	for _, entry := range entries {
		if _, ok := existingEventIDs[entry.Event.EventID]; ok {
			return validationError(ErrRevisionConflict, "event_id", entry.Event.EventID)
		}
		if existingEventID, ok := existingIdempotencyKeys[entry.Event.IdempotencyKey]; ok && existingEventID != entry.Event.EventID {
			return validationError(ErrRevisionConflict, "idempotency_key", entry.Event.IdempotencyKey)
		}
	}
	var head Revision
	if len(existing) > 0 {
		history, err := NewHistory(existing)
		if err != nil {
			return err
		}
		head = history.HeadRevision()
	}
	want := InitialRevision()
	if head != 0 {
		next, err := head.Next()
		if err != nil {
			return err
		}
		want = next
	}
	if entries[0].Revision != want {
		return validationError(ErrRevisionConflict, "revision", entries[0].Revision)
	}
	return nil
}

func matchesQuery(entry Entry, query Query) bool {
	if query.Aggregate != nil && entry.Aggregate != *query.Aggregate {
		return false
	}
	if query.AggregateType != "" && entry.Aggregate.Type != query.AggregateType {
		return false
	}
	if query.FromRevision != 0 && entry.Revision < query.FromRevision {
		return false
	}
	if query.ToRevision != 0 && entry.Revision > query.ToRevision {
		return false
	}
	if !query.FromRecordedAt.IsZero() && entry.Event.RecordedAt.Before(query.FromRecordedAt) {
		return false
	}
	if !query.ToRecordedAt.IsZero() && entry.Event.RecordedAt.After(query.ToRecordedAt) {
		return false
	}
	return true
}

func reverseEntries(entries []Entry) {
	for i, j := 0, len(entries)-1; i < j; i, j = i+1, j-1 {
		entries[i], entries[j] = entries[j], entries[i]
	}
}
