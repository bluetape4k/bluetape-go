package locktest

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

type lockRecord struct {
	owner     string
	expiresAt time.Time
}

type gateKey struct {
	key       string
	owner     string
	operation Operation
}

type failureKey struct {
	key       string
	owner     string
	operation Operation
}

type memoryControl struct {
	mu       sync.Mutex
	records  map[string]lockRecord
	gates    map[gateKey]*memoryGate
	failures map[failureKey]error
	counts   map[failureKey]int64
}

type memoryGate struct {
	phase       Phase
	started     chan struct{}
	resume      chan struct{}
	startedOnce sync.Once
	resumeOnce  sync.Once
}

// MemoryHarness lock conformance harness의 acquire/release ownership 동작을 수행한다.
func MemoryHarness() Harness {
	control := &memoryControl{
		records:  make(map[string]lockRecord),
		gates:    make(map[gateKey]*memoryGate),
		failures: make(map[failureKey]error),
		counts:   make(map[failureKey]int64),
	}
	return Harness{
		New: func(t testing.TB, config Config) (AcquireFunc, error) {
			t.Helper()
			if err := validateConfig(config); err != nil {
				return nil, err
			}
			return control.acquireFunc(config), nil
		},
		Control: control,
		IsProviderError: func(err error) bool {
			var target *fixtureError
			return errors.As(err, &target)
		},
	}
}

func (c *memoryControl) acquireFunc(config Config) AcquireFunc {
	return func(ctx context.Context) (ReleaseFunc, error) {
		if err := validateContext(ctx); err != nil {
			return nil, err
		}
		if err := c.passGate(ctx, config, OperationAcquire, PhaseBeforeLinearize); err != nil {
			return nil, err
		}

		c.mu.Lock()
		now := time.Now()
		record := c.records[config.Key]
		if record.owner != "" && record.expiresAt.After(now) {
			c.mu.Unlock()
			return nil, &fixtureError{operation: OperationAcquire}
		}
		c.records[config.Key] = lockRecord{owner: config.Owner, expiresAt: now.Add(config.TTL)}
		countKey := failureKey{key: config.Key, owner: config.Owner, operation: OperationAcquire}
		c.counts[countKey]++
		failure := c.failures[countKey]
		delete(c.failures, countKey)
		c.mu.Unlock()

		release := c.releaseFunc(config)
		_ = c.passGate(context.Background(), config, OperationAcquire, PhaseAfterLinearize)
		if failure != nil {
			return release, &fixtureError{operation: OperationAcquire, cause: failure}
		}
		return release, nil
	}
}

func (c *memoryControl) releaseFunc(config Config) ReleaseFunc {
	return func(ctx context.Context) (bool, error) {
		if err := validateContext(ctx); err != nil {
			return false, err
		}
		if err := c.passGate(ctx, config, OperationRelease, PhaseBeforeLinearize); err != nil {
			return false, err
		}

		c.mu.Lock()
		record := c.records[config.Key]
		deleted := record.owner == config.Owner && record.expiresAt.After(time.Now())
		if deleted {
			delete(c.records, config.Key)
		}
		countKey := failureKey{key: config.Key, owner: config.Owner, operation: OperationRelease}
		c.counts[countKey]++
		failure := c.failures[countKey]
		delete(c.failures, countKey)
		c.mu.Unlock()

		_ = c.passGate(context.Background(), config, OperationRelease, PhaseAfterLinearize)
		if failure != nil {
			return false, &fixtureError{operation: OperationRelease, cause: failure}
		}
		return deleted, nil
	}
}

func (c *memoryControl) GateNext(ctx context.Context, config Config, operation Operation, phase Phase) (Gate, error) {
	if err := validateContext(ctx); err != nil {
		return nil, err
	}
	if validateConfig(config) != nil || !validOperation(operation) || !validPhase(phase) {
		return nil, errInvalidInput
	}
	gate := &memoryGate{phase: phase, started: make(chan struct{}), resume: make(chan struct{})}
	c.mu.Lock()
	c.gates[gateKey{key: config.Key, owner: config.Owner, operation: operation}] = gate
	c.mu.Unlock()
	return gate, nil
}

func (c *memoryControl) FailNext(ctx context.Context, config Config, operation Operation, cause error) error {
	if err := validateContext(ctx); err != nil {
		return err
	}
	if validateConfig(config) != nil || !validOperation(operation) || cause == nil {
		return errInvalidInput
	}
	c.mu.Lock()
	c.failures[failureKey{key: config.Key, owner: config.Owner, operation: operation}] = cause
	c.mu.Unlock()
	return nil
}

func (c *memoryControl) Owner(ctx context.Context, config Config) (string, error) {
	if err := validateContext(ctx); err != nil {
		return "", err
	}
	if validateConfig(config) != nil {
		return "", errInvalidInput
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	record := c.records[config.Key]
	if record.owner != "" && !record.expiresAt.After(time.Now()) {
		delete(c.records, config.Key)
		return "", nil
	}
	return record.owner, nil
}

func (c *memoryControl) OperationCount(config Config, operation Operation) int64 {
	if validateConfig(config) != nil || !validOperation(operation) {
		return 0
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.counts[failureKey{key: config.Key, owner: config.Owner, operation: operation}]
}

func (c *memoryControl) passGate(ctx context.Context, config Config, operation Operation, phase Phase) error {
	key := gateKey{key: config.Key, owner: config.Owner, operation: operation}
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
	if g == nil {
		return
	}
	g.resumeOnce.Do(func() { close(g.resume) })
}
