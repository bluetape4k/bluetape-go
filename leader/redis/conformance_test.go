package redisleader_test

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/bluetape4k/bluetape-go/leader"
	"github.com/bluetape4k/bluetape-go/leader/leadertest"
	redisleader "github.com/bluetape4k/bluetape-go/leader/redis"
	"github.com/redis/go-redis/v9"
)

func TestRedisElectorConformance(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	controlClient := newRedisClient(ctx, t)
	control := &redisConformanceControl{
		client:        controlClient,
		failures:      make(map[string]map[leadertest.Operation]error),
		probeFailures: make(map[string]error),
		counts:        make(map[string]map[leadertest.Operation]int64),
	}
	harness := leadertest.Harness{
		New: func(tb testing.TB, opts leader.Options) (leader.Elector, error) {
			client := redis.NewClient(&redis.Options{Addr: controlClient.Options().Addr})
			client.AddHook(&redisConformanceHook{control: control, opts: opts})
			tb.Cleanup(func() { _ = client.Close() })
			return redisleader.New(client, opts)
		},
		Control: control,
	}
	leadertest.Run(t, harness)
}

type redisConformanceControl struct {
	client *redis.Client
	mu     sync.Mutex

	failures      map[string]map[leadertest.Operation]error
	probeFailures map[string]error
	counts        map[string]map[leadertest.Operation]int64
}

func (c *redisConformanceControl) ReplaceOwner(ctx context.Context, opts leader.Options, owner string) error {
	if ctx == nil {
		return leader.ErrInvalidContext
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	normalized, err := opts.Normalize()
	if err != nil || strings.TrimSpace(owner) == "" {
		return errors.New("redis leader conformance: invalid control input")
	}
	return c.client.Set(ctx, redisLeaderKey(normalized), owner, normalized.Lease).Err()
}

func (c *redisConformanceControl) FailNext(ctx context.Context, opts leader.Options, operation leadertest.Operation, cause error) error {
	if ctx == nil {
		return leader.ErrInvalidContext
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	normalized, err := opts.Normalize()
	if err != nil || cause == nil || !validLeaderOperation(operation) {
		return errors.New("redis leader conformance: invalid failure injection")
	}
	key := redisLeaderKey(normalized)
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.failures[key] == nil {
		c.failures[key] = make(map[leadertest.Operation]error)
	}
	c.failures[key][operation] = cause
	return nil
}

func (c *redisConformanceControl) Owner(ctx context.Context, opts leader.Options) (string, error) {
	if ctx == nil {
		return "", leader.ErrInvalidContext
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	normalized, err := opts.Normalize()
	if err != nil {
		return "", errors.New("redis leader conformance: invalid options")
	}
	value, err := c.client.Get(ctx, redisLeaderKey(normalized)).Result()
	if errors.Is(err, redis.Nil) {
		return "", nil
	}
	return value, err
}

func (c *redisConformanceControl) OperationCount(opts leader.Options, operation leadertest.Operation) int64 {
	normalized, err := opts.Normalize()
	if err != nil || !validLeaderOperation(operation) {
		return 0
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.counts[redisLeaderKey(normalized)][operation]
}

func (c *redisConformanceControl) after(key string, operation leadertest.Operation) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.counts[key] == nil {
		c.counts[key] = make(map[leadertest.Operation]int64)
	}
	c.counts[key][operation]++
	failure := c.failures[key][operation]
	delete(c.failures[key], operation)
	if failure != nil && operation == leadertest.OperationCampaign {
		c.probeFailures[key] = failure
	}
	return failure
}

func (c *redisConformanceControl) takeProbeFailure(key string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	err := c.probeFailures[key]
	delete(c.probeFailures, key)
	return err
}

type redisConformanceHook struct {
	control *redisConformanceControl
	opts    leader.Options
}

func (h *redisConformanceHook) DialHook(next redis.DialHook) redis.DialHook { return next }

func (h *redisConformanceHook) ProcessHook(next redis.ProcessHook) redis.ProcessHook {
	return func(ctx context.Context, cmd redis.Cmder) error {
		key := redisLeaderKey(h.opts)
		if cmd.Name() == "get" {
			if err := h.control.takeProbeFailure(key); err != nil {
				return err
			}
		}
		err := next(ctx, cmd)
		if err != nil {
			return err
		}
		operation, ok := redisLeaderOperation(cmd)
		if !ok {
			return nil
		}
		return h.control.after(key, operation)
	}
}

func (h *redisConformanceHook) ProcessPipelineHook(next redis.ProcessPipelineHook) redis.ProcessPipelineHook {
	return next
}

func redisLeaderOperation(cmd redis.Cmder) (leadertest.Operation, bool) {
	switch cmd.Name() {
	case "set", "setnx":
		return leadertest.OperationCampaign, true
	case "eval":
		args := cmd.Args()
		if len(args) > 1 {
			script, _ := args[1].(string)
			switch {
			case strings.Contains(script, "PEXPIRE"):
				return leadertest.OperationRenew, true
			case strings.Contains(script, "DEL"):
				return leadertest.OperationResign, true
			}
		}
	}
	return "", false
}

func redisLeaderKey(opts leader.Options) string {
	normalized, err := opts.Normalize()
	if err != nil {
		return ""
	}
	return normalized.KeyPrefix + ":" + normalized.Group
}

func validLeaderOperation(operation leadertest.Operation) bool {
	switch operation {
	case leadertest.OperationCampaign, leadertest.OperationRenew, leadertest.OperationResign:
		return true
	default:
		return false
	}
}
