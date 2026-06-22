package concurrencytest

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/bluetape4k/bluetape-go/concurrency"
)

type recorder struct {
	started       int32
	completed     int32
	failures      int32
	panics        int32
	running       int32
	maxConcurrent int32

	mu     sync.Mutex
	errors []error
}

func (r *recorder) start() {
	current := atomic.AddInt32(&r.running, 1)
	atomic.AddInt32(&r.started, 1)
	for {
		observed := atomic.LoadInt32(&r.maxConcurrent)
		if current <= observed || atomic.CompareAndSwapInt32(&r.maxConcurrent, observed, current) {
			return
		}
	}
}

func (r *recorder) finish(err error) {
	defer atomic.AddInt32(&r.running, -1)

	if err == nil {
		atomic.AddInt32(&r.completed, 1)
		return
	}

	atomic.AddInt32(&r.failures, 1)
	var panicErr concurrency.PanicError
	if errors.As(err, &panicErr) {
		atomic.AddInt32(&r.panics, 1)
	}

	r.mu.Lock()
	r.errors = append(r.errors, err)
	r.mu.Unlock()
}

func (r *recorder) addError(err error) {
	if err == nil {
		return
	}
	atomic.AddInt32(&r.failures, 1)
	var panicErr concurrency.PanicError
	if errors.As(err, &panicErr) {
		atomic.AddInt32(&r.panics, 1)
	}

	r.mu.Lock()
	r.errors = append(r.errors, err)
	r.mu.Unlock()
}

func (r *recorder) report(duration time.Duration, scheduled int) Report {
	started := int(atomic.LoadInt32(&r.started))
	skipped := scheduled - started
	if skipped < 0 {
		skipped = 0
	}
	return Report{
		Scheduled:     scheduled,
		Started:       started,
		Completed:     int(atomic.LoadInt32(&r.completed)),
		Failures:      int(atomic.LoadInt32(&r.failures)),
		Panics:        int(atomic.LoadInt32(&r.panics)),
		Skipped:       skipped,
		MaxConcurrent: int(atomic.LoadInt32(&r.maxConcurrent)),
		Duration:      duration,
	}
}

func (r *recorder) err() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if len(r.errors) == 0 {
		return nil
	}
	copied := make([]error, len(r.errors))
	copy(copied, r.errors)
	return RunError{Errors: copied}
}

type unit struct {
	index int
	task  Task
}

func buildUnits(rounds int, tasks []Task) []unit {
	units := make([]unit, 0, rounds*len(tasks))
	for range rounds {
		for index, task := range tasks {
			units = append(units, unit{index: index, task: task})
		}
	}
	return units
}

func validateTasks(tasks []Task) error {
	if len(tasks) == 0 {
		return fmt.Errorf("at least one task is required")
	}
	for index, task := range tasks {
		if task == nil {
			return fmt.Errorf("task %d must not be nil", index)
		}
	}
	return nil
}

func runSafely(ctx context.Context, task Task) (err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err = concurrency.PanicError{Value: recovered}
		}
	}()

	if err := ctx.Err(); err != nil {
		return err
	}
	return task(ctx)
}

func runAll(ctx context.Context, opts Options, tasks []Task) (Report, error) {
	opts, err := opts.normalize()
	if err != nil {
		return Report{}, err
	}
	if err := validateTasks(tasks); err != nil {
		return Report{}, err
	}

	runCtx, cancel := withTimeout(ctx, opts.Timeout)
	defer cancel()

	startedAt := time.Now()
	units := buildUnits(opts.RoundsPerTask, tasks)
	jobs := make(chan unit)
	var record recorder
	var wg sync.WaitGroup

	for range opts.Workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-runCtx.Done():
					return
				case job, ok := <-jobs:
					if !ok {
						return
					}
					record.start()
					record.finish(runSafely(runCtx, job.task))
				}
			}
		}()
	}

	for _, job := range units {
		select {
		case <-runCtx.Done():
			record.addError(runCtx.Err())
			close(jobs)
			wg.Wait()
			return record.report(time.Since(startedAt), len(units)), record.err()
		case jobs <- job:
		}
	}

	close(jobs)
	wg.Wait()
	return record.report(time.Since(startedAt), len(units)), record.err()
}
