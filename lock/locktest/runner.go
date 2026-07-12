package locktest

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

var runnerID atomic.Uint64

// Run executes all mandatory lock conformance cases.
func Run(t *testing.T, harness Harness) {
	t.Helper()
	if err := validateHarness(harness); err != nil {
		t.Fatal(err)
	}
	if err := validatePositiveClassifier(t, harness); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name string
		run  func(*testing.T, Harness, Config) error
	}{
		{"acquire-release", runAcquireRelease},
		{"contention", runContention},
		{"repeated-release", runRepeatedRelease},
		{"expiry-takeover", runExpiryTakeover},
		{"pre-canceled-acquire", runPreCanceledAcquire},
		{"pre-canceled-release", runPreCanceledRelease},
		{"cancel-before-linearize", runCancelBefore},
		{"cancel-after-linearize", runCancelAfter},
		{"lost-response", runLostResponse},
		{"stale-release", runStaleRelease},
		{"exact-contention", runExactContention},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			id := runnerID.Add(1)
			config := Config{Key: fmt.Sprintf("locktest-%s-%d", tc.name, id), Owner: fmt.Sprintf("owner-%d", id), TTL: 100 * time.Millisecond}
			if err := tc.run(t, harness, config); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func makeAcquire(t *testing.T, h Harness, config Config) (AcquireFunc, error) {
	t.Helper()
	acquire, err := h.New(t, config)
	if err != nil {
		return nil, err
	}
	if acquire == nil {
		return nil, errors.New("locktest: factory returned nil acquire function")
	}
	return acquire, nil
}

func runAcquireRelease(t *testing.T, h Harness, config Config) error {
	acquire, err := makeAcquire(t, h, config)
	if err != nil {
		return err
	}
	release, err := acquire(context.Background())
	if err != nil || release == nil {
		return fmt.Errorf("acquire = %v, %w", release, err)
	}
	deleted, err := release(context.Background())
	if err != nil || !deleted {
		return fmt.Errorf("release = %v, %w", deleted, err)
	}
	return nil
}

func runContention(t *testing.T, h Harness, config Config) error {
	first, err := makeAcquire(t, h, config)
	if err != nil {
		return err
	}
	secondConfig := config
	secondConfig.Owner += "-other"
	second, err := makeAcquire(t, h, secondConfig)
	if err != nil {
		return err
	}
	release, err := first(context.Background())
	if err != nil {
		return err
	}
	defer release(context.Background())
	otherRelease, err := second(context.Background())
	if otherRelease != nil || err == nil {
		return fmt.Errorf("contention tuple = %v, %v", otherRelease, err)
	}
	return nil
}

func runRepeatedRelease(t *testing.T, h Harness, config Config) error {
	acquire, err := makeAcquire(t, h, config)
	if err != nil {
		return err
	}
	release, err := acquire(context.Background())
	if err != nil {
		return err
	}
	if deleted, err := release(context.Background()); err != nil || !deleted {
		return fmt.Errorf("first release = %v, %v", deleted, err)
	}
	if deleted, err := release(context.Background()); err != nil || deleted {
		return fmt.Errorf("second release = %v, %v", deleted, err)
	}
	return nil
}

func runExpiryTakeover(t *testing.T, h Harness, config Config) error {
	first, _ := makeAcquire(t, h, config)
	release, err := first(context.Background())
	if err != nil {
		return err
	}
	defer release(context.Background())
	time.Sleep(config.TTL + 10*time.Millisecond)
	other := config
	other.Owner += "-other"
	second, _ := makeAcquire(t, h, other)
	secondRelease, err := second(context.Background())
	if err != nil || secondRelease == nil {
		return fmt.Errorf("expiry takeover = %v, %v", secondRelease, err)
	}
	_, err = secondRelease(context.Background())
	return err
}

func runPreCanceledAcquire(t *testing.T, h Harness, config Config) error {
	acquire, _ := makeAcquire(t, h, config)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	release, err := acquire(ctx)
	if release != nil || !errors.Is(err, context.Canceled) {
		return fmt.Errorf("pre-canceled acquire = %v, %v", release, err)
	}
	return nil
}

func runPreCanceledRelease(t *testing.T, h Harness, config Config) error {
	acquire, _ := makeAcquire(t, h, config)
	release, err := acquire(context.Background())
	if err != nil {
		return err
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	deleted, err := release(ctx)
	if deleted || !errors.Is(err, context.Canceled) {
		return fmt.Errorf("pre-canceled release = %v, %v", deleted, err)
	}
	_, cleanupErr := release(context.Background())
	return cleanupErr
}

func runCancelBefore(t *testing.T, h Harness, config Config) error {
	acquire, _ := makeAcquire(t, h, config)
	gate, err := h.Control.GateNext(context.Background(), config, OperationAcquire, PhaseBeforeLinearize)
	if err != nil || gate == nil {
		return fmt.Errorf("gate = %v, %v", gate, err)
	}
	defer gate.Resume()
	ctx, cancel := context.WithCancel(context.Background())
	result := make(chan error, 1)
	go func() { _, err := acquire(ctx); result <- err }()
	if err := gate.AwaitStarted(context.Background()); err != nil {
		return err
	}
	cancel()
	if err := <-result; !errors.Is(err, context.Canceled) {
		return fmt.Errorf("before-linearize error = %v", err)
	}
	owner, err := h.Control.Owner(context.Background(), config)
	if err != nil || owner != "" {
		return fmt.Errorf("before-linearize owner = %q, %v", owner, err)
	}
	return nil
}

func runCancelAfter(t *testing.T, h Harness, config Config) error {
	acquire, _ := makeAcquire(t, h, config)
	gate, err := h.Control.GateNext(context.Background(), config, OperationAcquire, PhaseAfterLinearize)
	if err != nil {
		return err
	}
	defer gate.Resume()
	ctx, cancel := context.WithCancel(context.Background())
	type result struct {
		release ReleaseFunc
		err     error
	}
	resultCh := make(chan result, 1)
	go func() { release, err := acquire(ctx); resultCh <- result{release, err} }()
	if err := gate.AwaitStarted(context.Background()); err != nil {
		return err
	}
	cancel()
	gate.Resume()
	got := <-resultCh
	if got.err != nil || got.release == nil {
		return fmt.Errorf("after-linearize acquire = %v, %v", got.release, got.err)
	}
	_, err = got.release(context.Background())
	return err
}

func runLostResponse(t *testing.T, h Harness, config Config) error {
	acquire, _ := makeAcquire(t, h, config)
	if err := h.Control.FailNext(context.Background(), config, OperationAcquire, errors.New("injected-cause")); err != nil {
		return err
	}
	release, err := acquire(context.Background())
	if release == nil || err == nil || !h.IsProviderError(err) || !h.IsProviderError(fmt.Errorf("nested: %w", err)) {
		return fmt.Errorf("lost acquire tuple = %v, %v", release, err)
	}
	if err := h.Control.FailNext(context.Background(), config, OperationRelease, errors.New("injected-cause")); err != nil {
		return err
	}
	deleted, err := release(context.Background())
	if deleted || err == nil || !h.IsProviderError(err) {
		return fmt.Errorf("lost release tuple = %v, %v", deleted, err)
	}
	if deleted, err := release(context.Background()); err != nil || deleted {
		return fmt.Errorf("release retry = %v, %v", deleted, err)
	}
	return nil
}

func runStaleRelease(t *testing.T, h Harness, config Config) error {
	first, _ := makeAcquire(t, h, config)
	staleRelease, err := first(context.Background())
	if err != nil {
		return err
	}
	time.Sleep(config.TTL + 10*time.Millisecond)
	other := config
	other.Owner += "-other"
	second, _ := makeAcquire(t, h, other)
	activeRelease, err := second(context.Background())
	if err != nil {
		return err
	}
	if deleted, err := staleRelease(context.Background()); err != nil || deleted {
		return fmt.Errorf("stale release = %v, %v", deleted, err)
	}
	owner, err := h.Control.Owner(context.Background(), other)
	if err != nil || owner != other.Owner {
		return fmt.Errorf("replacement owner = %q, %v", owner, err)
	}
	_, err = activeRelease(context.Background())
	return err
}

func runExactContention(t *testing.T, h Harness, config Config) error {
	const workers = 8
	var wg sync.WaitGroup
	var successes atomic.Int64
	var providerErrors atomic.Int64
	var winnerMu sync.Mutex
	var winner ReleaseFunc
	for i := range workers {
		candidate := config
		candidate.Owner = fmt.Sprintf("%s-%d", config.Owner, i)
		acquire, err := makeAcquire(t, h, candidate)
		if err != nil {
			return err
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			release, err := acquire(context.Background())
			if err == nil {
				successes.Add(1)
				winnerMu.Lock()
				winner = release
				winnerMu.Unlock()
			} else if release == nil {
				providerErrors.Add(1)
			}
		}()
	}
	wg.Wait()
	if successes.Load() != 1 || providerErrors.Load() != workers-1 || winner == nil {
		return fmt.Errorf("contention totals success=%d provider=%d", successes.Load(), providerErrors.Load())
	}
	if _, err := winner(context.Background()); err != nil {
		return err
	}
	takeover, _ := makeAcquire(t, h, config)
	release, err := takeover(context.Background())
	if err != nil {
		return err
	}
	_, err = release(context.Background())
	return err
}
