package bttesting_test

import (
	"sync/atomic"
	"testing"
	"time"

	bttesting "github.com/bluetape4k/bluetape-go/testing"
)

func TestEventually(t *testing.T) {
	var ready atomic.Bool
	go func() {
		time.Sleep(10 * time.Millisecond)
		ready.Store(true)
	}()

	bttesting.Eventually(t, time.Second, ready.Load)
}

func TestEventuallyWithPolling(t *testing.T) {
	var ready atomic.Bool
	go func() {
		time.Sleep(10 * time.Millisecond)
		ready.Store(true)
	}()

	bttesting.EventuallyWithPolling(t, time.Second, 5*time.Millisecond, ready.Load)
}

func TestConsistently(t *testing.T) {
	var ready atomic.Bool
	ready.Store(true)

	bttesting.Consistently(t, 50*time.Millisecond, ready.Load)
}

func TestConsistentlyWithPolling(t *testing.T) {
	var ready atomic.Bool
	ready.Store(true)

	bttesting.ConsistentlyWithPolling(t, 50*time.Millisecond, 5*time.Millisecond, ready.Load)
}
