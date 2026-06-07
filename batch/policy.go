package batch

import (
	"context"
	"errors"
	"fmt"
)

// ErrorPredicate decides whether a policy applies to err.
type ErrorPredicate func(error) bool

// RetryPolicy retries processor and writer failures before skip/fail handling.
type RetryPolicy struct {
	MaxAttempts int
	RetryIf     ErrorPredicate
}

// SkipPolicy skips failed processor items or failed writer chunks after retry
// is exhausted.
type SkipPolicy struct {
	MaxSkips int
	SkipIf   ErrorPredicate
}

// RetryErrors creates a retry policy for matching non-context errors.
func RetryErrors(maxAttempts int, retryIf ErrorPredicate) (RetryPolicy, error) {
	if maxAttempts <= 0 {
		return RetryPolicy{}, fmt.Errorf("max attempts must be positive")
	}
	return RetryPolicy{MaxAttempts: maxAttempts, RetryIf: retryIf}, nil
}

// SkipErrors creates a skip policy for matching non-context errors.
func SkipErrors(maxSkips int, skipIf ErrorPredicate) (SkipPolicy, error) {
	if maxSkips <= 0 {
		return SkipPolicy{}, fmt.Errorf("max skips must be positive")
	}
	return SkipPolicy{MaxSkips: maxSkips, SkipIf: skipIf}, nil
}

func (p RetryPolicy) normalize() (RetryPolicy, error) {
	if p.MaxAttempts == 0 {
		p.MaxAttempts = 1
	}
	if p.MaxAttempts < 0 {
		return p, fmt.Errorf("max attempts must be positive")
	}
	if p.RetryIf == nil && p.MaxAttempts > 1 {
		p.RetryIf = nonContextError
	}
	return p, nil
}

func (p RetryPolicy) shouldRetry(err error, attempt int) bool {
	if err == nil || attempt >= p.MaxAttempts || isContextError(err) {
		return false
	}
	return p.RetryIf != nil && p.RetryIf(err)
}

func (p SkipPolicy) normalize() (SkipPolicy, error) {
	if p.MaxSkips < 0 {
		return p, fmt.Errorf("max skips must not be negative")
	}
	if p.SkipIf == nil && p.MaxSkips > 0 {
		p.SkipIf = nonContextError
	}
	return p, nil
}

func (p SkipPolicy) shouldSkip(err error, used int, cost int) bool {
	if err == nil || cost <= 0 || p.MaxSkips <= 0 || isContextError(err) {
		return false
	}
	if used+cost > p.MaxSkips {
		return false
	}
	return p.SkipIf != nil && p.SkipIf(err)
}

func nonContextError(err error) bool {
	return err != nil && !isContextError(err)
}

func isContextError(err error) bool {
	return errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)
}
