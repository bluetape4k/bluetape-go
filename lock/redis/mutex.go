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

// Mutex 는 Redis key 하나에 대한 owner-token lock이다.
type Mutex struct {
	client redis.Cmdable
	opts   options
}

// Lease 는 성공적으로 획득한 Redis lock 소유권이다.
type Lease struct {
	mutex       *Mutex
	key         string
	token       string
	sharedLease *btredis.Lease
}

// New 는 Redis lock mutex를 만든다.
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

// TryLock 은 lock 획득을 한 번 시도한다.
//
// ErrCommitUnknown과 함께 non-nil Lease가 반환되면 type-first로 오류를 판별한 뒤
// 그 Lease로 bounded Unlock을 즉시 시도해야 한다. 같은 Lease의 Unlock은 재시도할 수
// 있으며 정리가 확인되지 않으면 TTL 만료를 기다려야 한다.
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

// Key 는 Redis lock key를 반환한다.
func (m *Mutex) Key() string {
	return m.opts.key
}

// Key 는 lease가 소유한 Redis lock key를 반환한다.
func (l *Lease) Key() string {
	if l == nil {
		return ""
	}
	return l.key
}

// Token 은 lease owner token을 반환한다.
func (l *Lease) Token() string {
	if l == nil {
		return ""
	}
	return l.token
}

// Unlock 은 현재 token이 아직 owner일 때만 lock key를 제거한다.
//
// ErrCommitUnknown이면 동일 Lease로 bounded Unlock을 재시도한다. false와 nil은 이미
// 삭제됐거나 owner가 교체됐음을 뜻하며 replacement owner는 삭제하지 않는다.
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
