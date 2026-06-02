package concurrency

import (
	"context"

	"golang.org/x/sync/errgroup"
)

// Task is a context-aware goroutine body.
type Task func(context.Context) error

// Group runs related tasks with shared cancellation and error propagation.
type Group struct {
	group *errgroup.Group
	ctx   context.Context
}

// NewGroup creates a task group whose context is cancelled after the first
// failing task or after Wait returns.
func NewGroup(ctx context.Context) *Group {
	if ctx == nil {
		ctx = context.Background()
	}

	group, groupCtx := errgroup.WithContext(ctx)
	return &Group{
		group: group,
		ctx:   groupCtx,
	}
}

// Context returns the group context passed to tasks.
func (g *Group) Context() context.Context {
	return g.ctx
}

// SetLimit limits the number of tasks that may run at the same time.
func (g *Group) SetLimit(limit int) {
	g.group.SetLimit(limit)
}

// Go starts task in the group.
func (g *Group) Go(task Task) {
	g.group.Go(func() error {
		return runTask(g.ctx, task)
	})
}

// TryGo starts task if doing so does not exceed the group's concurrency limit.
func (g *Group) TryGo(task Task) bool {
	return g.group.TryGo(func() error {
		return runTask(g.ctx, task)
	})
}

// Wait blocks until all started tasks finish and returns the first error.
func (g *Group) Wait() error {
	return g.group.Wait()
}
