package redislock

import (
	"context"
	"errors"
	"fmt"
	"time"

	btredis "github.com/bluetape4k/bluetape-go/redis"
	"github.com/redis/go-redis/v9"
)

const unlockScript = `
if redis.call("GET", KEYS[1]) == ARGV[1] then
	return redis.call("DEL", KEYS[1])
end
return 0
`

// Mutex Redis key 하나에 대한 owner-token lock이다.
type Mutex struct {
	client redis.Cmdable
	opts   options
}

// Lease 성공적으로 획득한 Redis lock 소유권이다.
type Lease struct {
	mutex       *Mutex
	key         string
	token       string
	sharedLease *btredis.Lease
}

// New Redis lock mutex를 만든다.
func New(client redis.Cmdable, opts Options) (*Mutex, error) {
	if client == nil {
		return nil, fmt.Errorf("redis client must not be nil")
	}
	normalized, err := opts.normalize()
	if err != nil {
		return nil, err
	}
	return &Mutex{client: client, opts: normalized}, nil
}

// TryLock Redis lock key, owner token, TTL, unlock safety 동작을 수행한다.
//
// 매개변수:
//   - ctx: 호출자가 소유한 취소, deadline, 요청 범위를 전달한다.
//
// 반환 오류는 입력 검증 실패, context 취소, Redis/backend 실패, lock ownership 불일치, quota 거절, package sentinel error와 typed error를 그대로 드러낸다.
func (m *Mutex) TryLock(ctx context.Context) (*Lease, error) {
	ctx = normalizeContext(ctx)
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	token := m.opts.token
	if token == "" {
		generated, err := btredis.NewOwnerToken()
		if err != nil {
			return nil, fmt.Errorf("generate redis lock token: %w", err)
		}
		token = generated.RedisValue()
	}
	sharedLease := sharedLeaseFor(m.opts.key, token)

	ok, err := m.client.SetNX(ctx, m.opts.key, token, m.opts.ttl).Result()
	if err != nil {
		lease := &Lease{mutex: m, key: m.opts.key, token: token, sharedLease: sharedLease}
		confirmed, probeErr := m.reconcileOwner(token)
		if probeErr == nil && confirmed {
			return lease, nil
		}
		wrapped := btredis.NewOpError(
			btredis.OpLabels{Family: "lock", Operation: "acquire"},
			m.opts.key,
			err,
		)
		if probeErr != nil {
			return lease, errors.Join(wrapped, btredis.ErrCommitUnknown)
		}
		if ctx.Err() != nil && errors.Is(err, ctx.Err()) {
			return nil, ctx.Err()
		}
		return nil, wrapped
	}
	if !ok {
		return nil, ErrNotAcquired
	}
	return &Lease{mutex: m, key: m.opts.key, token: token, sharedLease: sharedLease}, nil
}

// Key Redis lock key를 반환한다.
func (m *Mutex) Key() string {
	return m.opts.key
}

// Key lease가 소유한 Redis lock key를 반환한다.
func (l *Lease) Key() string {
	if l == nil {
		return ""
	}
	return l.key
}

// Token Redis lock key, owner token, TTL, unlock safety 동작을 수행한다.
func (l *Lease) Token() string {
	if l == nil {
		return ""
	}
	return l.token
}

// Unlock Redis lock key, owner token, TTL, unlock safety 동작을 수행한다.
//
// 매개변수:
//   - ctx: 호출자가 소유한 취소, deadline, 요청 범위를 전달한다.
//
// 반환 오류는 입력 검증 실패, context 취소, Redis/backend 실패, lock ownership 불일치, quota 거절, package sentinel error와 typed error를 그대로 드러낸다.
func (l *Lease) Unlock(ctx context.Context) (bool, error) {
	if l == nil || l.mutex == nil {
		return false, nil
	}
	ctx = normalizeContext(ctx)
	if err := ctx.Err(); err != nil {
		return false, err
	}
	if l.sharedLease != nil {
		deleted, err := btredis.CompareAndDelete(ctx, l.mutex.client, *l.sharedLease, "lock")
		if err != nil {
			if l.preDispatchCancellation(ctx) {
				return false, ctx.Err()
			}
			return false, errors.Join(err, btredis.ErrCommitUnknown)
		}
		return deleted, nil
	}

	result, err := l.mutex.client.Eval(ctx, unlockScript, []string{l.key}, l.token).Int()
	if err != nil {
		if l.preDispatchCancellation(ctx) {
			return false, ctx.Err()
		}
		return false, errors.Join(btredis.NewOpError(
			btredis.OpLabels{Family: "lock", Operation: "compare-delete"},
			l.key,
			err,
		), btredis.ErrCommitUnknown)
	}
	return result == 1, nil
}

func (l *Lease) preDispatchCancellation(ctx context.Context) bool {
	if ctx == nil || ctx.Err() == nil || l == nil || l.mutex == nil {
		return false
	}
	matched, err := l.mutex.reconcileOwner(l.token)
	return err == nil && matched
}

func (m *Mutex) reconcileOwner(token string) (bool, error) {
	probeCtx, cancel := context.WithTimeout(context.Background(), min(m.opts.ttl, 250*time.Millisecond))
	defer cancel()
	owner, err := m.client.Get(probeCtx, m.opts.key).Result()
	if errors.Is(err, redis.Nil) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return owner == token, nil
}

func normalizeContext(ctx context.Context) context.Context {
	if ctx == nil {
		return context.Background()
	}
	return ctx
}

func sharedLeaseFor(key string, token string) *btredis.Lease {
	ownerToken, parseErr := btredis.ParseOwnerToken(token)
	if parseErr != nil {
		return nil
	}
	lease, err := btredis.NewLease(key, ownerToken)
	if err != nil {
		return nil
	}
	return &lease
}
