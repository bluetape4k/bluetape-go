package ratelimittest

import "testing"

func TestRunMemoryHarness(t *testing.T) { Run(t, MemoryHarness()) }
