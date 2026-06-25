package collections

import "fmt"

// RingBuffer is a fixed-capacity FIFO-oriented buffer.
//
// Adding beyond capacity overwrites the oldest value. RingBuffer is not
// goroutine-safe and is not a blocking queue.
type RingBuffer[T any] struct {
	values   []T
	start    int
	length   int
	capacity int
}

// NewRingBuffer creates a fixed-capacity ring buffer.
func NewRingBuffer[T any](capacity int) (*RingBuffer[T], error) {
	if capacity <= 0 {
		return nil, fmt.Errorf("ring buffer capacity[%d] must be positive", capacity)
	}
	return &RingBuffer[T]{
		values:   make([]T, capacity),
		capacity: capacity,
	}, nil
}

// Capacity returns the maximum number of retained values.
func (r *RingBuffer[T]) Capacity() int {
	if r == nil {
		return 0
	}
	return r.capacity
}

// Len returns the number of retained values.
func (r *RingBuffer[T]) Len() int {
	if r == nil {
		return 0
	}
	return r.length
}

// Empty reports whether the ring has no values.
func (r *RingBuffer[T]) Empty() bool {
	return r.Len() == 0
}

// Add appends value, overwriting the oldest value when the ring is full.
func (r *RingBuffer[T]) Add(value T) {
	if r == nil || r.capacity <= 0 {
		return
	}
	if r.length < r.capacity {
		index := (r.start + r.length) % r.capacity
		r.values[index] = value
		r.length++
		return
	}
	r.values[r.start] = value
	r.start = (r.start + 1) % r.capacity
}

// AddAll appends values in argument order.
func (r *RingBuffer[T]) AddAll(values ...T) {
	for _, value := range values {
		r.Add(value)
	}
}

// At returns the value at index, where index 0 is the oldest retained value.
func (r *RingBuffer[T]) At(index int) (T, bool) {
	var zero T
	if r == nil || index < 0 || index >= r.length {
		return zero, false
	}
	return r.values[(r.start+index)%r.capacity], true
}

// Values returns an oldest-to-newest shallow snapshot.
func (r *RingBuffer[T]) Values() []T {
	if r == nil {
		return nil
	}
	result := make([]T, r.length)
	for index := 0; index < r.length; index++ {
		result[index] = r.values[(r.start+index)%r.capacity]
	}
	return result
}

// Drop removes n oldest values.
func (r *RingBuffer[T]) Drop(n int) error {
	if n < 0 {
		return fmt.Errorf("drop count[%d] must be non-negative", n)
	}
	if r == nil || n == 0 {
		return nil
	}
	if n >= r.length {
		r.Clear()
		return nil
	}
	var zero T
	for index := 0; index < n; index++ {
		r.values[(r.start+index)%r.capacity] = zero
	}
	r.start = (r.start + n) % r.capacity
	r.length -= n
	return nil
}

// Clear removes all values.
func (r *RingBuffer[T]) Clear() {
	if r == nil {
		return
	}
	var zero T
	for index := 0; index < r.length; index++ {
		r.values[(r.start+index)%r.capacity] = zero
	}
	r.start = 0
	r.length = 0
}
