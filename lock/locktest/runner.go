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

const conformanceWaitTimeout = 2 * time.Second

// Run executes all mandatory lock conformance cases.
func Run(t *testing.T, harness Harness) {
	t.Helper()
	if err := validateHarness(harness); err != nil {
		t.Fatal("locktest: invalid harness")
	}
	if err := validatePositiveClassifier(t, harness); err != nil {
		t.Fatal("locktest: provider classifier probe failed")
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
				t.Fatalf("locktest: conformance case failed: %s", conformanceFailureReason(harness, err))
			}
		})
	}
}

func conformanceFailureReason(h Harness, err error) (reason string) {
	reason = "contract"
	defer func() { _ = recover() }()
	switch {
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		return "context"
	case errors.Is(err, errInvalidInput):
		return "validation"
	case h.IsProviderError(err):
		return "provider"
	default:
		return reason
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
		return lockFailure(fmt.Sprintf("acquire release=%v", release), err)
	}
	deleted, err := release(context.Background())
	if err != nil || !deleted {
		return lockFailure(fmt.Sprintf("release deleted=%v", deleted), err)
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
	defer func() { _, _ = release(context.Background()) }()
	otherRelease, err := second(context.Background())
	if otherRelease != nil || err == nil {
		return lockFailure(fmt.Sprintf("contention tuple release=%v", otherRelease), err)
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
		return lockFailure(fmt.Sprintf("first release deleted=%v", deleted), err)
	}
	if deleted, err := release(context.Background()); err != nil || deleted {
		return lockFailure(fmt.Sprintf("second release deleted=%v", deleted), err)
	}
	return nil
}

func runExpiryTakeover(t *testing.T, h Harness, config Config) error {
	first, err := makeAcquire(t, h, config)
	if err != nil {
		return err
	}
	release, err := first(context.Background())
	if err != nil {
		return err
	}
	defer func() { _, _ = release(context.Background()) }()
	time.Sleep(config.TTL + 10*time.Millisecond)
	other := config
	other.Owner += "-other"
	second, err := makeAcquire(t, h, other)
	if err != nil {
		return err
	}
	secondRelease, err := second(context.Background())
	if err != nil || secondRelease == nil {
		return lockFailure(fmt.Sprintf("expiry takeover release=%v", secondRelease), err)
	}
	_, err = secondRelease(context.Background())
	return err
}

func runPreCanceledAcquire(t *testing.T, h Harness, config Config) error {
	acquire, err := makeAcquire(t, h, config)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	release, err := acquire(ctx)
	if release != nil || !errors.Is(err, context.Canceled) {
		return lockFailure(fmt.Sprintf("pre-canceled acquire release=%v", release), err)
	}
	return nil
}

func runPreCanceledRelease(t *testing.T, h Harness, config Config) error {
	acquire, err := makeAcquire(t, h, config)
	if err != nil {
		return err
	}
	release, err := acquire(context.Background())
	if err != nil {
		return err
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	deleted, err := release(ctx)
	if deleted || !errors.Is(err, context.Canceled) {
		return lockFailure(fmt.Sprintf("pre-canceled release deleted=%v", deleted), err)
	}
	_, cleanupErr := release(context.Background())
	return cleanupErr
}

func runCancelBefore(t *testing.T, h Harness, config Config) error {
	acquire, err := makeAcquire(t, h, config)
	if err != nil {
		return err
	}
	gate, err := h.Control.GateNext(context.Background(), config, OperationAcquire, PhaseBeforeLinearize)
	if err != nil || gate == nil {
		return lockFailure(fmt.Sprintf("gate=%v", gate), err)
	}
	defer gate.Resume()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	result := make(chan error, 1)
	go func() { _, err := acquire(ctx); result <- err }()
	waitCtx, waitCancel := context.WithTimeout(context.Background(), conformanceWaitTimeout)
	err = gate.AwaitStarted(waitCtx)
	waitCancel()
	if err != nil {
		cancel()
		gate.Resume()
		return errors.New("locktest: acquire gate did not start")
	}
	cancel()
	select {
	case err = <-result:
	case <-time.After(conformanceWaitTimeout):
		gate.Resume()
		return errors.New("locktest: canceled acquire did not return")
	}
	if !errors.Is(err, context.Canceled) {
		return lockFailure("before-linearize cancellation mismatch", err)
	}
	owner, err := h.Control.Owner(context.Background(), config)
	if err != nil || owner != "" {
		return lockFailure(fmt.Sprintf("before-linearize owner=%q", owner), err)
	}
	return nil
}

func runCancelAfter(t *testing.T, h Harness, config Config) error {
	acquire, err := makeAcquire(t, h, config)
	if err != nil {
		return err
	}
	gate, err := h.Control.GateNext(context.Background(), config, OperationAcquire, PhaseAfterLinearize)
	if err != nil {
		return err
	}
	defer gate.Resume()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	type result struct {
		release ReleaseFunc
		err     error
	}
	resultCh := make(chan result, 1)
	go func() { release, err := acquire(ctx); resultCh <- result{release, err} }()
	waitCtx, waitCancel := context.WithTimeout(context.Background(), conformanceWaitTimeout)
	err = gate.AwaitStarted(waitCtx)
	waitCancel()
	if err != nil {
		cancel()
		gate.Resume()
		return errors.New("locktest: acquire gate did not start")
	}
	cancel()
	gate.Resume()
	var got result
	select {
	case got = <-resultCh:
	case <-time.After(conformanceWaitTimeout):
		return errors.New("locktest: linearized acquire did not return")
	}
	if got.err != nil || got.release == nil {
		return lockFailure(fmt.Sprintf("after-linearize acquire release=%v", got.release), got.err)
	}
	_, err = got.release(context.Background())
	return err
}

func runLostResponse(t *testing.T, h Harness, config Config) error {
	acquire, err := makeAcquire(t, h, config)
	if err != nil {
		return err
	}
	if err := h.Control.FailNext(context.Background(), config, OperationAcquire, errors.New("injected-cause")); err != nil {
		return err
	}
	release, err := acquire(context.Background())
	if release == nil || err == nil || !h.IsProviderError(err) || !h.IsProviderError(fmt.Errorf("nested: %w", err)) {
		return lockFailure(fmt.Sprintf("lost acquire tuple release=%v", release), err)
	}
	if err := h.Control.FailNext(context.Background(), config, OperationRelease, errors.New("injected-cause")); err != nil {
		return err
	}
	deleted, err := release(context.Background())
	if deleted || err == nil || !h.IsProviderError(err) {
		return lockFailure(fmt.Sprintf("lost release tuple deleted=%v", deleted), err)
	}
	if deleted, err := release(context.Background()); err != nil || deleted {
		return lockFailure(fmt.Sprintf("release retry deleted=%v", deleted), err)
	}
	return nil
}

func runStaleRelease(t *testing.T, h Harness, config Config) error {
	first, err := makeAcquire(t, h, config)
	if err != nil {
		return err
	}
	staleRelease, err := first(context.Background())
	if err != nil {
		return err
	}
	time.Sleep(config.TTL + 10*time.Millisecond)
	other := config
	other.Owner += "-other"
	second, err := makeAcquire(t, h, other)
	if err != nil {
		return err
	}
	activeRelease, err := second(context.Background())
	if err != nil {
		return err
	}
	if deleted, err := staleRelease(context.Background()); err != nil || deleted {
		return lockFailure(fmt.Sprintf("stale release deleted=%v", deleted), err)
	}
	owner, err := h.Control.Owner(context.Background(), other)
	if err != nil || owner != other.Owner {
		return lockFailure(fmt.Sprintf("replacement owner=%q", owner), err)
	}
	_, err = activeRelease(context.Background())
	return err
}

func lockFailure(message string, err error) error {
	if err != nil {
		return fmt.Errorf("%s: %w", message, err)
	}
	return errors.New(message)
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
	takeover, err := makeAcquire(t, h, config)
	if err != nil {
		return err
	}
	release, err := takeover(context.Background())
	if err != nil {
		return err
	}
	_, err = release(context.Background())
	return err
}
