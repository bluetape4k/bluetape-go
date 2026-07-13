package ratelimittest

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"testing"
	"time"
)

var runnerID atomic.Uint64

const conformanceWaitTimeout = 2 * time.Second
const conformanceCaseTimeout = 5 * time.Second

// Run executes all mandatory token-bucket conformance cases.
func Run(t *testing.T, harness Harness) {
	t.Helper()
	if err := validateHarness(harness); err != nil {
		t.Fatal("ratelimittest: invalid harness")
	}
	classifierResult := make(chan error, 1)
	go func() { classifierResult <- validatePositiveClassifier(t, harness) }()
	select {
	case err := <-classifierResult:
		if err != nil {
			t.Fatal("ratelimittest: provider classifier probe failed")
		}
	case <-time.After(conformanceCaseTimeout):
		t.Fatal("ratelimittest: provider classifier probe timed out")
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
		passed := t.Run(tc.name, func(t *testing.T) {
			id := runnerID.Add(1)
			config := Config{RatePerSecond: 100, Burst: 5, IdleTTL: time.Second}
			result := make(chan error, 1)
			go func() { result <- tc.run(t, harness, config, fmt.Sprintf("ratelimittest-%s-%d", tc.name, id)) }()
			select {
			case err := <-result:
				if err != nil {
					t.Fatalf("ratelimittest: conformance case failed: %s", conformanceFailureReason(harness, err))
				}
			case <-time.After(conformanceCaseTimeout):
				t.Fatal("ratelimittest: conformance case timed out")
			}
		})
		if !passed {
			return
		}
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
	allow, err := makeAllow(t, h, config)
	if err != nil {
		return err
	}
	result, err := allow(context.Background(), key, config.Burst)
	if err != nil || !result.Allowed || result.Remaining != 0 || result.Requested != config.Burst {
		return rateFailure(fmt.Sprintf("initial burst=%+v", result), err)
	}
	return nil
}

func runOverBurst(t *testing.T, h Harness, config Config, key string) error {
	allow, err := makeAllow(t, h, config)
	if err != nil {
		return err
	}
	before := h.Control.OperationCount(key)
	result, err := allow(context.Background(), key, config.Burst+1)
	if err == nil || result != (Result{}) || h.Control.OperationCount(key) != before {
		return rateFailure(fmt.Sprintf("over burst=%+v", result), err)
	}
	return nil
}

func runRejection(t *testing.T, h Harness, config Config, key string) error {
	config = noRefillDuringCase(config)
	allow, err := makeAllow(t, h, config)
	if err != nil {
		return err
	}
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
	allow, err := makeAllow(t, h, config)
	if err != nil {
		return err
	}
	if _, err := allow(context.Background(), key, config.Burst); err != nil {
		return err
	}
	deadline := time.Now().Add(conformanceWaitTimeout)
	for {
		result, err := allow(context.Background(), key, 1)
		if err != nil {
			return rateFailure(fmt.Sprintf("refill=%+v", result), err)
		}
		if result.Allowed {
			return nil
		}
		if result.RetryAfter <= 0 {
			return rateFailure(fmt.Sprintf("refill=%+v", result), nil)
		}
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return errors.New("ratelimittest: refill did not become available")
		}
		wait := min(result.RetryAfter, remaining)
		time.Sleep(wait)
	}
}

func runKeyIsolation(t *testing.T, h Harness, config Config, key string) error {
	allow, err := makeAllow(t, h, config)
	if err != nil {
		return err
	}
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
	allow, err := makeAllow(t, h, config)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	result, err := allow(ctx, key, 1)
	if result != (Result{}) || !errors.Is(err, context.Canceled) || h.Control.OperationCount(key) != 0 {
		return rateFailure(fmt.Sprintf("pre-canceled=%+v", result), err)
	}
	return nil
}

func runCancelBefore(t *testing.T, h Harness, config Config, key string) error {
	allow, err := makeAllow(t, h, config)
	if err != nil {
		return err
	}
	gate, err := h.Control.GateNext(context.Background(), key, PhaseBeforeLinearize)
	if err != nil || gate == nil {
		return rateFailure(fmt.Sprintf("gate=%v", gate), err)
	}
	defer gate.Resume()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	resultCh := make(chan error, 1)
	go func() { _, err := allow(ctx, key, 1); resultCh <- err }()
	waitCtx, waitCancel := context.WithTimeout(context.Background(), conformanceWaitTimeout)
	err = gate.AwaitStarted(waitCtx)
	waitCancel()
	if err != nil {
		cancel()
		gate.Resume()
		return errors.New("ratelimittest: allow gate did not start")
	}
	cancel()
	select {
	case err = <-resultCh:
	case <-time.After(conformanceWaitTimeout):
		gate.Resume()
		return errors.New("ratelimittest: canceled allow did not return")
	}
	if !errors.Is(err, context.Canceled) {
		return rateFailure("before cancellation mismatch", err)
	}
	result, err := allow(context.Background(), key, config.Burst)
	if err != nil || !result.Allowed {
		return rateFailure(fmt.Sprintf("before cancellation consumed quota=%+v", result), err)
	}
	return nil
}

func runCancelAfter(t *testing.T, h Harness, config Config, key string) error {
	config = noRefillDuringCase(config)
	allow, err := makeAllow(t, h, config)
	if err != nil {
		return err
	}
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
	waitCtx, waitCancel := context.WithTimeout(context.Background(), conformanceWaitTimeout)
	err = gate.AwaitStarted(waitCtx)
	waitCancel()
	if err != nil {
		cancel()
		gate.Resume()
		return errors.New("ratelimittest: allow gate did not start")
	}
	cancel()
	gate.Resume()
	var got response
	select {
	case got = <-resultCh:
	case <-time.After(conformanceWaitTimeout):
		return errors.New("ratelimittest: linearized allow did not return")
	}
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
	config = noRefillDuringCase(config)
	allow, err := makeAllow(t, h, config)
	if err != nil {
		return err
	}
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

func noRefillDuringCase(config Config) Config {
	config.RatePerSecond = 0.1
	config.IdleTTL = time.Minute
	return config
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
	allow, err := makeAllow(t, h, config)
	if err != nil {
		return err
	}
	requests := int(config.Burst + 7)
	ctx, cancel := context.WithTimeout(context.Background(), conformanceWaitTimeout)
	defer cancel()
	type concurrencyResult struct {
		result Result
		err    error
	}
	results := make(chan concurrencyResult, requests)
	for range requests {
		go func() {
			result, err := allow(ctx, key, 1)
			results <- concurrencyResult{result: result, err: err}
		}()
	}
	var allowed int64
	var rejected int64
	for range requests {
		select {
		case got := <-results:
			if got.err == nil && got.result.Allowed {
				allowed += got.result.Requested
			} else if got.err == nil {
				rejected++
			}
		case <-ctx.Done():
			return errors.New("concurrency workers did not return")
		}
	}
	if allowed != config.Burst || rejected != int64(requests)-config.Burst {
		return fmt.Errorf("concurrency totals allowed=%d rejected=%d", allowed, rejected)
	}
	return nil
}
