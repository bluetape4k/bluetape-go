package redisleader

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/bluetape4k/bluetape-go/core"
	"github.com/bluetape4k/bluetape-go/leader"
	"github.com/redis/go-redis/v9"
)

const (
	defaultStrategicKeyPrefix = "bluetape:leader-strategy"
	strategicIndexTTLBuffer   = 5 * time.Second
)

const strategicRegisterScript = `
local t = redis.call("TIME")
local now = tonumber(t[1]) * 1000 + math.floor(tonumber(t[2]) / 1000)
local ttl = tonumber(ARGV[2])
redis.call("SET", KEYS[1], ARGV[1], "PX", ttl)
redis.call("ZADD", KEYS[2], now + ttl, ARGV[3])
redis.call("PEXPIRE", KEYS[2], ttl + tonumber(ARGV[4]))
return 1
`

const strategicListScript = `
local t = redis.call("TIME")
local now = tonumber(t[1]) * 1000 + math.floor(tonumber(t[2]) / 1000)
redis.call("ZREMRANGEBYSCORE", KEYS[1], 0, now)
local ids = redis.call("ZRANGE", KEYS[1], 0, -1)
local result = {}
for _, id in ipairs(ids) do
	local value = redis.call("GET", ARGV[1] .. id)
	if value then
		table.insert(result, value)
	else
		redis.call("ZREM", KEYS[1], id)
	end
end
return result
`

const strategicUpdateResultScript = `
local value = redis.call("GET", KEYS[1])
if not value then
	return 0
end
local ttl = redis.call("PTTL", KEYS[1])
if ttl <= 0 then
	return 0
end
local candidate = cjson.decode(value)
if ARGV[1] == "success" then
	candidate["SuccessCount"] = (candidate["SuccessCount"] or 0) + 1
elseif ARGV[1] == "failure" then
	candidate["FailureCount"] = (candidate["FailureCount"] or 0) + 1
else
	return -1
end
candidate["LastCompletedAt"] = ARGV[2]
redis.call("SET", KEYS[1], cjson.encode(candidate), "PX", ttl)
return 1
`

// StrategicElector는 leader backend election에서 caller-visible 상태와 의미를 설명한다.
type StrategicElector[T any] struct {
	client redis.Cmdable
	opts   leader.Options
}

// NewStrategic는 leader backend election에서 생성과 초기화 계약을 설명한다.
func NewStrategic[T any](client redis.Cmdable, opts leader.Options) (*StrategicElector[T], error) {
	if client == nil {
		return nil, errors.New("redis client must not be nil")
	}
	if opts.KeyPrefix == "" {
		opts.KeyPrefix = defaultStrategicKeyPrefix
	}

	normalized, err := opts.Normalize()
	if err != nil {
		return nil, err
	}
	return &StrategicElector[T]{client: client, opts: normalized}, nil
}

// RegisterCandidate는 leader backend election에서 caller-visible 상태와 의미를 설명한다.
func (e *StrategicElector[T]) RegisterCandidate(
	ctx context.Context,
	group string,
	info leader.CandidateInfo,
	ttl time.Duration,
) error {
	if err := validateGroup(group); err != nil {
		return err
	}
	if err := core.RequireNotBlank("nodeID", info.NodeID); err != nil {
		return err
	}
	if err := core.RequirePositive("ttl", ttl); err != nil {
		return err
	}
	if info.RegisteredAt.IsZero() {
		info.RegisteredAt = time.Now().UTC()
	} else {
		info.RegisteredAt = info.RegisteredAt.UTC()
	}
	info.LastStartedAt = normalizeTime(info.LastStartedAt)
	info.LastCompletedAt = normalizeTime(info.LastCompletedAt)

	payload, err := json.Marshal(info)
	if err != nil {
		return fmt.Errorf("redis strategic candidate encode: %w", err)
	}

	ttlMillis := durationMillis(ttl)
	bufferMillis := int64(strategicIndexTTLBuffer / time.Millisecond)
	_, err = e.client.Eval(
		ctx,
		strategicRegisterScript,
		[]string{e.candidateKey(group, info.NodeID), e.indexKey(group)},
		string(payload),
		ttlMillis,
		info.NodeID,
		bufferMillis,
	).Result()
	if err != nil {
		return fmt.Errorf("redis strategic candidate register: %w", err)
	}
	return nil
}

// UnregisterCandidate는 leader backend election에서 caller-visible 상태와 의미를 설명한다.
func (e *StrategicElector[T]) UnregisterCandidate(ctx context.Context, group string, nodeID string) error {
	if err := validateGroup(group); err != nil {
		return err
	}
	if err := core.RequireNotBlank("nodeID", nodeID); err != nil {
		return err
	}

	if err := e.client.Del(ctx, e.candidateKey(group, nodeID)).Err(); err != nil {
		return fmt.Errorf("redis strategic candidate unregister: %w", err)
	}
	if err := e.client.ZRem(ctx, e.indexKey(group), nodeID).Err(); err != nil {
		return fmt.Errorf("redis strategic candidate unregister index: %w", err)
	}
	return nil
}

// ListCandidates는 leader backend election에서 반환값과 오류 의미를 설명한다.
func (e *StrategicElector[T]) ListCandidates(ctx context.Context, group string) ([]leader.CandidateInfo, error) {
	if err := validateGroup(group); err != nil {
		return nil, err
	}

	values, err := e.client.Eval(
		ctx,
		strategicListScript,
		[]string{e.indexKey(group)},
		e.candidateKeyPrefix(group),
	).StringSlice()
	if err != nil {
		return nil, fmt.Errorf("redis strategic candidate list: %w", err)
	}

	candidates := make([]leader.CandidateInfo, 0, len(values))
	for _, value := range values {
		var candidate leader.CandidateInfo
		if err := json.Unmarshal([]byte(value), &candidate); err != nil {
			return nil, fmt.Errorf("redis strategic candidate decode: %w", err)
		}
		candidates = append(candidates, candidate)
	}
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].NodeID < candidates[j].NodeID
	})
	return candidates, nil
}

// UpdateResult는 leader backend election에서 동작과 caller-visible 계약을 설명한다.
func (e *StrategicElector[T]) UpdateResult(
	ctx context.Context,
	group string,
	nodeID string,
	result leader.CandidateResult,
) error {
	if err := validateGroup(group); err != nil {
		return err
	}
	if err := core.RequireNotBlank("nodeID", nodeID); err != nil {
		return err
	}

	var outcome string
	switch result {
	case leader.CandidateSucceeded:
		outcome = "success"
	case leader.CandidateFailed:
		outcome = "failure"
	default:
		return fmt.Errorf("redis strategic candidate result: unknown result %d", result)
	}

	updated, err := e.client.Eval(
		ctx,
		strategicUpdateResultScript,
		[]string{e.candidateKey(group, nodeID)},
		outcome,
		time.Now().UTC().Format(time.RFC3339Nano),
	).Int()
	if err != nil {
		return fmt.Errorf("redis strategic candidate result store: %w", err)
	}
	if updated == 0 {
		return leader.ErrNotLeader
	}
	return nil
}

// RunIfLeader는 leader backend election에서 실행, cancellation, cleanup 계약을 설명한다.
func (e *StrategicElector[T]) RunIfLeader(
	ctx context.Context,
	group string,
	strategy leader.ElectionStrategy,
	action func(context.Context) (T, error),
) (T, bool, error) {
	var zero T
	if strategy == nil {
		return zero, false, errors.New("strategy must not be nil")
	}
	if action == nil {
		return zero, false, errors.New("action must not be nil")
	}

	candidates, err := e.ListCandidates(ctx, group)
	if err != nil {
		return zero, false, err
	}
	winner, ok := strategy.Elect(candidates)
	if !ok || winner.NodeID != e.opts.MemberID {
		return zero, false, nil
	}

	result, actionErr := action(ctx)
	outcome := leader.CandidateSucceeded
	if actionErr != nil {
		outcome = leader.CandidateFailed
	}
	updateErr := e.UpdateResult(ctx, group, e.opts.MemberID, outcome)
	if actionErr != nil || updateErr != nil {
		return result, true, errors.Join(actionErr, updateErr)
	}
	return result, true, nil
}

func (e *StrategicElector[T]) indexKey(group string) string {
	return fmt.Sprintf("%s:%s:index", e.opts.KeyPrefix, group)
}

func (e *StrategicElector[T]) candidateKey(group string, nodeID string) string {
	return e.candidateKeyPrefix(group) + nodeID
}

func (e *StrategicElector[T]) candidateKeyPrefix(group string) string {
	return fmt.Sprintf("%s:%s:candidates:", e.opts.KeyPrefix, group)
}

func validateGroup(group string) error {
	return core.RequireNotBlank("group", group)
}

func normalizeTime(value time.Time) time.Time {
	if value.IsZero() {
		return value
	}
	return value.UTC()
}

func durationMillis(value time.Duration) int64 {
	return int64((value + time.Millisecond - 1) / time.Millisecond)
}

var _ leader.StrategicElector[string] = (*StrategicElector[string])(nil)
