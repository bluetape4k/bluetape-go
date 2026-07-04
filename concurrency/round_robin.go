package concurrency

import (
	"fmt"
	"sync/atomic"
)

// RoundRobin is a goroutine-safe cyclic counter over [0, maximum).
type RoundRobin struct {
	maximum uint64
	counter atomic.Uint64
}

// NewRoundRobin creates a cyclic counter whose values are in [0, maximum).
func NewRoundRobin(maximum int) (*RoundRobin, error) {
	if maximum <= 0 {
		return nil, fmt.Errorf("maximum must be positive")
	}
	return &RoundRobin{maximum: uint64(maximum)}, nil
}

// Get returns the current counter value.
func (r *RoundRobin) Get() int {
	if r == nil || r.maximum == 0 {
		return 0
	}
	return int(r.counter.Load())
}

// Set sets the current counter value.
func (r *RoundRobin) Set(value int) error {
	if r == nil || r.maximum == 0 {
		return fmt.Errorf("round robin is nil")
	}
	if value < 0 || uint64(value) >= r.maximum {
		return fmt.Errorf("value must be in range [0,%d)", r.maximum)
	}
	r.counter.Store(uint64(value))
	return nil
}

// Next increments the counter and returns the next value.
func (r *RoundRobin) Next() int {
	if r == nil || r.maximum == 0 {
		return 0
	}
	for {
		current := r.counter.Load()
		next := current + 1
		if next >= r.maximum {
			next = 0
		}
		if r.counter.CompareAndSwap(current, next) {
			return int(next)
		}
	}
}
