package leadertest_test

import (
	"fmt"
	"testing"

	"github.com/bluetape4k/bluetape-go/leader/leadertest"
)

var _ = leadertest.Harness{nil, nil}

func TestProviderConformance(t *testing.T) {
	harness := leadertest.MemoryHarness()
	leadertest.Run(t, harness)
}

func ExampleMemoryHarness() {
	harness := leadertest.MemoryHarness()
	fmt.Println(harness.New != nil, harness.Control != nil)
	// Output: true true
}
