package bttesting

import (
	"testing"
	"time"
)

// Eventually keeps evaluating condition until it succeeds or the timeout ends.
func Eventually(t *testing.T, timeout time.Duration, condition func() bool) {
	t.Helper()

	deadline := time.Now().Add(timeout)
	for {
		if condition() {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("condition did not become true within %s", timeout)
		}
		time.Sleep(25 * time.Millisecond)
	}
}
