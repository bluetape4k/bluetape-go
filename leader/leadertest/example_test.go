package leadertest_test

import (
	"context"
	"testing"
	"time"

	"github.com/bluetape4k/bluetape-go/leader"
	"github.com/bluetape4k/bluetape-go/leader/leadertest"
)

func ExampleRun() {
	harness := leadertest.MemoryHarness()
	_ = func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		t.Cleanup(cancel)
		if err := harness.Control.FailNext(ctx, caseOptions(), leadertest.OperationCampaign, context.DeadlineExceeded); err != nil {
			t.Fatal(err)
		}
		leadertest.Run(t, harness)
	}
}

func caseOptions() leader.Options {
	return leader.Options{
		Group:         "example",
		MemberID:      "member",
		Lease:         time.Second,
		RenewInterval: 250 * time.Millisecond,
	}
}
