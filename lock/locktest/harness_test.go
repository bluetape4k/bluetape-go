package locktest

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestHarnessAndClassifierValidation(t *testing.T) {
	if validateHarness(Harness{}) == nil {
		t.Fatal("empty harness passed")
	}
	if err := validateHarness(MemoryHarness()); err != nil {
		t.Fatalf("MemoryHarness validation = %v", err)
	}
	for _, classifier := range []ErrorClassifier{
		func(error) bool { return true },
		func(error) bool { return false },
		func(error) bool { panic("boom") },
	} {
		h := MemoryHarness()
		h.IsProviderError = classifier
		if err := validateHarness(h); err == nil {
			if err := validatePositiveClassifier(t, h); err == nil {
				t.Fatal("broken classifier passed")
			}
		}
	}
}

func TestMemoryGateIsIdempotentAndCancelable(t *testing.T) {
	h := MemoryHarness()
	config := Config{Key: "gate", Owner: "owner", TTL: time.Second}
	gate, err := h.Control.GateNext(context.Background(), config, OperationAcquire, PhaseBeforeLinearize)
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

func TestMemoryControlInvalidInputHasZeroCount(t *testing.T) {
	h := MemoryHarness()
	invalid := Config{}
	if gate, err := h.Control.GateNext(context.Background(), invalid, Operation("bad"), Phase("bad")); err == nil || gate != nil {
		t.Fatalf("invalid gate = %v, %v", gate, err)
	}
	if count := h.Control.OperationCount(invalid, OperationAcquire); count != 0 {
		t.Fatalf("invalid count = %d", count)
	}
}
