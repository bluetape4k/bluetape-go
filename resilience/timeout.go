package resilience

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// TimeoutOptions configures a timeout policy.
type TimeoutOptions struct {
	Name    string
	Timeout time.Duration
	OnEvent EventHandler
}

// TimeoutPolicy applies a per-operation context timeout.
type TimeoutPolicy[T any] struct {
	options TimeoutOptions
}

// NewTimeout creates a timeout policy.
func NewTimeout[T any](options TimeoutOptions) (*TimeoutPolicy[T], error) {
	if options.Timeout <= 0 {
		return nil, fmt.Errorf("timeout must be positive")
	}
	return &TimeoutPolicy[T]{options: options}, nil
}

// Apply wraps operation with timeout behavior.
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
				PolicyType: "timeout",
				Kind:       EventSuccess,
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
				PolicyName: p.options.Name,
				PolicyType: "timeout",
				Kind:       EventTimeout,
				Err:        timeoutErr,
			})
			return value, timeoutErr
		}

		return value, err
	}
}
