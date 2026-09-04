package redissem

import (
	"context"
	"errors"
	"fmt"

	btredis "github.com/bluetape4k/bluetape-go/redis"
	"github.com/redis/go-redis/v9"
)

// Semaphore is a bounded Redis semaphore whose permits expire after their TTL.
type Semaphore struct {
	client redis.Cmdable
	opts   options
	keys   keySet
}

// Lease is an owner-safe semaphore permit.
type Lease struct {
	semaphore *Semaphore
	key       string
	owner     btredis.OwnerToken
}

// New creates a Semaphore using the caller-owned Redis client. New does not
// ping or otherwise contact Redis.
func New(client redis.Cmdable, opts Options) (*Semaphore, error) {
	normalized, err := opts.normalize(client)
	if err != nil {
		return nil, err
	}
	keys, err := buildKeys(normalized.key)
	if err != nil {
		return nil, err
	}
	return &Semaphore{client: client, opts: normalized, keys: keys}, nil
}

// Key returns the logical key configured for the semaphore.
func (s *Semaphore) Key() string {
	if s == nil {
		return ""
	}
	return s.opts.key
}

// Permits returns the configured maximum number of live leases.
func (s *Semaphore) Permits() int {
	if s == nil {
		return 0
	}
	return s.opts.permits
}

// TryAcquire attempts one immediate permit acquisition. ErrNotAcquired means
// that all permits are currently live; provider errors are not retried.
func (s *Semaphore) TryAcquire(ctx context.Context) (*Lease, error) {
	if s == nil {
		return nil, fmt.Errorf("%w: nil semaphore", btredis.ErrInvalidKey)
	}
	ctx = normalizeContext(ctx)
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	owner, err := btredis.NewOwnerToken()
	if err != nil {
		return nil, fmt.Errorf("generate semaphore owner token: %w", err)
	}
	ttlMillis, err := btredis.TTLMillis("semaphore", s.opts.ttl)
	if err != nil {
		return nil, err
	}
	cmd := acquireScript.Run(ctx, s.client, []string{s.keys.leases}, s.opts.permits, ttlMillis, owner.RedisValue())
	acquired, err := parseAcquireResult(cmd)
	if err != nil {
		return nil, operationError("acquire", s.keys.keyID, err)
	}
	if !acquired {
		return nil, ErrNotAcquired
	}
	return &Lease{semaphore: s, key: s.opts.key, owner: owner}, nil
}

// Release removes this exact owner-token member. It is idempotent: an expired
// or mismatched lease returns false, nil.
func (l *Lease) Release(ctx context.Context) (bool, error) {
	if l == nil || l.semaphore == nil {
		return false, nil
	}
	ctx = normalizeContext(ctx)
	if err := ctx.Err(); err != nil {
		return false, err
	}
	cmd := releaseScript.Run(ctx, l.semaphore.client, []string{l.semaphore.keys.leases}, l.owner.RedisValue())
	result, err := cmd.Int64()
	if err != nil {
		return false, operationError("release", l.semaphore.keys.keyID, err)
	}
	switch result {
	case 0:
		return false, nil
	case 1:
		return true, nil
	default:
		return false, operationError("release", l.semaphore.keys.keyID, fmt.Errorf("unexpected semaphore release result %d", result))
	}
}

// Key returns the logical key associated with this lease.
func (l *Lease) Key() string {
	if l == nil {
		return ""
	}
	return l.key
}

// OwnerToken returns the opaque owner token for this lease.
func (l *Lease) OwnerToken() btredis.OwnerToken {
	if l == nil {
		return btredis.OwnerToken{}
	}
	return l.owner
}

func operationError(operation, keyID string, err error) error {
	wrapped := btredis.NewOpErrorWithRedactedKey(
		btredis.OpLabels{Family: "redis semaphore", Operation: operation},
		keyID,
		err,
	)
	return errors.Join(wrapped, btredis.ErrCommitUnknown)
}

func normalizeContext(ctx context.Context) context.Context {
	if ctx == nil {
		return context.Background()
	}
	return ctx
}
