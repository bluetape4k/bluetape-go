package resilience

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// RetryPredicate func 공개 타입이며 취소, deadline, retry, timeout, circuit breaker 상태를 보존한다.
type RetryPredicate func(error) bool

// Sleeper interface 공개 타입이며 취소, deadline, retry, timeout, circuit breaker 상태를 보존한다.
type Sleeper interface {
	Sleep(context.Context, time.Duration) error
}

type realSleeper struct{}

func (realSleeper) Sleep(ctx context.Context, delay time.Duration) error {
	if delay <= 0 {
		return ctx.Err()
	}

	timer := time.NewTimer(delay)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

// RetryOptions struct 공개 타입이며 취소, deadline, retry, timeout, circuit breaker 상태를 보존한다.
type RetryOptions struct {
	Name        string
	MaxAttempts int
	Backoff     Backoff
	RetryIf     RetryPredicate
	Sleeper     Sleeper
	OnEvent     EventHandler
}

// RetryPolicy struct 공개 타입이며 취소, deadline, retry, timeout, circuit breaker 상태를 보존한다.
type RetryPolicy[T any] struct {
	options RetryOptions
}

// NewRetry NewRetry 공개 API의 동작을 수행하며 취소, deadline, retry, timeout, circuit breaker 상태를 보존한다.
//
// 매개변수:
//   - options: 적용할 옵션 목록이다. nil이면 기본값만 사용한다.
//
// 반환 오류는 입력 검증 실패, context 취소/deadline, 상태 전이 실패, 패키지 sentinel error와 typed error를 그대로 드러낸다.
func NewRetry[T any](options RetryOptions) (*RetryPolicy[T], error) {
	if options.MaxAttempts <= 0 {
		return nil, fmt.Errorf("max attempts must be positive")
	}
	if options.Backoff == nil {
		options.Backoff = NoBackoff()
	}
	if options.RetryIf == nil {
		options.RetryIf = retryAllButCanceled
	}
	if options.Sleeper == nil {
		options.Sleeper = realSleeper{}
	}
	return &RetryPolicy[T]{options: options}, nil
}

// Apply Apply 공개 API의 동작을 수행하며 취소, deadline, retry, timeout, circuit breaker 상태를 보존한다.
//
// 매개변수:
//   - operation: 보호 정책 안에서 실행할 작업이다.
func (p *RetryPolicy[T]) Apply(operation Operation[T]) Operation[T] {
	return func(ctx context.Context) (T, error) {
		var zero T
		if ctx == nil {
			ctx = context.Background()
		}

		var lastErr error
		for attempt := 1; attempt <= p.options.MaxAttempts; attempt++ {
			if err := ctx.Err(); err != nil {
				return zero, err
			}

			value, err := runOperation(ctx, operation)
			if err == nil {
				emitEvent(ctx, p.options.OnEvent, Event{
					PolicyName: p.options.Name,
					PolicyType: PolicyTypeRetry,
					Kind:       EventSuccess,
					Category:   EventCategorySuccess,
					Attempt:    attempt,
				})
				return value, nil
			}

			lastErr = err
			if IsNonRetryable(err) {
				emitEvent(ctx, p.options.OnEvent, Event{
					PolicyName:    p.options.Name,
					PolicyType:    PolicyTypeRetry,
					Kind:          EventFailure,
					Category:      EventCategoryFailure,
					Attempt:       attempt,
					Err:           err,
					ErrorCategory: categorizeError(err),
				})
				return zero, err
			}
			if !p.options.RetryIf(err) {
				emitEvent(ctx, p.options.OnEvent, Event{
					PolicyName:    p.options.Name,
					PolicyType:    PolicyTypeRetry,
					Kind:          EventFailure,
					Category:      EventCategoryFailure,
					Attempt:       attempt,
					Err:           err,
					ErrorCategory: categorizeError(err),
				})
				return zero, err
			}
			if attempt == p.options.MaxAttempts {
				retryErr := RetryError{
					PolicyName: p.options.Name,
					Attempts:   attempt,
					Cause:      lastErr,
				}
				emitEvent(ctx, p.options.OnEvent, Event{
					PolicyName:    p.options.Name,
					PolicyType:    PolicyTypeRetry,
					Kind:          EventFailure,
					Category:      EventCategoryFailure,
					Attempt:       attempt,
					Err:           retryErr,
					ErrorCategory: categorizeError(retryErr),
				})
				return zero, retryErr
			}

			delay := p.options.Backoff.Delay(attempt)
			emitEvent(ctx, p.options.OnEvent, Event{
				PolicyName:    p.options.Name,
				PolicyType:    PolicyTypeRetry,
				Kind:          EventRetry,
				Category:      EventCategoryRetry,
				Attempt:       attempt,
				Delay:         delay,
				Err:           err,
				ErrorCategory: categorizeError(err),
			})

			if err := p.options.Sleeper.Sleep(ctx, delay); err != nil {
				return zero, err
			}
		}

		return zero, RetryError{
			PolicyName: p.options.Name,
			Attempts:   p.options.MaxAttempts,
			Cause:      lastErr,
		}
	}
}

func retryAllButCanceled(err error) bool {
	if err == nil || errors.Is(err, context.Canceled) {
		return false
	}
	if isTimeoutError(err) {
		return true
	}
	return !errors.Is(err, context.DeadlineExceeded)
}

func isTimeoutError(err error) bool {
	var value TimeoutError
	if errors.As(err, &value) {
		return true
	}

	var pointer *TimeoutError
	return errors.As(err, &pointer)
}
