package collections

import "fmt"

// BoundedStack는 struct 공개 타입이다.
// 값의 zero value, nil 허용 여부, 동시성 소유권은 생성자와 메서드의 한국어 주석 및 테스트 계약을 따른다.
type BoundedStack[T any] struct {
	values   []T
	capacity int
}

// NewBoundedStack는 NewBoundedStack 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - capacity: NewBoundedStack 동작에 필요한 capacity 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//
// 반환 오류는 입력 검증 실패, 취소, 외부 원인, 또는 패키지 sentinel/typed error 계약을 보존한다.
func NewBoundedStack[T any](capacity int) (*BoundedStack[T], error) {
	if capacity <= 0 {
		return nil, fmt.Errorf("%w: bounded stack capacity[%d] must be positive", ErrInvalidArgument, capacity)
	}
	return &BoundedStack[T]{
		values:   make([]T, 0, capacity),
		capacity: capacity,
	}, nil
}

// Capacity는 Capacity 공개 API의 동작을 수행한다.
func (s *BoundedStack[T]) Capacity() int {
	if s == nil {
		return 0
	}
	return s.capacity
}

// Len는 Len 공개 API의 동작을 수행한다.
func (s *BoundedStack[T]) Len() int {
	if s == nil {
		return 0
	}
	return len(s.values)
}

// Empty는 Empty 공개 API의 동작을 수행한다.
func (s *BoundedStack[T]) Empty() bool {
	return s.Len() == 0
}

// Push는 Push 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - value: Push 동작에 필요한 value 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
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

// PushAll는 PushAll 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - values: PushAll 동작에 필요한 values 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
func (s *BoundedStack[T]) PushAll(values ...T) {
	for _, value := range values {
		s.Push(value)
	}
}

// Pop는 Pop 공개 API의 동작을 수행한다.
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

// Peek는 Peek 공개 API의 동작을 수행한다.
func (s *BoundedStack[T]) Peek() (T, bool) {
	if s == nil || len(s.values) == 0 {
		var zero T
		return zero, false
	}
	return s.values[len(s.values)-1], true
}

// At는 At 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - index: At 동작에 필요한 index 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
func (s *BoundedStack[T]) At(index int) (T, bool) {
	var zero T
	if s == nil || index < 0 || index >= len(s.values) {
		return zero, false
	}
	return s.values[len(s.values)-1-index], true
}

// Values는 Values 공개 API의 동작을 수행한다.
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

// Clear는 Clear 공개 API의 동작을 수행한다.
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
