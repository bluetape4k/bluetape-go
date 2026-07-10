package redisleader

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/bluetape4k/bluetape-go/leader"
	btredis "github.com/bluetape4k/bluetape-go/redis"
	"github.com/redis/go-redis/v9"
)

const releaseScript = `
if redis.call("GET", KEYS[1]) == ARGV[1] then
	return redis.call("DEL", KEYS[1])
end
return 0
`

const renewScript = `
if redis.call("GET", KEYS[1]) == ARGV[1] then
	return redis.call("PEXPIRE", KEYS[1], ARGV[2])
end
return 0
`

// Elector 는 Redis 기반 leader elector다.
type Elector struct {
	client redis.Cmdable
	opts   leader.Options
	key    string
	token  string

	mu     sync.RWMutex
	owned  bool
	cancel context.CancelFunc
	done   chan struct{}
}

// New 는 Redis 기반 leader elector를 만든다.
func New(client redis.Cmdable, opts leader.Options) (*Elector, error) {
	if client == nil {
		return nil, errors.New("redis client must not be nil")
	}

	normalized, err := opts.Normalize()
	if err != nil {
		return nil, err
	}

	token, err := newElectorToken(normalized.MemberID)
	if err != nil {
		return nil, err
	}

	return &Elector{
		client: client,
		opts:   normalized,
		key:    fmt.Sprintf("%s:%s", normalized.KeyPrefix, normalized.Group),
		token:  token,
	}, nil
}

// Campaign 은 이 member의 leadership 획득을 시도한다.
func (e *Elector) Campaign(ctx context.Context) error {
	e.mu.Lock()
	if e.owned {
		e.mu.Unlock()
		return leader.ErrAlreadyLeader
	}
	e.mu.Unlock()

	ok, err := e.client.SetNX(ctx, e.key, e.token, e.opts.Lease).Result()
	if err != nil {
		return btredis.NewOpError(btredis.OpLabels{Family: "leader redis", Operation: "campaign"}, e.key, err)
	}
	if !ok {
		return leader.ErrNotLeader
	}

	renewCtx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})

	e.mu.Lock()
	e.owned = true
	e.cancel = cancel
	e.done = done
	e.mu.Unlock()

	go e.renewLoop(renewCtx, done)
	return nil
}

// Resign 은 이 elector가 아직 소유한 leadership만 해제한다.
func (e *Elector) Resign(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}
	e.mu.Lock()
	if !e.owned {
		e.mu.Unlock()
		return nil
	}
	cancel := e.cancel
	done := e.done
	e.owned = false
	e.cancel = nil
	e.done = nil
	e.mu.Unlock()

	if cancel != nil {
		cancel()
	}
	if done != nil {
		select {
		case <-done:
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	_, err := e.client.Eval(ctx, releaseScript, []string{e.key}, e.token).Result()
	if err != nil {
		return btredis.NewOpError(btredis.OpLabels{Family: "leader redis", Operation: "resign"}, e.key, err)
	}
	return nil
}

// IsLeader 는 이 elector가 아직 leader라고 판단하는지 알려준다.
func (e *Elector) IsLeader() bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.owned
}

// Leader 는 Redis에 기록된 현재 leader token을 반환한다.
func (e *Elector) Leader(ctx context.Context) (string, error) {
	value, err := e.client.Get(ctx, e.key).Result()
	if errors.Is(err, redis.Nil) {
		return "", nil
	}
	if err != nil {
		return "", btredis.NewOpError(btredis.OpLabels{Family: "leader redis", Operation: "lookup"}, e.key, err)
	}
	return value, nil
}

func (e *Elector) renewLoop(ctx context.Context, done chan<- struct{}) {
	defer close(done)

	ticker := time.NewTicker(e.opts.RenewInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			ok, err := e.renew(ctx)
			if err != nil || !ok {
				e.mu.Lock()
				e.owned = false
				e.cancel = nil
				e.done = nil
				e.mu.Unlock()
				return
			}
		}
	}
}

func (e *Elector) renew(ctx context.Context) (bool, error) {
	renewCtx, cancel := context.WithTimeout(ctx, e.opts.RenewInterval)
	defer cancel()

	ttlMillis := int64(e.opts.Lease / time.Millisecond)
	result, err := e.client.Eval(renewCtx, renewScript, []string{e.key}, e.token, ttlMillis).Int()
	if err != nil {
		return false, btredis.NewOpError(btredis.OpLabels{Family: "leader redis", Operation: "renew"}, e.key, err)
	}
	return result == 1, nil
}

func newElectorToken(memberID string) (string, error) {
	token, err := btredis.NewOwnerToken()
	if err != nil {
		return "", err
	}
	return memberID + ":" + token.RedisValue(), nil
}
