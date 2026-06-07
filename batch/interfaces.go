package batch

import "context"

// Reader supplies input items for a batch step.
type Reader[T any] interface {
	Open(context.Context) error
	Read(context.Context) (T, bool, error)
	Close(context.Context) error
}

// Processor transforms or filters one input item.
//
// Returning keep=false filters the item without treating it as a failure.
type Processor[I any, O any] interface {
	Process(context.Context, I) (O, bool, error)
}

// ProcessorFunc adapts a function into a Processor.
type ProcessorFunc[I any, O any] func(context.Context, I) (O, bool, error)

// Process transforms or filters one input item.
func (f ProcessorFunc[I, O]) Process(ctx context.Context, item I) (O, bool, error) {
	return f(ctx, item)
}

// IdentityProcessor returns a processor that forwards every input item.
func IdentityProcessor[T any]() Processor[T, T] {
	return ProcessorFunc[T, T](func(ctx context.Context, item T) (T, bool, error) {
		if err := ctx.Err(); err != nil {
			var zero T
			return zero, false, err
		}
		return item, true, nil
	})
}

// Writer persists processed items in chunk units.
type Writer[T any] interface {
	Open(context.Context) error
	Write(context.Context, []T) error
	Close(context.Context) error
}
