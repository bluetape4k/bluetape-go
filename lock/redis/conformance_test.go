package redislock_test

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/bluetape4k/bluetape-go/lock/locktest"
	redislock "github.com/bluetape4k/bluetape-go/lock/redis"
	btredis "github.com/bluetape4k/bluetape-go/redis"
	"github.com/redis/go-redis/v9"
)

func TestRedisLockConformance(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	controlClient := redisClient(ctx, t)
	control := newRedisLockControl(controlClient)
	harness := locktest.Harness{
		New: func(tb testing.TB, config locktest.Config) (locktest.AcquireFunc, error) {
			client := redis.NewClient(&redis.Options{Addr: controlClient.Options().Addr})
			client.AddHook(&redisLockHook{control: control, config: config})
			tb.Cleanup(func() { _ = client.Close() })
			mutex, err := redislock.New(client, redislock.Options{Key: config.Key, TTL: config.TTL, Token: config.Owner})
			if err != nil {
				return nil, err
			}
			return func(ctx context.Context) (locktest.ReleaseFunc, error) {
				lease, err := mutex.TryLock(ctx)
				if lease == nil {
					return nil, err
				}
				return func(ctx context.Context) (bool, error) { return lease.Unlock(ctx) }, err
			}, nil
		},
		Control: control,
		IsProviderError: func(err error) bool {
			var target *btredis.OpError
			return errors.As(err, &target)
		},
	}
	locktest.Run(t, harness)

	t.Run("commit-unknown-sentinel-and-redaction", func(t *testing.T) {
		config := locktest.Config{
			Key:   "raw-key-marker",
			Owner: "raw-owner-marker",
			TTL:   time.Second,
		}
		acquire, err := harness.New(t, config)
		if err != nil {
			t.Fatal(err)
		}
		cause := errors.New("raw-endpoint-injected-cause-marker")
		if err := control.FailNext(context.Background(), config, locktest.OperationAcquire, cause); err != nil {
			t.Fatal(err)
		}
		release, err := acquire(context.Background())
		if release == nil || !errors.Is(err, btredis.ErrCommitUnknown) {
			t.Fatalf("acquire tuple = %v, %v", release, err)
		}
		var operationErr *btredis.OpError
		if !errors.As(err, &operationErr) {
			t.Fatalf("acquire error type = %T", err)
		}
		for _, marker := range []string{config.Key, config.Owner, "raw-endpoint", "injected-cause"} {
			if strings.Contains(err.Error(), marker) {
				t.Fatalf("acquire error leaked marker %q: %v", marker, err)
			}
		}
		if err := control.FailNext(context.Background(), config, locktest.OperationRelease, cause); err != nil {
			t.Fatal(err)
		}
		if deleted, err := release(context.Background()); deleted || !errors.Is(err, btredis.ErrCommitUnknown) {
			t.Fatalf("release lost response = %v, %v", deleted, err)
		}
		if deleted, err := release(context.Background()); deleted || err != nil {
			t.Fatalf("release retry = %v, %v", deleted, err)
		}
	})
}

type redisLockControl struct {
	client *redis.Client
	mu     sync.Mutex
	gates  map[redisLockControlKey]*redisLockGate
	fails  map[redisLockControlKey]error
	probes map[string]error
	counts map[redisLockControlKey]int64
}

type redisLockControlKey struct {
	key       string
	owner     string
	operation locktest.Operation
}

func newRedisLockControl(client *redis.Client) *redisLockControl {
	return &redisLockControl{
		client: client,
		gates:  make(map[redisLockControlKey]*redisLockGate),
		fails:  make(map[redisLockControlKey]error),
		probes: make(map[string]error),
		counts: make(map[redisLockControlKey]int64),
	}
}

func (c *redisLockControl) GateNext(ctx context.Context, config locktest.Config, operation locktest.Operation, phase locktest.Phase) (locktest.Gate, error) {
	if err := validateRedisLockControl(ctx, config, operation); err != nil || !validLockPhase(phase) {
		if err != nil {
			return nil, err
		}
		return nil, errors.New("redis lock conformance: invalid phase")
	}
	gate := &redisLockGate{phase: phase, started: make(chan struct{}), resume: make(chan struct{})}
	c.mu.Lock()
	c.gates[redisLockControlKey{config.Key, config.Owner, operation}] = gate
	c.mu.Unlock()
	return gate, nil
}

func (c *redisLockControl) FailNext(ctx context.Context, config locktest.Config, operation locktest.Operation, cause error) error {
	if err := validateRedisLockControl(ctx, config, operation); err != nil {
		return err
	}
	if cause == nil {
		return errors.New("redis lock conformance: nil failure")
	}
	c.mu.Lock()
	c.fails[redisLockControlKey{config.Key, config.Owner, operation}] = cause
	c.mu.Unlock()
	return nil
}

func (c *redisLockControl) Owner(ctx context.Context, config locktest.Config) (string, error) {
	if ctx == nil {
		return "", errors.New("redis lock conformance: nil context")
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if strings.TrimSpace(config.Key) == "" || strings.TrimSpace(config.Owner) == "" || config.TTL <= 0 {
		return "", errors.New("redis lock conformance: invalid config")
	}
	owner, err := c.client.Get(ctx, config.Key).Result()
	if errors.Is(err, redis.Nil) {
		return "", nil
	}
	return owner, err
}

func (c *redisLockControl) OperationCount(config locktest.Config, operation locktest.Operation) int64 {
	if strings.TrimSpace(config.Key) == "" || strings.TrimSpace(config.Owner) == "" || config.TTL <= 0 || !validLockOperation(operation) {
		return 0
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.counts[redisLockControlKey{config.Key, config.Owner, operation}]
}

func (c *redisLockControl) before(ctx context.Context, config locktest.Config, operation locktest.Operation) error {
	key := redisLockControlKey{config.Key, config.Owner, operation}
	c.mu.Lock()
	gate := c.gates[key]
	if gate != nil && gate.phase == locktest.PhaseBeforeLinearize {
		delete(c.gates, key)
	}
	c.mu.Unlock()
	if gate == nil || gate.phase != locktest.PhaseBeforeLinearize {
		return nil
	}
	return gate.wait(ctx)
}

func (c *redisLockControl) after(config locktest.Config, operation locktest.Operation) error {
	key := redisLockControlKey{config.Key, config.Owner, operation}
	c.mu.Lock()
	c.counts[key]++
	gate := c.gates[key]
	if gate != nil && gate.phase == locktest.PhaseAfterLinearize {
		delete(c.gates, key)
	}
	failure := c.fails[key]
	delete(c.fails, key)
	if failure != nil && operation == locktest.OperationAcquire {
		c.probes[config.Key] = failure
	}
	c.mu.Unlock()
	if gate != nil && gate.phase == locktest.PhaseAfterLinearize {
		if err := gate.wait(context.Background()); err != nil {
			return err
		}
	}
	return failure
}

func (c *redisLockControl) takeProbe(key string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	err := c.probes[key]
	delete(c.probes, key)
	return err
}

type redisLockGate struct {
	phase       locktest.Phase
	started     chan struct{}
	resume      chan struct{}
	startedOnce sync.Once
	resumeOnce  sync.Once
}

func (g *redisLockGate) AwaitStarted(ctx context.Context) error {
	if ctx == nil {
		return errors.New("redis lock conformance: nil context")
	}
	select {
	case <-g.started:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (g *redisLockGate) Resume() { g.resumeOnce.Do(func() { close(g.resume) }) }

func (g *redisLockGate) wait(ctx context.Context) error {
	g.startedOnce.Do(func() { close(g.started) })
	select {
	case <-g.resume:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

type redisLockHook struct {
	control *redisLockControl
	config  locktest.Config
}

func (h *redisLockHook) DialHook(next redis.DialHook) redis.DialHook { return next }

func (h *redisLockHook) ProcessHook(next redis.ProcessHook) redis.ProcessHook {
	return func(ctx context.Context, cmd redis.Cmder) error {
		if cmd.Name() == "get" {
			if err := h.control.takeProbe(h.config.Key); err != nil {
				return err
			}
		}
		operation, ok := redisLockOperation(cmd)
		if !ok {
			return next(ctx, cmd)
		}
		if err := h.control.before(ctx, h.config, operation); err != nil {
			return err
		}
		if err := next(ctx, cmd); err != nil {
			return err
		}
		return h.control.after(h.config, operation)
	}
}

func (h *redisLockHook) ProcessPipelineHook(next redis.ProcessPipelineHook) redis.ProcessPipelineHook {
	return next
}

func redisLockOperation(cmd redis.Cmder) (locktest.Operation, bool) {
	switch cmd.Name() {
	case "set", "setnx":
		return locktest.OperationAcquire, true
	case "eval", "evalsha":
		return locktest.OperationRelease, true
	default:
		return "", false
	}
}

func validateRedisLockControl(ctx context.Context, config locktest.Config, operation locktest.Operation) error {
	if ctx == nil {
		return errors.New("redis lock conformance: nil context")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if strings.TrimSpace(config.Key) == "" || strings.TrimSpace(config.Owner) == "" || config.TTL <= 0 || !validLockOperation(operation) {
		return errors.New("redis lock conformance: invalid control input")
	}
	return nil
}

func validLockOperation(operation locktest.Operation) bool {
	return operation == locktest.OperationAcquire || operation == locktest.OperationRelease
}

func validLockPhase(phase locktest.Phase) bool {
	return phase == locktest.PhaseBeforeLinearize || phase == locktest.PhaseAfterLinearize
}
