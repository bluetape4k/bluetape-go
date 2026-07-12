package redisratelimit

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/bluetape4k/bluetape-go/ratelimit/ratelimittest"
	btredis "github.com/bluetape4k/bluetape-go/redis"
	"github.com/redis/go-redis/v9"
)

func TestRedisRateLimiterConformance(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	controlClient := redisClient(ctx, t)
	control := newRedisRateControl()
	harness := ratelimittest.Harness{
		New: func(tb testing.TB, config ratelimittest.Config) (ratelimittest.AllowFunc, error) {
			client := redis.NewClient(&redis.Options{Addr: controlClient.Options().Addr})
			client.AddHook(&redisRateHook{control: control})
			tb.Cleanup(func() { _ = client.Close() })
			limiter, err := New(Options{
				Client: client, Namespace: "conformance", RatePerSecond: config.RatePerSecond,
				Burst: config.Burst, IdleTTL: config.IdleTTL,
			})
			if err != nil {
				return nil, err
			}
			return func(ctx context.Context, key string, tokens int64) (ratelimittest.Result, error) {
				result, err := limiter.Allow(ctx, key, tokens)
				return ratelimittest.Result{
					Allowed: result.Allowed, Requested: result.Requested, Remaining: result.Remaining,
					RetryAfter: result.RetryAfter, ResetAfter: result.ResetAfter,
				}, err
			}, nil
		},
		Control: control,
		IsProviderError: func(err error) bool {
			var target *btredis.OpError
			return errors.As(err, &target)
		},
	}
	ratelimittest.Run(t, harness)

	t.Run("commit-unknown-sentinel-and-redaction", func(t *testing.T) {
		config := ratelimittest.Config{RatePerSecond: 1, Burst: 2, IdleTTL: 5 * time.Second}
		allow, err := harness.New(t, config)
		if err != nil {
			t.Fatal(err)
		}
		key := "raw-key-marker"
		cause := errors.New("raw-endpoint-injected-cause-marker")
		if err := control.FailNext(context.Background(), key, cause); err != nil {
			t.Fatal(err)
		}
		result, err := allow(context.Background(), key, 1)
		if result != (ratelimittest.Result{}) || !errors.Is(err, btredis.ErrCommitUnknown) {
			t.Fatalf("Allow lost response = %+v, %v", result, err)
		}
		var operationErr *btredis.OpError
		if !errors.As(err, &operationErr) {
			t.Fatalf("Allow error type = %T", err)
		}
		for _, marker := range []string{key, "raw-endpoint", "injected-cause"} {
			if strings.Contains(err.Error(), marker) {
				t.Fatalf("Allow error leaked marker %q: %v", marker, err)
			}
		}
		if count := control.OperationCount(key); count != 1 {
			t.Fatalf("operation count = %d, want 1", count)
		}
	})
}

type redisRateControl struct {
	mu       sync.Mutex
	gates    map[string]*redisRateGate
	failures map[string]error
	counts   map[string]int64
}

func newRedisRateControl() *redisRateControl {
	return &redisRateControl{gates: make(map[string]*redisRateGate), failures: make(map[string]error), counts: make(map[string]int64)}
}

func (c *redisRateControl) GateNext(ctx context.Context, key string, phase ratelimittest.Phase) (ratelimittest.Gate, error) {
	if err := validateRedisRateControl(ctx, key); err != nil {
		return nil, err
	}
	if phase != ratelimittest.PhaseBeforeLinearize && phase != ratelimittest.PhaseAfterLinearize {
		return nil, errors.New("redis rate conformance: invalid phase")
	}
	gate := &redisRateGate{phase: phase, started: make(chan struct{}), resume: make(chan struct{})}
	c.mu.Lock()
	c.gates[key] = gate
	c.mu.Unlock()
	return gate, nil
}

func (c *redisRateControl) FailNext(ctx context.Context, key string, cause error) error {
	if err := validateRedisRateControl(ctx, key); err != nil {
		return err
	}
	if cause == nil {
		return errors.New("redis rate conformance: nil failure")
	}
	c.mu.Lock()
	c.failures[key] = cause
	c.mu.Unlock()
	return nil
}

func (c *redisRateControl) OperationCount(key string) int64 {
	if strings.TrimSpace(key) == "" {
		return 0
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.counts[key]
}

func (c *redisRateControl) before(ctx context.Context, key string) error {
	c.mu.Lock()
	gate := c.gates[key]
	if gate != nil && gate.phase == ratelimittest.PhaseBeforeLinearize {
		delete(c.gates, key)
	}
	c.mu.Unlock()
	if gate == nil || gate.phase != ratelimittest.PhaseBeforeLinearize {
		return nil
	}
	if err := gate.wait(ctx); err != nil {
		return &notDispatchedError{cause: err}
	}
	return nil
}

func (c *redisRateControl) after(key string) error {
	c.mu.Lock()
	c.counts[key]++
	gate := c.gates[key]
	if gate != nil && gate.phase == ratelimittest.PhaseAfterLinearize {
		delete(c.gates, key)
	}
	failure := c.failures[key]
	delete(c.failures, key)
	c.mu.Unlock()
	if gate != nil && gate.phase == ratelimittest.PhaseAfterLinearize {
		if err := gate.wait(context.Background()); err != nil {
			return err
		}
	}
	return failure
}

type redisRateGate struct {
	phase       ratelimittest.Phase
	started     chan struct{}
	resume      chan struct{}
	startedOnce sync.Once
	resumeOnce  sync.Once
}

func (g *redisRateGate) AwaitStarted(ctx context.Context) error {
	if ctx == nil {
		return errors.New("redis rate conformance: nil context")
	}
	select {
	case <-g.started:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (g *redisRateGate) Resume() { g.resumeOnce.Do(func() { close(g.resume) }) }

func (g *redisRateGate) wait(ctx context.Context) error {
	g.startedOnce.Do(func() { close(g.started) })
	select {
	case <-g.resume:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

type redisRateHook struct{ control *redisRateControl }

func (h *redisRateHook) DialHook(next redis.DialHook) redis.DialHook { return next }

func (h *redisRateHook) ProcessHook(next redis.ProcessHook) redis.ProcessHook {
	return func(ctx context.Context, cmd redis.Cmder) error {
		key, ok := redisRateKey(cmd)
		if !ok {
			return next(ctx, cmd)
		}
		if err := h.control.before(ctx, key); err != nil {
			return err
		}
		if err := next(ctx, cmd); err != nil {
			return err
		}
		return h.control.after(key)
	}
}

func (h *redisRateHook) ProcessPipelineHook(next redis.ProcessPipelineHook) redis.ProcessPipelineHook {
	return next
}

func redisRateKey(cmd redis.Cmder) (string, bool) {
	if cmd.Name() != "eval" && cmd.Name() != "evalsha" {
		return "", false
	}
	for _, arg := range cmd.Args() {
		value, ok := arg.(string)
		if !ok {
			continue
		}
		const marker = ":bucket:"
		if index := strings.Index(value, marker); index >= 0 {
			return value[index+len(marker):], true
		}
	}
	return "", false
}

func validateRedisRateControl(ctx context.Context, key string) error {
	if ctx == nil || strings.TrimSpace(key) == "" {
		return errors.New("redis rate conformance: invalid control input")
	}
	return ctx.Err()
}
