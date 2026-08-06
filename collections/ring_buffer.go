package collections

import "fmt"

// RingBuffer 패키지에서 공개하는 구조체다.
type RingBuffer[T any] struct {
	values   []T
	start    int
	length   int
	capacity int
}

// NewRingBuffer RingBuffer 인스턴스를 생성한다.
//
// 매개변수:
//   - capacity: NewRingBuffer에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
//
// 반환 오류는 입력 검증 실패와 패키지에서 정의한 sentinel error/typed error를 그대로 드러낸다.
func NewRingBuffer[T any](capacity int) (*RingBuffer[T], error) {
	if capacity <= 0 {
		return nil, fmt.Errorf("%w: ring buffer capacity[%d] must be positive", ErrInvalidArgument, capacity)
	}
	return &RingBuffer[T]{
		values:   make([]T, capacity),
		capacity: capacity,
	}, nil
}

// Capacity 저장 가능한 최대 항목 수를 반환한다.
func (r *RingBuffer[T]) Capacity() int {
	if r == nil {
		return 0
	}
	return r.capacity
}

// Len 현재 항목 수를 반환한다.
func (r *RingBuffer[T]) Len() int {
	if r == nil {
		return 0
	}
	return r.length
}

// Empty 저장된 항목이 없는지 반환한다.
func (r *RingBuffer[T]) Empty() bool {
	return r.Len() == 0
}

// Add 현재 값에 입력 값을 더한 결과를 반환한다.
//
// 매개변수:
//   - value: Add에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
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

// AddAll 여러 값을 순서대로 추가한다.
//
// 매개변수:
//   - values: AddAll에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
func (r *RingBuffer[T]) AddAll(values ...T) {
	for _, value := range values {
		r.Add(value)
	}
}

// At index 위치의 값을 반환한다.
//
// 매개변수:
//   - index: At에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
func (r *RingBuffer[T]) At(index int) (T, bool) {
	var zero T
	if r == nil || index < 0 || index >= r.length {
		return zero, false
	}
	return r.values[(r.start+index)%r.capacity], true
}

// Values 현재 값을 슬라이스로 반환한다.
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

// Drop 앞쪽 n개 항목을 제거한다.
//
// 매개변수:
//   - n: Drop에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
//
// 반환 오류는 입력 검증 실패와 패키지에서 정의한 sentinel error/typed error를 그대로 드러낸다.
func (r *RingBuffer[T]) Drop(n int) error {
	if n < 0 {
		return fmt.Errorf("%w: drop count[%d] must be non-negative", ErrInvalidArgument, n)
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

// Clear 저장된 항목을 모두 제거한다.
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
