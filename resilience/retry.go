package resilience

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// RetryPredicate decides whether err should be retried.
type RetryPredicate func(error) bool

// Sleeper waits for retry delays. Tests can replace it with a fake sleeper.
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

// RetryOptions configures a retry policy.
type RetryOptions struct {
	Name        string
	MaxAttempts int
	Backoff     Backoff
	RetryIf     RetryPredicate
	Sleeper     Sleeper
	OnEvent     EventHandler
}

// RetryPolicy retries failed operations.
type RetryPolicy[T any] struct {
	options RetryOptions
}

// NewRetry creates a retry policy.
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

// Apply wraps operation with retry behavior.
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
