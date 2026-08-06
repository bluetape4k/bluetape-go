package ratelimittest_test

import (
	"fmt"
	"testing"

	"github.com/bluetape4k/bluetape-go/ratelimit/ratelimittest"
)

func TestProviderConformance(t *testing.T) {
	harness := ratelimittest.MemoryHarness()
	ratelimittest.Run(t, harness)
}

func ExampleMemoryHarness() {
	harness := ratelimittest.MemoryHarness()
	fmt.Println(harness.New != nil, harness.Control != nil, harness.IsProviderError != nil)
	// Output: true true true
}
