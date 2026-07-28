package audit

import (
	"context"
	"strings"
	"time"
)

// Repository는 interface 공개 타입이며 audit entry, event, repository, recorder, history 계약을 보존한다.
// 필드와 zero value, nil 허용 여부, 동시성/transaction 소유권은 생성자와 메서드의 한국어 주석 및 테스트 계약을 따른다.
type Repository interface {
	Append(ctx context.Context, entries ...Entry) error
	HistoryReader
}

// HistoryReader는 interface 공개 타입이며 audit entry, event, repository, recorder, history 계약을 보존한다.
// 필드와 zero value, nil 허용 여부, 동시성/transaction 소유권은 생성자와 메서드의 한국어 주석 및 테스트 계약을 따른다.
type HistoryReader interface {
	Find(ctx context.Context, query Query) ([]Entry, error)
	LoadHistory(ctx context.Context, aggregate AggregateID) (History, bool, error)
	Latest(ctx context.Context, aggregate AggregateID) (Entry, bool, error)
	LatestSnapshot(ctx context.Context, aggregate AggregateID) (Entry, bool, error)
	PreviousSnapshot(ctx context.Context, aggregate AggregateID, before Revision) (Entry, bool, error)
}

// Query는 struct 공개 타입이며 audit entry, event, repository, recorder, history 계약을 보존한다.
// 필드와 zero value, nil 허용 여부, 동시성/transaction 소유권은 생성자와 메서드의 한국어 주석 및 테스트 계약을 따른다.
type Query struct {
	Aggregate      *AggregateID
	AggregateType  string
	FromRevision   Revision
	ToRevision     Revision
	FromRecordedAt time.Time
	ToRecordedAt   time.Time
	NewestFirst    bool
	Limit          int
}

// Validate는 Validate 공개 API의 동작을 수행하며 audit entry, event, repository, recorder, history 계약을 보존한다.
//
// 반환 오류는 입력 검증 실패, 취소, transaction 실패, repository/outbox 실패, 또는 package sentinel/typed error 계약을 보존한다.
func (q Query) Validate() (Query, error) {
	normalized := q
	if q.Aggregate != nil {
		aggregate := *q.Aggregate
		if err := aggregate.Validate(); err != nil {
			return Query{}, validationCause(ErrInvalidQuery, "aggregate", aggregate, err)
		}
		normalized.Aggregate = &aggregate
	}
	normalized.AggregateType = strings.TrimSpace(q.AggregateType)
	if q.FromRevision != 0 {
		if err := q.FromRevision.Validate(); err != nil {
			return Query{}, validationCause(ErrInvalidQuery, "from_revision", q.FromRevision, err)
		}
	}
	if q.ToRevision != 0 {
		if err := q.ToRevision.Validate(); err != nil {
			return Query{}, validationCause(ErrInvalidQuery, "to_revision", q.ToRevision, err)
		}
	}
	if q.FromRevision != 0 && q.ToRevision != 0 && q.FromRevision > q.ToRevision {
		return Query{}, validationError(ErrInvalidQuery, "revision_range", q.FromRevision)
	}
	if !q.FromRecordedAt.IsZero() && !q.ToRecordedAt.IsZero() && q.FromRecordedAt.After(q.ToRecordedAt) {
		return Query{}, validationError(ErrInvalidQuery, "recorded_at_range", q.FromRecordedAt)
	}
	if q.Limit < 0 {
		return Query{}, validationError(ErrInvalidQuery, "limit", q.Limit)
	}
	return normalized, nil
}

func normalizeContext(ctx context.Context) context.Context {
	if ctx == nil {
		return context.Background()
	}
	return ctx
}

func checkContext(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		return nil
	}
}
