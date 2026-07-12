package ratelimittest

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

// Run executes all mandatory token-bucket conformance cases.
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
		run  func(*testing.T, Harness, Config, string) error
	}{
		{"initial-burst", runInitialBurst},
		{"over-burst-validation", runOverBurst},
		{"rejection-result", runRejection},
		{"refill", runRefill},
		{"key-isolation", runKeyIsolation},
		{"pre-canceled", runPreCanceled},
		{"cancel-before-linearize", runCancelBefore},
		{"cancel-after-linearize", runCancelAfter},
		{"lost-response", runLostResponse},
		{"exact-concurrency", runExactConcurrency},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			id := runnerID.Add(1)
			config := Config{RatePerSecond: 100, Burst: 5, IdleTTL: time.Second}
			if err := tc.run(t, harness, config, fmt.Sprintf("ratelimittest-%s-%d", tc.name, id)); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func makeAllow(t *testing.T, h Harness, config Config) (AllowFunc, error) {
	t.Helper()
	allow, err := h.New(t, config)
	if err != nil {
		return nil, err
	}
	if allow == nil {
		return nil, errors.New("ratelimittest: nil allow function")
	}
	return allow, nil
}

func runInitialBurst(t *testing.T, h Harness, config Config, key string) error {
	allow, _ := makeAllow(t, h, config)
	result, err := allow(context.Background(), key, config.Burst)
	if err != nil || !result.Allowed || result.Remaining != 0 || result.Requested != config.Burst {
		return rateFailure(fmt.Sprintf("initial burst=%+v", result), err)
	}
	return nil
}

func runOverBurst(t *testing.T, h Harness, config Config, key string) error {
	allow, _ := makeAllow(t, h, config)
	before := h.Control.OperationCount(key)
	result, err := allow(context.Background(), key, config.Burst+1)
	if err == nil || result != (Result{}) || h.Control.OperationCount(key) != before {
		return rateFailure(fmt.Sprintf("over burst=%+v", result), err)
	}
	return nil
}

func runRejection(t *testing.T, h Harness, config Config, key string) error {
	allow, _ := makeAllow(t, h, config)
	if _, err := allow(context.Background(), key, config.Burst); err != nil {
		return err
	}
	result, err := allow(context.Background(), key, 1)
	if err != nil || result.Allowed || result.RetryAfter <= 0 || result.Requested != 1 {
		return rateFailure(fmt.Sprintf("rejection=%+v", result), err)
	}
	return nil
}

func runRefill(t *testing.T, h Harness, config Config, key string) error {
	allow, _ := makeAllow(t, h, config)
	if _, err := allow(context.Background(), key, config.Burst); err != nil {
		return err
	}
	time.Sleep(2*time.Duration(float64(time.Second)/config.RatePerSecond) + 5*time.Millisecond)
	result, err := allow(context.Background(), key, 1)
	if err != nil || !result.Allowed {
		return rateFailure(fmt.Sprintf("refill=%+v", result), err)
	}
	return nil
}

func runKeyIsolation(t *testing.T, h Harness, config Config, key string) error {
	allow, _ := makeAllow(t, h, config)
	if _, err := allow(context.Background(), key, config.Burst); err != nil {
		return err
	}
	result, err := allow(context.Background(), key+"-other", config.Burst)
	if err != nil || !result.Allowed || result.Remaining != 0 {
		return rateFailure(fmt.Sprintf("key isolation=%+v", result), err)
	}
	return nil
}

func runPreCanceled(t *testing.T, h Harness, config Config, key string) error {
	allow, _ := makeAllow(t, h, config)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	result, err := allow(ctx, key, 1)
	if result != (Result{}) || !errors.Is(err, context.Canceled) || h.Control.OperationCount(key) != 0 {
		return rateFailure(fmt.Sprintf("pre-canceled=%+v", result), err)
	}
	return nil
}

func runCancelBefore(t *testing.T, h Harness, config Config, key string) error {
	allow, _ := makeAllow(t, h, config)
	gate, err := h.Control.GateNext(context.Background(), key, PhaseBeforeLinearize)
	if err != nil || gate == nil {
		return rateFailure(fmt.Sprintf("gate=%v", gate), err)
	}
	defer gate.Resume()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	resultCh := make(chan error, 1)
	go func() { _, err := allow(ctx, key, 1); resultCh <- err }()
	if err := gate.AwaitStarted(context.Background()); err != nil {
		return err
	}
	cancel()
	if err := <-resultCh; !errors.Is(err, context.Canceled) {
		return rateFailure("before cancellation mismatch", err)
	}
	result, err := allow(context.Background(), key, config.Burst)
	if err != nil || !result.Allowed {
		return rateFailure(fmt.Sprintf("before cancellation consumed quota=%+v", result), err)
	}
	return nil
}

func runCancelAfter(t *testing.T, h Harness, config Config, key string) error {
	allow, _ := makeAllow(t, h, config)
	gate, err := h.Control.GateNext(context.Background(), key, PhaseAfterLinearize)
	if err != nil {
		return err
	}
	defer gate.Resume()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	type response struct {
		result Result
		err    error
	}
	resultCh := make(chan response, 1)
	go func() { result, err := allow(ctx, key, 1); resultCh <- response{result, err} }()
	if err := gate.AwaitStarted(context.Background()); err != nil {
		return err
	}
	cancel()
	gate.Resume()
	got := <-resultCh
	if got.err != nil || !got.result.Allowed {
		return rateFailure(fmt.Sprintf("after cancellation=%+v", got.result), got.err)
	}
	next, err := allow(context.Background(), key, config.Burst)
	if err != nil || next.Allowed {
		return rateFailure(fmt.Sprintf("after cancellation debit missing=%+v", next), err)
	}
	return nil
}

func runLostResponse(t *testing.T, h Harness, config Config, key string) error {
	allow, _ := makeAllow(t, h, config)
	if err := h.Control.FailNext(context.Background(), key, errors.New("injected-cause")); err != nil {
		return err
	}
	result, err := allow(context.Background(), key, 1)
	if err == nil {
		if !result.Allowed || result.Requested != 1 {
			return fmt.Errorf("confirmed lost response = %+v", result)
		}
	} else if result != (Result{}) || !h.IsProviderError(err) || !h.IsProviderError(fmt.Errorf("nested: %w", err)) {
		return rateFailure(fmt.Sprintf("indeterminate lost response=%+v", result), err)
	}
	if count := h.Control.OperationCount(key); count != 1 {
		return fmt.Errorf("lost response count = %d", count)
	}
	next, err := allow(context.Background(), key, config.Burst)
	if err != nil || next.Allowed {
		return rateFailure(fmt.Sprintf("lost response debit missing=%+v", next), err)
	}
	return nil
}

func rateFailure(message string, err error) error {
	if err != nil {
		return fmt.Errorf("%s: %w", message, err)
	}
	return errors.New(message)
}

func runExactConcurrency(t *testing.T, h Harness, config Config, key string) error {
	config.RatePerSecond = 1
	config.IdleTTL = 10 * time.Second
	allow, _ := makeAllow(t, h, config)
	requests := int(config.Burst + 7)
	var wg sync.WaitGroup
	var allowed atomic.Int64
	var rejected atomic.Int64
	for range requests {
		wg.Add(1)
		go func() {
			defer wg.Done()
			result, err := allow(context.Background(), key, 1)
			if err != nil {
				return
			}
			if result.Allowed {
				allowed.Add(result.Requested)
			} else {
				rejected.Add(1)
			}
		}()
	}
	wg.Wait()
	if allowed.Load() != config.Burst || rejected.Load() != int64(requests)-config.Burst {
		return fmt.Errorf("concurrency totals allowed=%d rejected=%d", allowed.Load(), rejected.Load())
	}
	return nil
}
