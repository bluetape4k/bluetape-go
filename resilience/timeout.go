package resilience

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// TimeoutOptions struct 공개 타입이며 취소, deadline, retry, timeout, circuit breaker 상태를 보존한다.
// 필드와 zero value, nil 허용 여부, 동시성 소유권은 생성자와 메서드의 한국어 주석 및 테스트 계약을 따른다.
type TimeoutOptions struct {
	Name    string
	Timeout time.Duration
	OnEvent EventHandler
}

// TimeoutPolicy struct 공개 타입이며 취소, deadline, retry, timeout, circuit breaker 상태를 보존한다.
// 필드와 zero value, nil 허용 여부, 동시성 소유권은 생성자와 메서드의 한국어 주석 및 테스트 계약을 따른다.
type TimeoutPolicy[T any] struct {
	options TimeoutOptions
}

// NewTimeout NewTimeout 공개 API의 동작을 수행하며 취소, deadline, retry, timeout, circuit breaker 상태를 보존한다.
//
// 매개변수:
//   - options: 적용할 옵션 목록이다. nil이면 기본값만 사용한다.
//
// 반환 오류는 입력 검증 실패, context 취소/deadline, 상태 전이 실패, 패키지 sentinel error와 typed error를 그대로 드러낸다.
func NewTimeout[T any](options TimeoutOptions) (*TimeoutPolicy[T], error) {
	if options.Timeout <= 0 {
		return nil, fmt.Errorf("timeout must be positive")
	}
	return &TimeoutPolicy[T]{options: options}, nil
}

// Apply Apply 공개 API의 동작을 수행하며 취소, deadline, retry, timeout, circuit breaker 상태를 보존한다.
//
// 매개변수:
//   - operation: 보호 정책 안에서 실행할 작업이다.
func (p *TimeoutPolicy[T]) Apply(operation Operation[T]) Operation[T] {
	return func(ctx context.Context) (T, error) {
		if ctx == nil {
			ctx = context.Background()
		}

		timeoutCtx, cancel := context.WithTimeout(ctx, p.options.Timeout)
		defer cancel()

		value, err := runOperation(timeoutCtx, operation)
		if err == nil {
			emitEvent(ctx, p.options.OnEvent, Event{
				PolicyName: p.options.Name,
				PolicyType: PolicyTypeTimeout,
				Kind:       EventSuccess,
				Category:   EventCategorySuccess,
				Timeout:    p.options.Timeout,
			})
			return value, nil
		}

		if errors.Is(err, context.DeadlineExceeded) &&
			timeoutCtx.Err() == context.DeadlineExceeded &&
			ctx.Err() == nil {
			timeoutErr := TimeoutError{
				PolicyName: p.options.Name,
				Timeout:    p.options.Timeout,
				Cause:      err,
			}
			emitEvent(ctx, p.options.OnEvent, Event{
				PolicyName:    p.options.Name,
				PolicyType:    PolicyTypeTimeout,
				Kind:          EventTimeout,
				Category:      EventCategoryTimeout,
				Timeout:       p.options.Timeout,
				Err:           timeoutErr,
				ErrorCategory: categorizeError(timeoutErr),
			})
			return value, timeoutErr
		}

		return value, err
	}
}
