package locktest

import "testing"

func TestRunMemoryHarness(t *testing.T) {
	Run(t, MemoryHarness())
}
