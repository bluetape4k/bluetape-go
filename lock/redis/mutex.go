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

// Mutex는 struct 공개 타입이며 Redis lock key, owner token, TTL, unlock safety 계약을 보존한다.
// 필드와 zero value, nil 허용 여부, 동시성 소유권은 생성자와 메서드의 한국어 주석 및 테스트 계약을 따른다.
type Mutex struct {
	client redis.Cmdable
	opts   options
}

// Lease는 struct 공개 타입이며 Redis lock key, owner token, TTL, unlock safety 계약을 보존한다.
// 필드와 zero value, nil 허용 여부, 동시성 소유권은 생성자와 메서드의 한국어 주석 및 테스트 계약을 따른다.
type Lease struct {
	mutex       *Mutex
	key         string
	token       string
	sharedLease *btredis.Lease
}

// New는 New 공개 API의 동작을 수행하며 Redis lock key, owner token, TTL, unlock safety 계약을 보존한다.
//
// 매개변수:
//   - client: Redis backend 또는 conformance provider다. 연결/종료 소유권은 생성자와 harness 계약을 따른다.
//   - opts: New 동작에 필요한 opts 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//
// 반환 오류는 입력 검증 실패, 취소, Redis/backend 실패, lock ownership 불일치, quota 거절, 또는 package sentinel/typed error 계약을 보존한다.
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

// TryLock는 TryLock 공개 API의 동작을 수행하며 Redis lock key, owner token, TTL, unlock safety 계약을 보존한다.
//
// 매개변수:
//   - ctx: 호출자가 소유한 취소, deadline, 요청 범위를 전달한다.
//
// 반환 오류는 입력 검증 실패, 취소, Redis/backend 실패, lock ownership 불일치, quota 거절, 또는 package sentinel/typed error 계약을 보존한다.
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

// Key는 Key 공개 API의 동작을 수행하며 Redis lock key, owner token, TTL, unlock safety 계약을 보존한다.
func (m *Mutex) Key() string {
	return m.opts.key
}

// Key는 Key 공개 API의 동작을 수행하며 Redis lock key, owner token, TTL, unlock safety 계약을 보존한다.
func (l *Lease) Key() string {
	if l == nil {
		return ""
	}
	return l.key
}

// Token는 Token 공개 API의 동작을 수행하며 Redis lock key, owner token, TTL, unlock safety 계약을 보존한다.
func (l *Lease) Token() string {
	if l == nil {
		return ""
	}
	return l.token
}

// Unlock는 Unlock 공개 API의 동작을 수행하며 Redis lock key, owner token, TTL, unlock safety 계약을 보존한다.
//
// 매개변수:
//   - ctx: 호출자가 소유한 취소, deadline, 요청 범위를 전달한다.
//
// 반환 오류는 입력 검증 실패, 취소, Redis/backend 실패, lock ownership 불일치, quota 거절, 또는 package sentinel/typed error 계약을 보존한다.
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
