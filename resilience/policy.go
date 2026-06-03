package resilience

import (
	"context"
	"fmt"
)

// Operation is a context-aware unit of work protected by resilience policies.
type Operation[T any] func(context.Context) (T, error)

// Policy wraps an operation with resilience behavior.
type Policy[T any] interface {
	Apply(Operation[T]) Operation[T]
}

// PolicyFunc adapts a function into a Policy.
type PolicyFunc[T any] func(Operation[T]) Operation[T]

// Apply wraps operation with fn.
func (fn PolicyFunc[T]) Apply(operation Operation[T]) Operation[T] {
	return fn(operation)
}

// Compose returns a policy that applies policies in the order they are listed.
// The first policy is the outermost policy.
func Compose[T any](policies ...Policy[T]) Policy[T] {
	return PolicyFunc[T](func(operation Operation[T]) Operation[T] {
		wrapped := operation
		for index := len(policies) - 1; index >= 0; index-- {
			if policies[index] == nil {
				continue
			}
			wrapped = policies[index].Apply(wrapped)
		}
		return wrapped
	})
}

// Run executes operation after applying policies in order.
func Run[T any](ctx context.Context, operation Operation[T], policies ...Policy[T]) (T, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	wrapped := Compose(policies...).Apply(operation)
	return runOperation(ctx, wrapped)
}

func runOperation[T any](ctx context.Context, operation Operation[T]) (T, error) {
	var zero T
	if operation == nil {
		return zero, fmt.Errorf("operation must not be nil")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return zero, err
	}
	return operation(ctx)
}
