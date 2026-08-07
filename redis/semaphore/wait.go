package redissem

import (
	"context"
	"errors"
	"time"
)

const (
	initialRetryDelay = 5 * time.Millisecond
	maxRetryDelay     = 100 * time.Millisecond
)

func waitForRetry(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func nextRetryDelay(delay time.Duration) time.Duration {
	if delay >= maxRetryDelay {
		return maxRetryDelay
	}
	next := delay * 2
	if next > maxRetryDelay {
		return maxRetryDelay
	}
	return next
}

// Acquire waits until a permit is available or ctx is canceled/deadlined.
func (s *Semaphore) Acquire(ctx context.Context) (*Lease, error) {
	ctx = normalizeContext(ctx)
	delay := initialRetryDelay
	for {
		lease, err := s.TryAcquire(ctx)
		if !errors.Is(err, ErrNotAcquired) {
			return lease, err
		}
		if err := waitForRetry(ctx, delay); err != nil {
			return nil, err
		}
		delay = nextRetryDelay(delay)
	}
}
