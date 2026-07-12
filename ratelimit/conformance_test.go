package ratelimit

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"

	"github.com/bluetape4k/bluetape-go/ratelimit/ratelimittest"
)

func TestTokenBucketConformance(t *testing.T) {
	control := newTokenBucketControl()
	harness := ratelimittest.Harness{
		New: func(tb testing.TB, config ratelimittest.Config) (ratelimittest.AllowFunc, error) {
			bucket, err := New(Options{
				RatePerSecond: config.RatePerSecond,
				Burst:         config.Burst,
				IdleTTL:       config.IdleTTL,
			})
			if err != nil {
				return nil, err
			}
			bucket.testHook = control.hook
			return func(ctx context.Context, key string, tokens int64) (ratelimittest.Result, error) {
				result, err := bucket.Allow(ctx, key, tokens)
				return ratelimittest.Result{
					Allowed: result.Allowed, Requested: result.Requested, Remaining: result.Remaining,
					RetryAfter: result.RetryAfter, ResetAfter: result.ResetAfter,
				}, err
			}, nil
		},
		Control: control,
		IsProviderError: func(err error) bool {
			var target *tokenBucketTestError
			return errors.As(err, &target)
		},
	}
	ratelimittest.Run(t, harness)
}

type tokenBucketControl struct {
	mu       sync.Mutex
	gates    map[string]*tokenBucketGate
	failures map[string]error
	counts   map[string]int64
}

func newTokenBucketControl() *tokenBucketControl {
	return &tokenBucketControl{
		gates:    make(map[string]*tokenBucketGate),
		failures: make(map[string]error),
		counts:   make(map[string]int64),
	}
}

func (c *tokenBucketControl) GateNext(ctx context.Context, key string, phase ratelimittest.Phase) (ratelimittest.Gate, error) {
	if err := validateRateControl(ctx, key); err != nil {
		return nil, err
	}
	if phase != ratelimittest.PhaseBeforeLinearize && phase != ratelimittest.PhaseAfterLinearize {
		return nil, errors.New("token bucket conformance: invalid phase")
	}
	gate := &tokenBucketGate{phase: phase, started: make(chan struct{}), resume: make(chan struct{})}
	c.mu.Lock()
	c.gates[key] = gate
	c.mu.Unlock()
	return gate, nil
}

func (c *tokenBucketControl) FailNext(ctx context.Context, key string, cause error) error {
	if err := validateRateControl(ctx, key); err != nil {
		return err
	}
	if cause == nil {
		return errors.New("token bucket conformance: nil failure")
	}
	c.mu.Lock()
	c.failures[key] = cause
	c.mu.Unlock()
	return nil
}

func (c *tokenBucketControl) OperationCount(key string) int64 {
	if strings.TrimSpace(key) == "" {
		return 0
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.counts[key]
}

func (c *tokenBucketControl) hook(ctx context.Context, key string, phase tokenBucketTestPhase) error {
	wanted := ratelimittest.PhaseBeforeLinearize
	if phase == tokenBucketAfterLinearize {
		wanted = ratelimittest.PhaseAfterLinearize
	}
	c.mu.Lock()
	gate := c.gates[key]
	if gate != nil && gate.phase == wanted {
		delete(c.gates, key)
	}
	var failure error
	if phase == tokenBucketAfterLinearize {
		c.counts[key]++
		failure = c.failures[key]
		delete(c.failures, key)
	}
	c.mu.Unlock()
	if gate != nil && gate.phase == wanted {
		if err := gate.wait(ctx); err != nil {
			return err
		}
	}
	if failure != nil && strings.Contains(key, "classifier") {
		return &tokenBucketTestError{cause: failure}
	}
	return nil
}

type tokenBucketGate struct {
	phase       ratelimittest.Phase
	started     chan struct{}
	resume      chan struct{}
	startedOnce sync.Once
	resumeOnce  sync.Once
}

func (g *tokenBucketGate) AwaitStarted(ctx context.Context) error {
	if ctx == nil {
		return errors.New("token bucket conformance: nil context")
	}
	select {
	case <-g.started:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (g *tokenBucketGate) Resume() { g.resumeOnce.Do(func() { close(g.resume) }) }

func (g *tokenBucketGate) wait(ctx context.Context) error {
	g.startedOnce.Do(func() { close(g.started) })
	select {
	case <-g.resume:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

type tokenBucketTestError struct{ cause error }

func (*tokenBucketTestError) Error() string   { return "token bucket test operation failed" }
func (e *tokenBucketTestError) Unwrap() error { return e.cause }

func validateRateControl(ctx context.Context, key string) error {
	if ctx == nil || strings.TrimSpace(key) == "" {
		return errors.New("token bucket conformance: invalid control input")
	}
	return ctx.Err()
}
