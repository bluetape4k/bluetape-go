package sqloutboxtest

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/bluetape4k/bluetape-go/audit"
	"github.com/bluetape4k/bluetape-go/audit/sqloutbox"
)

var (
	// ErrNilPublisherFunc는 PublisherFunc가 nil일 때 반환된다.
	ErrNilPublisherFunc = errors.New("nil sqloutbox publisher function")
	// ErrNilRecordingPublisher는 *RecordingPublisher receiver가 nil일 때 반환된다.
	ErrNilRecordingPublisher = errors.New("nil sqloutbox recording publisher")
	// ErrInjectedFailure는 WithFailures에 명시 오류가 없을 때 사용하는 기본 주입 오류다.
	ErrInjectedFailure = errors.New("injected sqloutbox publisher failure")
)

// PublisherFunc는 func 공개 타입이며 SQL outbox transaction, relay, idempotent delivery 계약을 보존한다.
// 필드와 zero value, nil 허용 여부, 동시성/transaction 소유권은 생성자와 메서드의 한국어 주석 및 테스트 계약을 따른다.
type PublisherFunc func(context.Context, sqloutbox.Record) error

// Publish는 Publish 공개 API의 동작을 수행하며 SQL outbox transaction, relay, idempotent delivery 계약을 보존한다.
//
// 매개변수:
//   - ctx: 호출자가 소유한 취소, deadline, 요청 범위를 전달한다.
//   - record: Publish 동작에 필요한 record 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//
// 반환 오류는 입력 검증 실패, 취소, transaction 실패, repository/outbox 실패, 또는 package sentinel/typed error 계약을 보존한다.
func (fn PublisherFunc) Publish(ctx context.Context, record sqloutbox.Record) error {
	if err := contextError(ctx); err != nil {
		return err
	}
	if fn == nil {
		return ErrNilPublisherFunc
	}
	return fn(ctx, record)
}

// DiscardPublisher는 struct 공개 타입이며 SQL outbox transaction, relay, idempotent delivery 계약을 보존한다.
// 필드와 zero value, nil 허용 여부, 동시성/transaction 소유권은 생성자와 메서드의 한국어 주석 및 테스트 계약을 따른다.
type DiscardPublisher struct{}

// Publish는 Publish 공개 API의 동작을 수행하며 SQL outbox transaction, relay, idempotent delivery 계약을 보존한다.
//
// 매개변수:
//   - ctx: 호출자가 소유한 취소, deadline, 요청 범위를 전달한다.
//   - _: Publish 동작에 필요한 _ 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//
// 반환 오류는 입력 검증 실패, 취소, transaction 실패, repository/outbox 실패, 또는 package sentinel/typed error 계약을 보존한다.
func (DiscardPublisher) Publish(ctx context.Context, _ sqloutbox.Record) error {
	return contextError(ctx)
}

// RecordingOption는 func 공개 타입이며 SQL outbox transaction, relay, idempotent delivery 계약을 보존한다.
// 필드와 zero value, nil 허용 여부, 동시성/transaction 소유권은 생성자와 메서드의 한국어 주석 및 테스트 계약을 따른다.
type RecordingOption func(*RecordingPublisher)

// WithFailures는 WithFailures 공개 API의 동작을 수행하며 SQL outbox transaction, relay, idempotent delivery 계약을 보존한다.
//
// 매개변수:
//   - failures: WithFailures 동작에 필요한 failures 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//   - err: WithFailures 동작에 필요한 err 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
func WithFailures(failures map[audit.EventID]int, err error) RecordingOption {
	return func(p *RecordingPublisher) {
		p.mu.Lock()
		defer p.mu.Unlock()

		if err == nil {
			err = ErrInjectedFailure
		}
		p.failureErr = err
		p.failures = make(map[audit.EventID]int, len(failures))
		for eventID, count := range failures {
			if count > 0 {
				p.failures[eventID] = count
			}
		}
	}
}

// RecordingPublisher는 struct 공개 타입이며 SQL outbox transaction, relay, idempotent delivery 계약을 보존한다.
// 필드와 zero value, nil 허용 여부, 동시성/transaction 소유권은 생성자와 메서드의 한국어 주석 및 테스트 계약을 따른다.
type RecordingPublisher struct {
	mu         sync.Mutex
	records    []sqloutbox.Record
	failures   map[audit.EventID]int
	failureErr error
}

// NewRecordingPublisher는 NewRecordingPublisher 공개 API의 동작을 수행하며 SQL outbox transaction, relay, idempotent delivery 계약을 보존한다.
//
// 매개변수:
//   - options: NewRecordingPublisher 동작에 필요한 options 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
func NewRecordingPublisher(options ...RecordingOption) *RecordingPublisher {
	publisher := &RecordingPublisher{}
	for _, option := range options {
		if option != nil {
			option(publisher)
		}
	}
	return publisher
}

// Publish는 Publish 공개 API의 동작을 수행하며 SQL outbox transaction, relay, idempotent delivery 계약을 보존한다.
//
// 매개변수:
//   - ctx: 호출자가 소유한 취소, deadline, 요청 범위를 전달한다.
//   - record: Publish 동작에 필요한 record 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//
// 반환 오류는 입력 검증 실패, 취소, transaction 실패, repository/outbox 실패, 또는 package sentinel/typed error 계약을 보존한다.
func (p *RecordingPublisher) Publish(ctx context.Context, record sqloutbox.Record) error {
	if err := contextError(ctx); err != nil {
		return err
	}
	if p == nil {
		return ErrNilRecordingPublisher
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	p.records = append(p.records, record)
	if p.failures != nil {
		if remaining := p.failures[record.EventID]; remaining > 0 {
			p.failures[record.EventID] = remaining - 1
			if p.failureErr == nil {
				return ErrInjectedFailure
			}
			return p.failureErr
		}
	}
	return nil
}

// Records는 Records 공개 API의 동작을 수행하며 SQL outbox transaction, relay, idempotent delivery 계약을 보존한다.
func (p *RecordingPublisher) Records() []sqloutbox.Record {
	if p == nil {
		return nil
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	records := make([]sqloutbox.Record, len(p.records))
	copy(records, p.records)
	return records
}

// EventIDs는 EventIDs 공개 API의 동작을 수행하며 SQL outbox transaction, relay, idempotent delivery 계약을 보존한다.
func (p *RecordingPublisher) EventIDs() []audit.EventID {
	if p == nil {
		return nil
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	ids := make([]audit.EventID, len(p.records))
	for index, record := range p.records {
		ids[index] = record.EventID
	}
	return ids
}

// Count는 Count 공개 API의 동작을 수행하며 SQL outbox transaction, relay, idempotent delivery 계약을 보존한다.
func (p *RecordingPublisher) Count() int {
	if p == nil {
		return 0
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	return len(p.records)
}

// Reset는 Reset 공개 API의 동작을 수행하며 SQL outbox transaction, relay, idempotent delivery 계약을 보존한다.
func (p *RecordingPublisher) Reset() {
	if p == nil {
		return
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	p.records = nil
	p.failures = nil
	p.failureErr = nil
}

func contextError(ctx context.Context) error {
	if ctx == nil {
		return fmt.Errorf("nil context")
	}
	return ctx.Err()
}
