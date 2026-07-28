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

const groupAcquireScript = `
local t = redis.call("TIME")
local now = tonumber(t[1]) * 1000 + math.floor(tonumber(t[2]) / 1000)
redis.call("ZREMRANGEBYSCORE", KEYS[1], 0, now)
if redis.call("ZCARD", KEYS[1]) < tonumber(ARGV[1]) then
	redis.call("ZADD", KEYS[1], now + tonumber(ARGV[3]), ARGV[2])
	redis.call("PEXPIRE", KEYS[1], tonumber(ARGV[3]) + 5000)
	return 1
end
return 0
`

const groupReleaseScript = `
return redis.call("ZREM", KEYS[1], ARGV[1])
`

const groupRenewScript = `
local t = redis.call("TIME")
local now = tonumber(t[1]) * 1000 + math.floor(tonumber(t[2]) / 1000)
redis.call("ZREMRANGEBYSCORE", KEYS[1], 0, now)
if redis.call("ZSCORE", KEYS[1], ARGV[1]) then
	redis.call("ZADD", KEYS[1], "XX", now + tonumber(ARGV[2]), ARGV[1])
	redis.call("PEXPIRE", KEYS[1], tonumber(ARGV[2]) + 5000)
	return 1
end
return 0
`

const groupStatusScript = `
local t = redis.call("TIME")
local now = tonumber(t[1]) * 1000 + math.floor(tonumber(t[2]) / 1000)
redis.call("ZREMRANGEBYSCORE", KEYS[1], 0, now)
local active = redis.call("ZCARD", KEYS[1])
return active
`

const groupPollInterval = 50 * time.Millisecond

// GroupElector struct 공개 타입이며 leader election의 lease, owner token, fencing, group key 계약을 보존한다.
// 필드와 zero value, nil 허용 여부, 동시성 소유권은 생성자와 메서드의 한국어 주석 및 테스트 계약을 따른다.
type GroupElector struct {
	client redis.Cmdable
	opts   leader.GroupOptions
	key    string
	token  string

	mu     sync.RWMutex
	owned  bool
	active bool
	cancel context.CancelFunc
	done   chan struct{}
}

// NewGroup NewGroup 공개 API의 동작을 수행하며 leader election의 lease, owner token, fencing, group key 계약을 보존한다.
//
// 매개변수:
//   - client: Redis backend client 또는 fixture다. 연결과 종료 소유권은 생성자 계약을 따른다.
//   - opts: NewGroup 동작에 필요한 opts 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//
// 반환 오류는 입력 검증 실패, 취소, Redis/backend 실패, lease/token 불일치, 또는 package sentinel/typed error 계약을 보존한다.
func NewGroup(client redis.Cmdable, opts leader.GroupOptions) (*GroupElector, error) {
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

	return &GroupElector{
		client: client,
		opts:   normalized,
		key:    fmt.Sprintf("%s:%s", normalized.KeyPrefix, normalized.Group),
		token:  token,
	}, nil
}

// Campaign Campaign 공개 API의 동작을 수행하며 leader election의 lease, owner token, fencing, group key 계약을 보존한다.
//
// 매개변수:
//   - ctx: 호출자가 소유한 취소, deadline, 요청 범위를 전달한다.
//
// 반환 오류는 입력 검증 실패, 취소, Redis/backend 실패, lease/token 불일치, 또는 package sentinel/typed error 계약을 보존한다.
func (e *GroupElector) Campaign(ctx context.Context) error {
	e.mu.Lock()
	if e.owned || e.active {
		e.mu.Unlock()
		return leader.ErrAlreadyLeader
	}
	e.active = true
	e.mu.Unlock()

	ticker := time.NewTicker(groupPollInterval)
	defer ticker.Stop()
	defer e.clearCampaigning()

	for {
		ok, err := e.acquire(ctx)
		if err != nil {
			return btredis.NewOpError(btredis.OpLabels{Family: "leader redis group", Operation: "campaign"}, e.key, err)
		}
		if ok {
			renewCtx, cancel := context.WithCancel(context.Background())
			done := make(chan struct{})

			e.mu.Lock()
			e.owned = true
			e.active = false
			e.cancel = cancel
			e.done = done
			e.mu.Unlock()

			go e.renewLoop(renewCtx, done)
			return nil
		}

		select {
		case <-ctx.Done():
			return fmt.Errorf("redis leader group campaign: %w", ctx.Err())
		case <-ticker.C:
		}
	}
}

// Resign Resign 공개 API의 동작을 수행하며 leader election의 lease, owner token, fencing, group key 계약을 보존한다.
//
// 매개변수:
//   - ctx: 호출자가 소유한 취소, deadline, 요청 범위를 전달한다.
//
// 반환 오류는 입력 검증 실패, 취소, Redis/backend 실패, lease/token 불일치, 또는 package sentinel/typed error 계약을 보존한다.
func (e *GroupElector) Resign(ctx context.Context) error {
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
	e.active = false
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

	if _, err := e.client.Eval(ctx, groupReleaseScript, []string{e.key}, e.token).Result(); err != nil {
		return btredis.NewOpError(btredis.OpLabels{Family: "leader redis group", Operation: "resign"}, e.key, err)
	}
	return nil
}

// IsLeader IsLeader 공개 API의 동작을 수행하며 leader election의 lease, owner token, fencing, group key 계약을 보존한다.
func (e *GroupElector) IsLeader() bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.owned
}

// ActiveCount ActiveCount 공개 API의 동작을 수행하며 leader election의 lease, owner token, fencing, group key 계약을 보존한다.
//
// 매개변수:
//   - ctx: 호출자가 소유한 취소, deadline, 요청 범위를 전달한다.
//
// 반환 오류는 입력 검증 실패, 취소, Redis/backend 실패, lease/token 불일치, 또는 package sentinel/typed error 계약을 보존한다.
func (e *GroupElector) ActiveCount(ctx context.Context) (int, error) {
	active, err := e.activeCount(ctx)
	if err != nil {
		return 0, btredis.NewOpError(btredis.OpLabels{Family: "leader redis group", Operation: "active_count"}, e.key, err)
	}
	return active, nil
}

// AvailableSlots AvailableSlots 공개 API의 동작을 수행하며 leader election의 lease, owner token, fencing, group key 계약을 보존한다.
//
// 매개변수:
//   - ctx: 호출자가 소유한 취소, deadline, 요청 범위를 전달한다.
//
// 반환 오류는 입력 검증 실패, 취소, Redis/backend 실패, lease/token 불일치, 또는 package sentinel/typed error 계약을 보존한다.
func (e *GroupElector) AvailableSlots(ctx context.Context) (int, error) {
	active, err := e.activeCount(ctx)
	if err != nil {
		return 0, btredis.NewOpError(btredis.OpLabels{Family: "leader redis group", Operation: "available_slots"}, e.key, err)
	}
	available := e.opts.MaxLeaders - active
	if available < 0 {
		return 0, nil
	}
	return available, nil
}

func (e *GroupElector) acquire(ctx context.Context) (bool, error) {
	ttlMillis := int64(e.opts.Lease / time.Millisecond)
	result, err := e.client.Eval(
		ctx,
		groupAcquireScript,
		[]string{e.key},
		e.opts.MaxLeaders,
		e.token,
		ttlMillis,
	).Int()
	if err != nil {
		return false, err
	}
	return result == 1, nil
}

func (e *GroupElector) renewLoop(ctx context.Context, done chan<- struct{}) {
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
				e.active = false
				e.cancel = nil
				e.done = nil
				e.mu.Unlock()
				return
			}
		}
	}
}

func (e *GroupElector) clearCampaigning() {
	e.mu.Lock()
	if !e.owned {
		e.active = false
	}
	e.mu.Unlock()
}

func (e *GroupElector) renew(ctx context.Context) (bool, error) {
	renewCtx, cancel := context.WithTimeout(ctx, e.opts.RenewInterval)
	defer cancel()

	ttlMillis := int64(e.opts.Lease / time.Millisecond)
	result, err := e.client.Eval(renewCtx, groupRenewScript, []string{e.key}, e.token, ttlMillis).Int()
	if err != nil {
		return false, btredis.NewOpError(btredis.OpLabels{Family: "leader redis group", Operation: "renew"}, e.key, err)
	}
	return result == 1, nil
}

func (e *GroupElector) activeCount(ctx context.Context) (int, error) {
	active, err := e.client.Eval(ctx, groupStatusScript, []string{e.key}).Int()
	if err != nil {
		return 0, err
	}
	return active, nil
}

var _ leader.GroupElector = (*GroupElector)(nil)
