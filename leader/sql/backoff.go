package sqlleader

import (
	"context"
	"time"
)

const campaignBackoffBase = 25 * time.Millisecond

type backoff struct {
	token   string
	cap     time.Duration
	attempt uint
}

func newBackoff(token string, lease time.Duration) *backoff {
	return &backoff{
		token: token,
		cap:   max(campaignBackoffBase, min(lease/4, time.Second)),
	}
}

func (b *backoff) wait(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	delay := b.delay()
	b.attempt++
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func (b *backoff) delay() time.Duration {
	shift := min(b.attempt, uint(6))
	delay := campaignBackoffBase << shift
	if delay > b.cap {
		delay = b.cap
	}

	hash := stableHash(b.token, b.attempt)
	delay = delay * time.Duration(80+hash%41) / 100
	if delay > b.cap {
		return b.cap
	}
	return delay
}

func stableHash(token string, attempt uint) uint64 {
	const (
		fnvOffset64 = uint64(14695981039346656037)
		fnvPrime64  = uint64(1099511628211)
	)
	hash := fnvOffset64
	for i := range len(token) {
		hash ^= uint64(token[i])
		hash *= fnvPrime64
	}
	for shift := range 8 {
		hash ^= uint64(byte(uint64(attempt) >> (8 * shift)))
		hash *= fnvPrime64
	}
	return hash
}
