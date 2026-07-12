package locktest_test

import (
	"fmt"
	"testing"

	"github.com/bluetape4k/bluetape-go/lock/locktest"
)

func TestProviderConformance(t *testing.T) {
	harness := locktest.MemoryHarness()
	locktest.Run(t, harness)
}

func ExampleMemoryHarness() {
	harness := locktest.MemoryHarness()
	fmt.Println(harness.New != nil, harness.Control != nil, harness.IsProviderError != nil)
	// Output: true true true
}
