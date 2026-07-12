package ratelimittest_test

import (
	"context"
	"testing"
	"time"

	"github.com/bluetape4k/bluetape-go/ratelimit/ratelimittest"
)

func ExampleRun() {
	harness := ratelimittest.MemoryHarness()
	_ = func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		t.Cleanup(cancel)
		gate, err := harness.Control.GateNext(ctx, "example", ratelimittest.PhaseBeforeLinearize)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(gate.Resume)
		_ = harness.Control.FailNext(ctx, "example", context.DeadlineExceeded)
		ratelimittest.Run(t, harness)
	}
}
