package ratelimittest

import (
	"context"
	"errors"
	"math"
	"sync"
	"testing"
	"time"
)

type bucket struct {
	tokens    float64
	updatedAt time.Time
}

type memoryControl struct {
	mu       sync.Mutex
	config   Config
	buckets  map[string]bucket
	gates    map[string]*memoryGate
	failures map[string]error
	counts   map[string]int64
}

type memoryGate struct {
	phase       Phase
	started     chan struct{}
	resume      chan struct{}
	startedOnce sync.Once
	resumeOnce  sync.Once
}

// MemoryHarness MemoryHarness 공개 API의 동작을 수행하며 rate-limit conformance harness의 quota/result ownership 계약을 보존한다.
func MemoryHarness() Harness {
	control := &memoryControl{
		buckets:  make(map[string]bucket),
		gates:    make(map[string]*memoryGate),
		failures: make(map[string]error),
		counts:   make(map[string]int64),
	}
	return Harness{
		New: func(t testing.TB, config Config) (AllowFunc, error) {
			t.Helper()
			if err := validateConfig(config); err != nil {
				return nil, err
			}
			control.mu.Lock()
			control.config = config
			control.mu.Unlock()
			return control.allow, nil
		},
		Control: control,
		IsProviderError: func(err error) bool {
			var target *fixtureError
			return errors.As(err, &target)
		},
	}
}

func (c *memoryControl) allow(ctx context.Context, key string, tokens int64) (Result, error) {
	if err := validateContext(ctx); err != nil {
		return Result{}, err
	}
	if validateKey(key) != nil || tokens <= 0 {
		return Result{}, errInvalidInput
	}
	c.mu.Lock()
	config := c.config
	c.mu.Unlock()
	if tokens > config.Burst {
		return Result{}, errInvalidInput
	}
	if err := c.passGate(ctx, key, PhaseBeforeLinearize); err != nil {
		return Result{}, err
	}

	c.mu.Lock()
	now := time.Now()
	state, ok := c.buckets[key]
	if !ok {
		state = bucket{tokens: float64(config.Burst), updatedAt: now}
	} else {
		elapsed := now.Sub(state.updatedAt).Seconds()
		state.tokens = math.Min(float64(config.Burst), state.tokens+elapsed*config.RatePerSecond)
		state.updatedAt = now
	}
	result := Result{Requested: tokens, Remaining: int64(math.Floor(state.tokens))}
	if state.tokens >= float64(tokens) {
		state.tokens -= float64(tokens)
		result.Allowed = true
		result.Remaining = int64(math.Floor(state.tokens))
		result.ResetAfter = durationFor(float64(config.Burst)-state.tokens, config.RatePerSecond)
	} else {
		result.RetryAfter = durationFor(float64(tokens)-state.tokens, config.RatePerSecond)
		result.ResetAfter = durationFor(float64(config.Burst)-state.tokens, config.RatePerSecond)
	}
	c.buckets[key] = state
	c.counts[key]++
	failure := c.failures[key]
	delete(c.failures, key)
	c.mu.Unlock()

	_ = c.passGate(context.Background(), key, PhaseAfterLinearize)
	if failure != nil {
		return Result{}, &fixtureError{cause: failure}
	}
	return result, nil
}

func (c *memoryControl) GateNext(ctx context.Context, key string, phase Phase) (Gate, error) {
	if err := validateContext(ctx); err != nil {
		return nil, err
	}
	if validateKey(key) != nil || (phase != PhaseBeforeLinearize && phase != PhaseAfterLinearize) {
		return nil, errInvalidInput
	}
	gate := &memoryGate{phase: phase, started: make(chan struct{}), resume: make(chan struct{})}
	c.mu.Lock()
	c.gates[key] = gate
	c.mu.Unlock()
	return gate, nil
}

func (c *memoryControl) FailNext(ctx context.Context, key string, cause error) error {
	if err := validateContext(ctx); err != nil {
		return err
	}
	if validateKey(key) != nil || cause == nil {
		return errInvalidInput
	}
	c.mu.Lock()
	c.failures[key] = cause
	c.mu.Unlock()
	return nil
}

func (c *memoryControl) OperationCount(key string) int64 {
	if validateKey(key) != nil {
		return 0
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.counts[key]
}

func (c *memoryControl) passGate(ctx context.Context, key string, phase Phase) error {
	c.mu.Lock()
	gate := c.gates[key]
	if gate != nil && gate.phase == phase {
		delete(c.gates, key)
	}
	c.mu.Unlock()
	if gate == nil || gate.phase != phase {
		return nil
	}
	gate.startedOnce.Do(func() { close(gate.started) })
	select {
	case <-gate.resume:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (g *memoryGate) AwaitStarted(ctx context.Context) error {
	if err := validateContext(ctx); err != nil {
		return err
	}
	select {
	case <-g.started:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (g *memoryGate) Resume() {
	if g != nil {
		g.resumeOnce.Do(func() { close(g.resume) })
	}
}

func durationFor(tokens, rate float64) time.Duration {
	if tokens <= 0 {
		return 0
	}
	return time.Duration(math.Ceil(tokens / rate * float64(time.Second)))
}
