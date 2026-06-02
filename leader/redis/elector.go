package redisleader

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/bluetape4k/bluetape-go/leader"
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

	token, err := randomToken()
	if err != nil {
		return nil, err
	}

	return &Elector{
		client: client,
		opts:   normalized,
		key:    fmt.Sprintf("%s:%s", normalized.KeyPrefix, normalized.Group),
		token:  normalized.MemberID + ":" + token,
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
		return fmt.Errorf("redis leader campaign: %w", err)
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
		<-done
	}

	_, err := e.client.Eval(ctx, releaseScript, []string{e.key}, e.token).Result()
	if err != nil {
		return fmt.Errorf("redis leader resign: %w", err)
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
		return "", fmt.Errorf("redis leader lookup: %w", err)
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
	ttlMillis := int64(e.opts.Lease / time.Millisecond)
	result, err := e.client.Eval(ctx, renewScript, []string{e.key}, e.token, ttlMillis).Int()
	if err != nil {
		return false, err
	}
	return result == 1, nil
}

func randomToken() (string, error) {
	var data [16]byte
	if _, err := rand.Read(data[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(data[:]), nil
}
