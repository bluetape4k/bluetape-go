package collections

import "fmt"

// BoundedStack 패키지에서 공개하는 구조체다.
type BoundedStack[T any] struct {
	values   []T
	capacity int
}

// NewBoundedStack BoundedStack 인스턴스를 생성한다.
//
// 매개변수:
//   - capacity: NewBoundedStack에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
//
// 반환 오류는 입력 검증 실패와 패키지에서 정의한 sentinel error/typed error를 그대로 드러낸다.
func NewBoundedStack[T any](capacity int) (*BoundedStack[T], error) {
	if capacity <= 0 {
		return nil, fmt.Errorf("%w: bounded stack capacity[%d] must be positive", ErrInvalidArgument, capacity)
	}
	return &BoundedStack[T]{
		values:   make([]T, 0, capacity),
		capacity: capacity,
	}, nil
}

// Capacity 저장 가능한 최대 항목 수를 반환한다.
func (s *BoundedStack[T]) Capacity() int {
	if s == nil {
		return 0
	}
	return s.capacity
}

// Len 현재 항목 수를 반환한다.
func (s *BoundedStack[T]) Len() int {
	if s == nil {
		return 0
	}
	return len(s.values)
}

// Empty 저장된 항목이 없는지 반환한다.
func (s *BoundedStack[T]) Empty() bool {
	return s.Len() == 0
}

// Push 값을 추가한다.
//
// 매개변수:
//   - value: Push에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
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

// PushAll 여러 값을 순서대로 stack에 추가한다.
//
// 매개변수:
//   - values: PushAll에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
func (s *BoundedStack[T]) PushAll(values ...T) {
	for _, value := range values {
		s.Push(value)
	}
}

// Pop 마지막 값을 제거하고 반환한다.
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

// Peek 마지막 값을 제거하지 않고 반환한다.
func (s *BoundedStack[T]) Peek() (T, bool) {
	if s == nil || len(s.values) == 0 {
		var zero T
		return zero, false
	}
	return s.values[len(s.values)-1], true
}

// At index 위치의 값을 반환한다.
//
// 매개변수:
//   - index: At에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
func (s *BoundedStack[T]) At(index int) (T, bool) {
	var zero T
	if s == nil || index < 0 || index >= len(s.values) {
		return zero, false
	}
	return s.values[len(s.values)-1-index], true
}

// Values 현재 값을 슬라이스로 반환한다.
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

// Clear 저장된 항목을 모두 제거한다.
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
