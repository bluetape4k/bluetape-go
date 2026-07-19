package leadertest_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/bluetape4k/bluetape-go/leader/leadertest"
)

var _ = leadertest.Harness{nil, nil}

func TestProviderConformance(t *testing.T) {
	harness := leadertest.MemoryHarness()
	leadertest.Run(t, harness)
}

func ExampleRunWithConfig() {
	run := func(t *testing.T, harness leadertest.Harness, abort leadertest.AbortFunc) {
		leadertest.RunWithConfig(t, harness, leadertest.Config{
			Timing: leadertest.Timing{CaseTimeout: 10 * time.Second},
			Abort:  abort,
		})
	}

	_ = run // Execute through `go test -timeout` so an unjoinable provider fail-stops.
}

func ExampleMemoryHarness() {
	harness := leadertest.MemoryHarness()
	fmt.Println(harness.New != nil, harness.Control != nil)
	// Output: true true
}
