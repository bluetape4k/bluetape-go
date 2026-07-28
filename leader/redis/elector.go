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

const (
	campaignRetryBase = 25 * time.Millisecond
	campaignRetryCap  = 250 * time.Millisecond
)

// Elector Redis 기반 leader elector다.
type Elector struct {
	client redis.Cmdable
	opts   leader.Options
	key    string
	token  string

	mu          sync.RWMutex
	owned       bool
	campaigning bool
	cleanup     bool
	generation  uint64
	cancel      context.CancelFunc
	done        chan struct{}
}

// New Redis 기반 leader elector를 만든다.
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

// Campaign leader election의 lease, owner token, fencing, group key 동작을 수행한다.
//
// 매개변수:
//   - ctx: 호출자가 소유한 취소, deadline, 요청 범위를 전달한다.
//
// 반환 오류는 입력 검증 실패, context 취소, Redis/backend 실패, lease/token 불일치, package sentinel error와 typed error를 그대로 드러낸다.
func (e *Elector) Campaign(ctx context.Context) error {
	if ctx == nil {
		return leader.ErrInvalidContext
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := e.beginCampaign(); err != nil {
		return err
	}
	defer e.endCampaign()

	var retry uint
	for {
		ok, err := e.client.SetNX(ctx, e.key, e.token, e.opts.Lease).Result()
		if err != nil {
			confirmed, probeErr := e.reconcileCampaign()
			if probeErr == nil && confirmed {
				e.startRenewal()
				return nil
			}
			wrapped := e.operationError("campaign", err)
			if probeErr != nil {
				e.mu.Lock()
				e.cleanup = true
				e.mu.Unlock()
				return errors.Join(wrapped, leader.ErrCommitUnknown, btredis.ErrCommitUnknown)
			}
			return wrapped
		}
		if ok {
			e.startRenewal()
			return nil
		}
		if err := sleepContext(ctx, campaignRetryDelay(e.token, retry)); err != nil {
			return err
		}
		retry++
	}
}

// Resign leader election의 lease, owner token, fencing, group key 동작을 수행한다.
//
// 매개변수:
//   - ctx: 호출자가 소유한 취소, deadline, 요청 범위를 전달한다.
//
// 반환 오류는 입력 검증 실패, context 취소, Redis/backend 실패, lease/token 불일치, package sentinel error와 typed error를 그대로 드러낸다.
func (e *Elector) Resign(ctx context.Context) error {
	if ctx == nil {
		return leader.ErrInvalidContext
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	e.mu.Lock()
	if !e.owned && !e.cleanup {
		e.mu.Unlock()
		return nil
	}
	generation := e.generation
	cancel := e.cancel
	done := e.done
	e.owned = false
	e.cleanup = true
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
		return errors.Join(e.operationError("resign", err), leader.ErrCommitUnknown, btredis.ErrCommitUnknown)
	}
	e.mu.Lock()
	if e.generation == generation {
		e.cleanup = false
		e.cancel = nil
		e.done = nil
	}
	e.mu.Unlock()
	return nil
}

// IsLeader 이 elector가 아직 leader라고 판단하는지 알려준다.
func (e *Elector) IsLeader() bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.owned
}

// Leader Redis에 기록된 현재 leader token을 반환한다.
func (e *Elector) Leader(ctx context.Context) (string, error) {
	if ctx == nil {
		return "", leader.ErrInvalidContext
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	value, err := e.client.Get(ctx, e.key).Result()
	if errors.Is(err, redis.Nil) {
		return "", nil
	}
	if err != nil {
		return "", e.operationError("lookup", err)
	}
	return value, nil
}

func (e *Elector) renewLoop(ctx context.Context, generation uint64, done chan struct{}) {
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
				e.clearOwnershipAfterLoss(generation, done, err != nil)
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
		return false, e.operationError("renew", err)
	}
	return result == 1, nil
}

func (e *Elector) beginCampaign() error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.cleanup {
		return leader.ErrCleanupPending
	}
	if e.owned {
		return leader.ErrAlreadyLeader
	}
	if e.campaigning {
		return leader.ErrCampaignInProgress
	}
	e.campaigning = true
	return nil
}

func (e *Elector) endCampaign() {
	e.mu.Lock()
	e.campaigning = false
	e.mu.Unlock()
}

func (e *Elector) startRenewal() {
	renewCtx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	e.mu.Lock()
	e.generation++
	generation := e.generation
	e.owned = true
	e.cleanup = false
	e.cancel = cancel
	e.done = done
	e.mu.Unlock()
	go e.renewLoop(renewCtx, generation, done)
}

func (e *Elector) clearOwnershipAfterLoss(generation uint64, done chan struct{}, cleanup bool) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.generation != generation || e.done != done {
		return
	}
	e.owned = false
	e.cleanup = cleanup
	e.cancel = nil
	e.done = nil
}

func (e *Elector) reconcileCampaign() (bool, error) {
	probeCtx, cancel := context.WithTimeout(context.Background(), min(e.opts.RenewInterval, 250*time.Millisecond))
	defer cancel()
	owner, err := e.client.Get(probeCtx, e.key).Result()
	if errors.Is(err, redis.Nil) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return owner == e.token, nil
}

func (e *Elector) operationError(operation string, cause error) error {
	redisErr := btredis.NewOpError(
		btredis.OpLabels{Family: "leader redis", Operation: operation},
		e.key,
		cause,
	)
	return leader.NewOperationError("redis", operation, redisErr)
}

func sleepContext(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func campaignRetryDelay(token string, attempt uint) time.Duration {
	shift := min(attempt, uint(4))
	delay := campaignRetryBase << shift
	if delay > campaignRetryCap {
		delay = campaignRetryCap
	}

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
	jitterPercent := time.Duration(80 + hash%41)
	return delay * jitterPercent / 100
}

func newElectorToken(memberID string) (string, error) {
	token, err := btredis.NewOwnerToken()
	if err != nil {
		return "", err
	}
	return memberID + ":" + token.RedisValue(), nil
}
