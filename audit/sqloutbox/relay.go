package sqloutbox

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/bluetape4k/bluetape-go/sqlkit"
)

const (
	defaultClaimLimit  = 10
	defaultMaxAttempts = 3
	defaultRetryDelay  = time.Second
	defaultIdleDelay   = 100 * time.Millisecond
)

// Publisher SQL outbox store, relay, transaction, idempotent delivery에서 사용하는 인터페이스이다.
type Publisher interface {
	Publish(context.Context, Record) error
}

// RelayOptions SQL outbox store, relay, transaction, idempotent delivery에서 사용하는 구조체다.
type RelayOptions struct {
	ClaimLimit  int
	MaxAttempts int
	RetryDelay  time.Duration
	IdleDelay   time.Duration
	Now         func() time.Time
}

// RelayResult SQL outbox store, relay, transaction, idempotent delivery에서 사용하는 구조체다.
type RelayResult struct {
	Claimed      int
	Published    int
	Failed       int
	DeadLettered int
}

// Relay SQL outbox store, relay, transaction, idempotent delivery에서 사용하는 구조체다.
type Relay struct {
	store      *Store
	publisher  Publisher
	claimLimit int
	maxAttempt int
	retryDelay time.Duration
	idleDelay  time.Duration
	now        func() time.Time
}

// NewRelay SQL outbox store, relay, transaction, idempotent delivery에 사용할 값을 생성한다.
//
// 매개변수:
//   - store: SQL transaction 또는 outbox 저장소 backend다. commit/rollback 소유권은 호출자와 store 계약을 따른다.
//   - publisher: outbox delivery 또는 relay publisher다. 중복 전송과 retry 의미는 outbox 계약을 따른다.
//   - options: 적용할 옵션 목록이다. nil이면 기본값만 사용한다.
//
// 반환 오류는 입력 검증 실패, context 취소, transaction 실패, repository/outbox 실패, package sentinel error와 typed error를 그대로 드러낸다.
func NewRelay(store *Store, publisher Publisher, options RelayOptions) (*Relay, error) {
	if store == nil {
		return nil, fmt.Errorf("%w: store must not be nil", ErrInvalidArgument)
	}
	if publisher == nil {
		return nil, fmt.Errorf("%w: publisher must not be nil", ErrInvalidArgument)
	}

	claimLimit := options.ClaimLimit
	if claimLimit == 0 {
		claimLimit = defaultClaimLimit
	}
	if claimLimit < 0 {
		return nil, fmt.Errorf("%w: claim limit must be positive", ErrInvalidArgument)
	}
	maxAttempts := options.MaxAttempts
	if maxAttempts == 0 {
		maxAttempts = defaultMaxAttempts
	}
	if maxAttempts < 0 {
		return nil, fmt.Errorf("%w: max attempts must be positive", ErrInvalidArgument)
	}
	retryDelay := options.RetryDelay
	if retryDelay == 0 {
		retryDelay = defaultRetryDelay
	}
	if retryDelay < 0 {
		return nil, fmt.Errorf("%w: retry delay must not be negative", ErrInvalidArgument)
	}
	idleDelay := options.IdleDelay
	if idleDelay == 0 {
		idleDelay = defaultIdleDelay
	}
	if idleDelay < 0 {
		return nil, fmt.Errorf("%w: idle delay must not be negative", ErrInvalidArgument)
	}
	now := options.Now
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}

	return &Relay{
		store:      store,
		publisher:  publisher,
		claimLimit: claimLimit,
		maxAttempt: maxAttempts,
		retryDelay: retryDelay,
		idleDelay:  idleDelay,
		now:        now,
	}, nil
}

// RunOnce SQL outbox store, relay, transaction, idempotent delivery의 쓰기 동작을 수행한다.
//
// 매개변수:
//   - ctx: 호출자가 소유한 취소, deadline, 요청 범위를 전달한다.
//   - db: SQL transaction 또는 outbox 저장소 backend다. commit/rollback 소유권은 호출자와 store 계약을 따른다.
//
// 반환 오류는 입력 검증 실패, context 취소, transaction 실패, repository/outbox 실패, package sentinel error와 typed error를 그대로 드러낸다.
func (r *Relay) RunOnce(ctx context.Context, db sqlkit.Session) (RelayResult, error) {
	if r == nil {
		return RelayResult{}, fmt.Errorf("%w: relay must not be nil", ErrInvalidArgument)
	}
	if db == nil {
		return RelayResult{}, fmt.Errorf("%w: session must not be nil", ErrInvalidArgument)
	}
	now := r.now()
	records, err := r.store.Claim(ctx, db, ClaimOptions{Limit: r.claimLimit, Now: now})
	if err != nil {
		return RelayResult{}, err
	}

	result := RelayResult{Claimed: len(records)}
	for _, record := range records {
		if err := ctx.Err(); err != nil {
			return result, err
		}
		if err := r.publisher.Publish(ctx, record); err != nil {
			if isCallerCancellation(ctx, err) {
				return result, err
			}
			deadLetter := record.Attempts >= r.maxAttempt
			if markErr := r.store.MarkFailed(ctx, db, Failure{
				ID:          record.ID,
				Attempt:     record.Attempts,
				Err:         err,
				RetryAt:     now.Add(r.retryDelay),
				MaxAttempts: r.maxAttempt,
				Now:         now,
			}); markErr != nil {
				return result, markErr
			}
			if deadLetter {
				result.DeadLettered++
			} else {
				result.Failed++
			}
			continue
		}
		if err := r.store.MarkPublished(ctx, db, record); err != nil {
			return result, err
		}
		result.Published++
	}
	return result, nil
}

func isCallerCancellation(ctx context.Context, err error) bool {
	if err == nil {
		return false
	}
	if ctx == nil {
		return false
	}
	ctxErr := ctx.Err()
	if ctxErr == nil {
		return false
	}
	return errors.Is(err, ctxErr)
}

// Run SQL outbox store, relay, transaction, idempotent delivery의 쓰기 동작을 수행한다.
//
// 매개변수:
//   - ctx: 호출자가 소유한 취소, deadline, 요청 범위를 전달한다.
//   - db: SQL transaction 또는 outbox 저장소 backend다. commit/rollback 소유권은 호출자와 store 계약을 따른다.
//
// 반환 오류는 입력 검증 실패, context 취소, transaction 실패, repository/outbox 실패, package sentinel error와 typed error를 그대로 드러낸다.
func (r *Relay) Run(ctx context.Context, db sqlkit.Session) error {
	if ctx == nil {
		ctx = context.Background()
	}
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		result, err := r.RunOnce(ctx, db)
		if err != nil {
			return err
		}
		if result.Claimed > 0 {
			continue
		}
		timer := time.NewTimer(r.idleDelay)
		select {
		case <-ctx.Done():
			if !timer.Stop() {
				<-timer.C
			}
			return ctx.Err()
		case <-timer.C:
		}
	}
}
