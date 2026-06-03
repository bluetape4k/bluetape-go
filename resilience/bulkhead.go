package resilience

import (
	"context"
	"fmt"
)

// BulkheadOptions configures a bulkhead policy.
type BulkheadOptions struct {
	Name          string
	MaxConcurrent int
	Wait          bool
	OnEvent       EventHandler
}

// BulkheadPolicy limits concurrent operation execution.
type BulkheadPolicy[T any] struct {
	options BulkheadOptions
	permits chan struct{}
}

// NewBulkhead creates a bulkhead policy.
func NewBulkhead[T any](options BulkheadOptions) (*BulkheadPolicy[T], error) {
	if options.MaxConcurrent <= 0 {
		return nil, fmt.Errorf("max concurrent must be positive")
	}
	return &BulkheadPolicy[T]{
		options: options,
		permits: make(chan struct{}, options.MaxConcurrent),
	}, nil
}

// InFlight returns the current number of admitted operations.
func (p *BulkheadPolicy[T]) InFlight() int {
	return len(p.permits)
}

// Apply wraps operation with bulkhead admission control.
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
