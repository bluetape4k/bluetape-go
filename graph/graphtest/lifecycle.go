package graphtest

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

type callbackResult[T any] struct {
	value      T
	err        error
	panicValue any
	contextErr error
}

var (
	errCancellationReturnedBeforeStart = errors.New("graphtest: cancellation returned before start")
	errCancellationStartTimeout        = errors.New("graphtest: cancellation start timeout")
	errCancellationDuplicateStart      = errors.New("graphtest: started called more than once")
)

func call[T any](ctx context.Context, timeout time.Duration, fn func(context.Context) (T, error)) callbackResult[T] {
	if err := ctx.Err(); err != nil {
		return callbackResult[T]{contextErr: err}
	}
	opCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	done := make(chan callbackResult[T], 1)
	go func() {
		result := callbackResult[T]{}
		defer func() {
			result.panicValue = recover()
			done <- result
		}()
		result.value, result.err = fn(opCtx)
	}()
	select {
	case result := <-done:
		if err := opCtx.Err(); err != nil {
			result.contextErr = err
		}
		return result
	case <-opCtx.Done():
		cancel()
		result := <-done
		result.contextErr = opCtx.Err()
		return result
	}
}

func callbackError[T any](phase string, result callbackResult[T]) error {
	if result.panicValue != nil {
		return fmt.Errorf("graphtest: %s panic", phase)
	}
	if result.contextErr != nil {
		return fmt.Errorf("graphtest: %s context: %w", phase, result.contextErr)
	}
	return category(phase, result.err)
}

func callbackStatus[T any](result callbackResult[T]) (status, categoryName string, timedOut bool) {
	switch {
	case result.panicValue != nil:
		return "error", "panic", false
	case errors.Is(result.contextErr, context.DeadlineExceeded):
		return "error", "timeout", true
	case result.contextErr != nil:
		return "error", "canceled", false
	case result.err != nil:
		return "error", "provider", false
	default:
		return "ok", "none", false
	}
}

func exerciseCancellation(parent context.Context, adapter Adapter, fixture Fixture, config Config) error {
	if err := parent.Err(); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(parent, config.CaseTimeout)
	defer cancel()
	startedCh := make(chan struct{})
	cancelIssued := make(chan struct{})
	var once sync.Once
	var duplicate atomic.Bool
	done := make(chan callbackResult[struct{}], 1)
	go func() {
		result := callbackResult[struct{}]{}
		defer func() {
			result.panicValue = recover()
			done <- result
		}()
		result.err = adapter.BlockUntilCanceled(ctx, fixture, func() {
			called := false
			once.Do(func() {
				called = true
				close(startedCh)
			})
			if !called {
				duplicate.Store(true)
			}
			<-cancelIssued
		})
	}()
	timer := time.NewTimer(config.CaseTimeout)
	defer timer.Stop()
	select {
	case <-startedCh:
		cancel()
		close(cancelIssued)
	case result := <-done:
		cancel()
		close(cancelIssued)
		if result.panicValue != nil {
			return errors.New("graphtest: cancellation callback panic")
		}
		return errCancellationReturnedBeforeStart
	case <-timer.C:
		cancel()
		close(cancelIssued)
		result := <-done
		return errors.Join(errCancellationStartTimeout, callbackError("cancellation", result))
	}
	grace := config.CaseTimeout / 10
	if config.CloseTimeout < grace {
		grace = config.CloseTimeout
	}
	graceTimer := time.NewTimer(grace)
	defer graceTimer.Stop()
	var result callbackResult[struct{}]
	select {
	case result = <-done:
	case <-graceTimer.C:
		result = <-done
	}
	if duplicate.Load() {
		return errCancellationDuplicateStart
	}
	if result.panicValue != nil {
		return errors.New("graphtest: cancellation callback panic")
	}
	return result.err
}

func cleanupAndClose(parent context.Context, adapter Adapter, fixture Fixture, config Config) error {
	base := context.WithoutCancel(parent)
	cleanupResult := call(base, config.CleanupTimeout, func(ctx context.Context) (struct{}, error) {
		return struct{}{}, adapter.CleanupFixture(ctx, fixture)
	})
	closeResult := call(base, config.CloseTimeout, func(ctx context.Context) (struct{}, error) {
		return struct{}{}, adapter.Close(ctx)
	})
	return errors.Join(
		callbackError("fixture cleanup", cleanupResult),
		callbackError("close", closeResult),
	)
}
