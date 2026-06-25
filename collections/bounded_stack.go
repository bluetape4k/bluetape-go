package collections

import "fmt"

// BoundedStack is a fixed-capacity LIFO container.
//
// Pushing beyond capacity drops the oldest bottom value. BoundedStack is not
// goroutine-safe; callers that share it across goroutines must synchronize
// access externally.
type BoundedStack[T any] struct {
	values   []T
	capacity int
}

// NewBoundedStack creates a stack that keeps at most capacity values.
func NewBoundedStack[T any](capacity int) (*BoundedStack[T], error) {
	if capacity <= 0 {
		return nil, fmt.Errorf("%w: bounded stack capacity[%d] must be positive", ErrInvalidArgument, capacity)
	}
	return &BoundedStack[T]{
		values:   make([]T, 0, capacity),
		capacity: capacity,
	}, nil
}

// Capacity returns the maximum number of retained values.
func (s *BoundedStack[T]) Capacity() int {
	if s == nil {
		return 0
	}
	return s.capacity
}

// Len returns the number of retained values.
func (s *BoundedStack[T]) Len() int {
	if s == nil {
		return 0
	}
	return len(s.values)
}

// Empty reports whether the stack has no values.
func (s *BoundedStack[T]) Empty() bool {
	return s.Len() == 0
}

// Push adds value to the top of the stack.
func (s *BoundedStack[T]) Push(value T) {
	if s == nil || s.capacity <= 0 {
		return
	}
	s.values = append(s.values, value)
	if len(s.values) > s.capacity {
		overflow := len(s.values) - s.capacity
		copy(s.values, s.values[overflow:])
		var zero T
		for index := s.capacity; index < len(s.values); index++ {
			s.values[index] = zero
		}
		s.values = s.values[:s.capacity]
	}
}

// PushAll adds values to the top of the stack in argument order.
func (s *BoundedStack[T]) PushAll(values ...T) {
	for _, value := range values {
		s.Push(value)
	}
}

// Pop removes and returns the top value.
func (s *BoundedStack[T]) Pop() (T, bool) {
	var zero T
	if s == nil || len(s.values) == 0 {
		return zero, false
	}
	last := len(s.values) - 1
	value := s.values[last]
	s.values[last] = zero
	s.values = s.values[:last]
	return value, true
}

// Peek returns the top value without removing it.
func (s *BoundedStack[T]) Peek() (T, bool) {
	if s == nil || len(s.values) == 0 {
		var zero T
		return zero, false
	}
	return s.values[len(s.values)-1], true
}

// At returns the value at index, where index 0 is the top.
func (s *BoundedStack[T]) At(index int) (T, bool) {
	var zero T
	if s == nil || index < 0 || index >= len(s.values) {
		return zero, false
	}
	return s.values[len(s.values)-1-index], true
}

// Values returns a top-to-bottom shallow snapshot.
func (s *BoundedStack[T]) Values() []T {
	if s == nil {
		return nil
	}
	result := make([]T, len(s.values))
	for index := range s.values {
		result[index] = s.values[len(s.values)-1-index]
	}
	return result
}

// Clear removes all values.
func (s *BoundedStack[T]) Clear() {
	if s == nil {
		return
	}
	var zero T
	for index := range s.values {
		s.values[index] = zero
	}
	s.values = s.values[:0]
}
