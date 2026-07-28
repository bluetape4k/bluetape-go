package resilience

import (
	"context"
	"fmt"
)

// BulkheadOptions struct 공개 타입이며 취소, deadline, retry, timeout, circuit breaker 상태를 보존한다.
// 필드와 zero value, nil 허용 여부, 동시성 소유권은 생성자와 메서드의 한국어 주석 및 테스트 계약을 따른다.
type BulkheadOptions struct {
	Name          string
	MaxConcurrent int
	Wait          bool
	OnEvent       EventHandler
}

// BulkheadPolicy struct 공개 타입이며 취소, deadline, retry, timeout, circuit breaker 상태를 보존한다.
// 필드와 zero value, nil 허용 여부, 동시성 소유권은 생성자와 메서드의 한국어 주석 및 테스트 계약을 따른다.
type BulkheadPolicy[T any] struct {
	options BulkheadOptions
	permits chan struct{}
}

// NewBulkhead NewBulkhead 공개 API의 동작을 수행하며 취소, deadline, retry, timeout, circuit breaker 상태를 보존한다.
//
// 매개변수:
//   - options: 적용할 옵션 목록이다. nil이면 기본값만 사용한다.
//
// 반환 오류는 입력 검증 실패, context 취소/deadline, 상태 전이 실패, 패키지 sentinel error와 typed error를 그대로 드러낸다.
func NewBulkhead[T any](options BulkheadOptions) (*BulkheadPolicy[T], error) {
	if options.MaxConcurrent <= 0 {
		return nil, fmt.Errorf("max concurrent must be positive")
	}
	return &BulkheadPolicy[T]{
		options: options,
		permits: make(chan struct{}, options.MaxConcurrent),
	}, nil
}

// InFlight InFlight 공개 API의 동작을 수행하며 취소, deadline, retry, timeout, circuit breaker 상태를 보존한다.
func (p *BulkheadPolicy[T]) InFlight() int {
	return len(p.permits)
}

// Apply Apply 공개 API의 동작을 수행하며 취소, deadline, retry, timeout, circuit breaker 상태를 보존한다.
//
// 매개변수:
//   - operation: 보호 정책 안에서 실행할 작업이다.
func (p *BulkheadPolicy[T]) Apply(operation Operation[T]) Operation[T] {
	return func(ctx context.Context) (T, error) {
		var zero T
		if ctx == nil {
			ctx = context.Background()
		}

		if err := p.acquire(ctx); err != nil {
			return zero, err
		}
		defer p.release()

		value, err := runOperation(ctx, operation)
		if err == nil {
			emitEvent(ctx, p.options.OnEvent, Event{
				PolicyName: p.options.Name,
				PolicyType: PolicyTypeBulkhead,
				Kind:       EventSuccess,
				Category:   EventCategorySuccess,
				InFlight:   p.InFlight(),
			})
		}
		return value, err
	}
}

func (p *BulkheadPolicy[T]) acquire(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	if p.options.Wait {
		select {
		case p.permits <- struct{}{}:
			p.emitAccepted(ctx)
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	select {
	case p.permits <- struct{}{}:
		p.emitAccepted(ctx)
		return nil
	default:
		rejection := BulkheadRejectedError{PolicyName: p.options.Name}
		emitEvent(ctx, p.options.OnEvent, Event{
			PolicyName:    p.options.Name,
			PolicyType:    PolicyTypeBulkhead,
			Kind:          EventBulkheadRejected,
			Category:      EventCategoryRejection,
			InFlight:      p.InFlight(),
			Err:           rejection,
			ErrorCategory: categorizeError(rejection),
		})
		return rejection
	}
}

func (p *BulkheadPolicy[T]) release() {
	<-p.permits
}

func (p *BulkheadPolicy[T]) emitAccepted(ctx context.Context) {
	emitEvent(ctx, p.options.OnEvent, Event{
		PolicyName: p.options.Name,
		PolicyType: PolicyTypeBulkhead,
		Kind:       EventBulkheadAccepted,
		Category:   EventCategoryAdmission,
		InFlight:   p.InFlight(),
	})
}
