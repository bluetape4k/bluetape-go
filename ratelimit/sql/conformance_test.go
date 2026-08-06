package sqlratelimit

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/bluetape4k/bluetape-go/ratelimit"
	"github.com/bluetape4k/bluetape-go/ratelimit/ratelimittest"
	postgrestestcontainer "github.com/bluetape4k/bluetape-go/testcontainers/postgres"
	_ "github.com/jackc/pgx/v5/stdlib"
)

func TestPostgresRateLimiterConformance(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	t.Cleanup(cancel)
	dsn := postgrestestcontainer.Start(ctx, t)
	admin, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = admin.Close() })
	if _, err := admin.ExecContext(ctx, SchemaSQL); err != nil {
		t.Fatal(err)
	}
	control := newPostgresRateControl()
	harness := ratelimittest.Harness{
		New: func(tb testing.TB, config ratelimittest.Config) (ratelimittest.AllowFunc, error) {
			db, err := sql.Open("pgx", dsn)
			if err != nil {
				return nil, err
			}
			tb.Cleanup(func() { _ = db.Close() })
			limiter, err := New(db, Options{
				Namespace: "conformance", RatePerSecond: config.RatePerSecond,
				Burst: config.Burst, IdleTTL: config.IdleTTL,
			})
			if err != nil {
				return nil, err
			}
			limiter.testHook = control.hook
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
			var target ratelimit.OperationError
			return errors.As(err, &target) && target.Family() == "rate limiter"
		},
	}
	ratelimittest.Run(t, harness)
}

type postgresRateControl struct {
	mu       sync.Mutex
	gates    map[string]*postgresRateGate
	failures map[string]error
	counts   map[string]int64
}

func newPostgresRateControl() *postgresRateControl {
	return &postgresRateControl{
		gates: make(map[string]*postgresRateGate), failures: make(map[string]error), counts: make(map[string]int64),
	}
}

func (c *postgresRateControl) GateNext(ctx context.Context, key string, phase ratelimittest.Phase) (ratelimittest.Gate, error) {
	if err := validatePostgresRateControl(ctx, key); err != nil {
		return nil, err
	}
	if phase != ratelimittest.PhaseBeforeLinearize && phase != ratelimittest.PhaseAfterLinearize {
		return nil, errors.New("postgres rate conformance: invalid phase")
	}
	gate := &postgresRateGate{phase: phase, started: make(chan struct{}), resume: make(chan struct{})}
	c.mu.Lock()
	c.gates[key] = gate
	c.mu.Unlock()
	return gate, nil
}

func (c *postgresRateControl) FailNext(ctx context.Context, key string, cause error) error {
	if err := validatePostgresRateControl(ctx, key); err != nil {
		return err
	}
	if cause == nil {
		return errors.New("postgres rate conformance: nil failure")
	}
	c.mu.Lock()
	c.failures[key] = cause
	c.mu.Unlock()
	return nil
}

func (c *postgresRateControl) OperationCount(key string) int64 {
	if strings.TrimSpace(key) == "" {
		return 0
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.counts[key]
}

func (c *postgresRateControl) hook(ctx context.Context, operation string, phase testPhase, key string) error {
	if operation != "allow" {
		return nil
	}
	if phase == phaseBeforeLinearize {
		c.mu.Lock()
		gate := c.gates[key]
		if gate != nil && gate.phase == ratelimittest.PhaseBeforeLinearize {
			delete(c.gates, key)
		}
		c.mu.Unlock()
		if gate != nil && gate.phase == ratelimittest.PhaseBeforeLinearize {
			return gate.wait(ctx)
		}
		return nil
	}

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

type postgresRateGate struct {
	phase       ratelimittest.Phase
	started     chan struct{}
	resume      chan struct{}
	startedOnce sync.Once
	resumeOnce  sync.Once
}

func (g *postgresRateGate) AwaitStarted(ctx context.Context) error {
	if ctx == nil {
		return errors.New("postgres rate conformance: nil context")
	}
	select {
	case <-g.started:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (g *postgresRateGate) Resume() { g.resumeOnce.Do(func() { close(g.resume) }) }

func (g *postgresRateGate) wait(ctx context.Context) error {
	g.startedOnce.Do(func() { close(g.started) })
	select {
	case <-g.resume:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func validatePostgresRateControl(ctx context.Context, key string) error {
	if ctx == nil || strings.TrimSpace(key) == "" {
		return errors.New("postgres rate conformance: invalid control input")
	}
	return ctx.Err()
}
