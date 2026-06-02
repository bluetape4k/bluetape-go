package concurrency

import (
	"context"
	"fmt"
)

// Go starts task in a goroutine and returns a single-result error channel.
func Go(ctx context.Context, task Task) <-chan error {
	result := make(chan error, 1)
	go func() {
		result <- runTask(ctx, task)
		close(result)
	}()
	return result
}

// ForEach applies worker to every value with a bounded concurrency limit.
func ForEach[T any](ctx context.Context, values []T, limit int, worker func(context.Context, T) error) error {
	if limit <= 0 {
		return fmt.Errorf("limit must be positive")
	}
	if worker == nil {
		return fmt.Errorf("worker must not be nil")
	}

	group := NewGroup(ctx)
	group.SetLimit(limit)

	for _, value := range values {
		value := value
		group.Go(func(taskCtx context.Context) error {
			return worker(taskCtx, value)
		})
	}

	return group.Wait()
}

// Map applies mapper to every value with a bounded concurrency limit and
// returns results in input order.
func Map[T any, R any](ctx context.Context, values []T, limit int, mapper func(context.Context, T) (R, error)) ([]R, error) {
	if limit <= 0 {
		return nil, fmt.Errorf("limit must be positive")
	}
	if mapper == nil {
		return nil, fmt.Errorf("mapper must not be nil")
	}
	if values == nil {
		return nil, nil
	}

	results := make([]R, len(values))
	group := NewGroup(ctx)
	group.SetLimit(limit)

	for index, value := range values {
		index, value := index, value
		group.Go(func(taskCtx context.Context) error {
			mapped, err := mapper(taskCtx, value)
			if err != nil {
				return err
			}
			results[index] = mapped
			return nil
		})
	}

	if err := group.Wait(); err != nil {
		return nil, err
	}
	return results, nil
}
