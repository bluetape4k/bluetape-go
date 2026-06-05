package redislock

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"

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
	mutex *Mutex
	key   string
	token string
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
func (m *Mutex) TryLock(ctx context.Context) (*Lease, error) {
	ctx = normalizeContext(ctx)
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	token := m.opts.token
	if token == "" {
		generated, err := randomToken()
		if err != nil {
			return nil, fmt.Errorf("generate redis lock token: %w", err)
		}
		token = generated
	}

	ok, err := m.client.SetNX(ctx, m.opts.key, token, m.opts.ttl).Result()
	if err != nil {
		return nil, fmt.Errorf("redis lock acquire: %w", err)
	}
	if !ok {
		return nil, ErrNotAcquired
	}
	return &Lease{mutex: m, key: m.opts.key, token: token}, nil
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
func (l *Lease) Unlock(ctx context.Context) (bool, error) {
	if l == nil || l.mutex == nil {
		return false, nil
	}
	ctx = normalizeContext(ctx)
	if err := ctx.Err(); err != nil {
		return false, err
	}

	result, err := l.mutex.client.Eval(ctx, unlockScript, []string{l.key}, l.token).Int()
	if err != nil {
		return false, fmt.Errorf("redis lock unlock: %w", err)
	}
	return result == 1, nil
}

func normalizeContext(ctx context.Context) context.Context {
	if ctx == nil {
		return context.Background()
	}
	return ctx
}

func randomToken() (string, error) {
	var data [16]byte
	if _, err := rand.Read(data[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(data[:]), nil
}
