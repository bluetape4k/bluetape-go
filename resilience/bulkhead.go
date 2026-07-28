package resilience

import (
	"context"
	"fmt"
)

// BulkheadOptions는 struct 공개 타입이며 취소, deadline, retry, timeout, circuit breaker 상태를 보존한다.
// 필드와 zero value, nil 허용 여부, 동시성 소유권은 생성자와 메서드의 한국어 주석 및 테스트 계약을 따른다.
type BulkheadOptions struct {
	Name          string
	MaxConcurrent int
	Wait          bool
	OnEvent       EventHandler
}

// BulkheadPolicy는 struct 공개 타입이며 취소, deadline, retry, timeout, circuit breaker 상태를 보존한다.
// 필드와 zero value, nil 허용 여부, 동시성 소유권은 생성자와 메서드의 한국어 주석 및 테스트 계약을 따른다.
type BulkheadPolicy[T any] struct {
	options BulkheadOptions
	permits chan struct{}
}

// NewBulkhead는 NewBulkhead 공개 API의 동작을 수행하며 취소, deadline, retry, timeout, circuit breaker 상태를 보존한다.
//
// 매개변수:
//   - options: NewBulkhead 동작에 필요한 options 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//
// 반환 오류는 입력 검증 실패, 취소, deadline, 상태 전이 실패, 또는 패키지 sentinel/typed error 계약을 보존한다.
func NewBulkhead[T any](options BulkheadOptions) (*BulkheadPolicy[T], error) {
	if options.MaxConcurrent <= 0 {
		return nil, fmt.Errorf("max concurrent must be positive")
	}
	return &BulkheadPolicy[T]{
		options: options,
		permits: make(chan struct{}, options.MaxConcurrent),
	}, nil
}

// InFlight는 InFlight 공개 API의 동작을 수행하며 취소, deadline, retry, timeout, circuit breaker 상태를 보존한다.
func (p *BulkheadPolicy[T]) InFlight() int {
	return len(p.permits)
}

// Apply는 Apply 공개 API의 동작을 수행하며 취소, deadline, retry, timeout, circuit breaker 상태를 보존한다.
//
// 매개변수:
//   - operation: Apply 동작에 필요한 operation 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
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
