package concurrency

import (
	"context"
	"fmt"
)

// WorkerPool runs jobs from a channel with a fixed number of workers.
type WorkerPool[T any] struct {
	size    int
	handler func(context.Context, T) error
}

// NewWorkerPool creates a bounded worker pool.
func NewWorkerPool[T any](size int, handler func(context.Context, T) error) (*WorkerPool[T], error) {
	if size <= 0 {
		return nil, fmt.Errorf("size must be positive")
	}
	if handler == nil {
		return nil, fmt.Errorf("handler must not be nil")
	}

	return &WorkerPool[T]{
		size:    size,
		handler: handler,
	}, nil
}

// Run consumes jobs until the channel closes, a worker fails, or ctx is
// cancelled.
func (p *WorkerPool[T]) Run(ctx context.Context, jobs <-chan T) error {
	if jobs == nil {
		return fmt.Errorf("jobs must not be nil")
	}

	group := NewGroup(ctx)
	for range p.size {
		group.Go(func(taskCtx context.Context) error {
			for {
				select {
				case <-taskCtx.Done():
					return taskCtx.Err()
				case job, ok := <-jobs:
					if !ok {
						return nil
					}
					if err := p.handler(taskCtx, job); err != nil {
						return err
					}
				}
			}
		})
	}

	return group.Wait()
}
