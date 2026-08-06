package ratelimittest

import (
	"context"
	"errors"
	"math"
	"testing"
	"time"
)

func TestConfigValidation(t *testing.T) {
	for _, config := range []Config{
		{},
		{RatePerSecond: math.NaN(), Burst: 1},
		{RatePerSecond: math.Inf(1), Burst: 1},
		{RatePerSecond: 1, Burst: 0},
		{RatePerSecond: 1, Burst: 2, IdleTTL: time.Second},
	} {
		if validateConfig(config) == nil {
			t.Fatalf("invalid config passed: %+v", config)
		}
	}
}

func TestHarnessClassifierValidation(t *testing.T) {
	if validateHarness(Harness{}) == nil {
		t.Fatal("empty harness passed")
	}
	if err := validateHarness(MemoryHarness()); err != nil {
		t.Fatal(err)
	}
	for _, classifier := range []ErrorClassifier{
		func(error) bool { return true },
		func(error) bool { return false },
		func(error) bool { panic("boom") },
	} {
		h := MemoryHarness()
		h.IsProviderError = classifier
		if validateHarness(h) == nil && validatePositiveClassifier(t, h) == nil {
			t.Fatal("broken classifier passed")
		}
	}
}

func TestGateResumeAndCanceledAwait(t *testing.T) {
	h := MemoryHarness()
	gate, err := h.Control.GateNext(context.Background(), "key", PhaseBeforeLinearize)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := gate.AwaitStarted(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("AwaitStarted error = %v", err)
	}
	gate.Resume()
	gate.Resume()
}
