package redislock

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	btredis "github.com/bluetape4k/bluetape-go/redis"
	"github.com/redis/go-redis/v9"
)

// FencedLock is a Redis-backed lock whose successful acquisitions receive a
// monotonically increasing fencing token.
type FencedLock struct {
	client redis.Cmdable
	opts   options
	keys   keySet
}

// Lease is the ownership returned by a successful FencedLock acquisition.
type Lease struct {
	lock    *FencedLock
	key     string
	owner   btredis.OwnerToken
	fencing uint64
	shared  btredis.Lease
}

// New creates a FencedLock using the caller-owned Redis client. New does not
// ping or otherwise contact Redis.
func New(client redis.Cmdable, opts Options) (*FencedLock, error) {
	normalized, err := opts.normalize(client)
	if err != nil {
		return nil, err
	}
	keys, err := buildKeys(normalized.key)
	if err != nil {
		return nil, err
	}
	return &FencedLock{client: client, opts: normalized, keys: keys}, nil
}

// Key returns the logical key configured for the lock.
func (l *FencedLock) Key() string {
	if l == nil {
		return ""
	}
	return l.opts.key
}

// TryAcquire attempts one immediate acquisition. ErrNotAcquired means that
// the lock is currently held; provider errors are returned without retrying.
func (l *FencedLock) TryAcquire(ctx context.Context) (*Lease, error) {
	if l == nil {
		return nil, fmt.Errorf("%w: nil lock", btredis.ErrInvalidKey)
	}
	ctx = normalizeContext(ctx)
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	owner, err := btredis.NewOwnerToken()
	if err != nil {
		return nil, fmt.Errorf("generate fenced lock owner token: %w", err)
	}
	millis, err := btredis.TTLMillis("lock", l.opts.ttl)
	if err != nil {
		return nil, err
	}
	cmd := acquireScript.Run(ctx, l.client, []string{l.keys.owner, l.keys.counter}, owner.RedisValue(), millis)
	acquired, fence, err := parseAcquireResult(cmd)
	if err != nil {
		return l.reconcileAcquire(owner, err)
	}
	if !acquired {
		return nil, ErrNotAcquired
	}
	return l.newLease(owner, fence)
}

func (l *FencedLock) newLease(owner btredis.OwnerToken, fence uint64) (*Lease, error) {
	shared, err := btredis.NewLease(l.keys.owner, owner)
	if err != nil {
		return nil, err
	}
	return &Lease{lock: l, key: l.opts.key, owner: owner, fencing: fence, shared: shared}, nil
}

func (l *FencedLock) reconcileAcquire(owner btredis.OwnerToken, cause error) (*Lease, error) {
	probeCtx, cancel := context.WithTimeout(context.Background(), min(l.opts.ttl, 250*time.Millisecond))
	defer cancel()

	value, ownerErr := l.client.Get(probeCtx, l.keys.owner).Result()
	if ownerErr == nil && value == owner.RedisValue() {
		rawFence, counterErr := l.client.Get(probeCtx, l.keys.counter).Result()
		if counterErr == nil {
			fence, parseErr := strconv.ParseUint(rawFence, 10, 64)
			if parseErr == nil && fence > 0 {
				lease, leaseErr := l.newLease(owner, fence)
				if leaseErr == nil {
					return lease, nil
				}
				return nil, operationError("acquire", l.keys.keyID, errors.Join(cause, leaseErr))
			}
			return nil, operationError("acquire", l.keys.keyID, errors.Join(cause, parseErr))
		}
		return nil, operationError("acquire", l.keys.keyID, errors.Join(cause, counterErr))
	}
	return nil, operationError("acquire", l.keys.keyID, errors.Join(cause, ownerErr))
}

func operationError(operation, keyID string, err error) error {
	wrapped := btredis.NewOpErrorWithRedactedKey(
		btredis.OpLabels{Family: "redis fenced lock", Operation: operation},
		keyID,
		err,
	)
	return errors.Join(wrapped, btredis.ErrCommitUnknown)
}

// Release removes the lease only when its owner token still matches. It is
// idempotent: an expired lease or a mismatched owner returns false, nil.
func (l *Lease) Release(ctx context.Context) (bool, error) {
	if l == nil || l.lock == nil {
		return false, nil
	}
	ctx = normalizeContext(ctx)
	if err := ctx.Err(); err != nil {
		return false, err
	}
	released, err := btredis.CompareAndDelete(ctx, l.lock.client, l.shared, "redis fenced lock")
	if err != nil {
		return false, operationError("release", l.lock.keys.keyID, err)
	}
	return released, nil
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

// FencingToken returns the monotonically increasing token assigned at acquire.
func (l *Lease) FencingToken() uint64 {
	if l == nil {
		return 0
	}
	return l.fencing
}

func normalizeContext(ctx context.Context) context.Context {
	if ctx == nil {
		return context.Background()
	}
	return ctx
}
