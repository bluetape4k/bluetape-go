package locktest_test

import (
	"context"
	"testing"
	"time"

	"github.com/bluetape4k/bluetape-go/lock/locktest"
)

func ExampleRun() {
	harness := locktest.MemoryHarness()
	_ = func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		t.Cleanup(cancel)
		config := locktest.Config{Key: "example", Owner: "owner", TTL: time.Second}
		gate, err := harness.Control.GateNext(ctx, config, locktest.OperationAcquire, locktest.PhaseBeforeLinearize)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(gate.Resume)
		_ = harness.Control.FailNext(ctx, config, locktest.OperationRelease, context.DeadlineExceeded)
		locktest.Run(t, harness)
	}
}
