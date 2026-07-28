package collections

import "fmt"

// RingBuffer는 struct 공개 타입이다.
// 값의 zero value, nil 허용 여부, 동시성 소유권은 생성자와 메서드의 한국어 주석 및 테스트 계약을 따른다.
type RingBuffer[T any] struct {
	values   []T
	start    int
	length   int
	capacity int
}

// NewRingBuffer는 NewRingBuffer 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - capacity: NewRingBuffer 동작에 필요한 capacity 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//
// 반환 오류는 입력 검증 실패, 취소, 외부 원인, 또는 패키지 sentinel/typed error 계약을 보존한다.
func NewRingBuffer[T any](capacity int) (*RingBuffer[T], error) {
	if capacity <= 0 {
		return nil, fmt.Errorf("%w: ring buffer capacity[%d] must be positive", ErrInvalidArgument, capacity)
	}
	return &RingBuffer[T]{
		values:   make([]T, capacity),
		capacity: capacity,
	}, nil
}

// Capacity는 Capacity 공개 API의 동작을 수행한다.
func (r *RingBuffer[T]) Capacity() int {
	if r == nil {
		return 0
	}
	return r.capacity
}

// Len는 Len 공개 API의 동작을 수행한다.
func (r *RingBuffer[T]) Len() int {
	if r == nil {
		return 0
	}
	return r.length
}

// Empty는 Empty 공개 API의 동작을 수행한다.
func (r *RingBuffer[T]) Empty() bool {
	return r.Len() == 0
}

// Add는 Add 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - value: Add 동작에 필요한 value 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
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

// AddAll는 AddAll 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - values: AddAll 동작에 필요한 values 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
func (r *RingBuffer[T]) AddAll(values ...T) {
	for _, value := range values {
		r.Add(value)
	}
}

// At는 At 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - index: At 동작에 필요한 index 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
func (r *RingBuffer[T]) At(index int) (T, bool) {
	var zero T
	if r == nil || index < 0 || index >= r.length {
		return zero, false
	}
	return r.values[(r.start+index)%r.capacity], true
}

// Values는 Values 공개 API의 동작을 수행한다.
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

// Drop는 Drop 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - n: Drop 동작에 필요한 n 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//
// 반환 오류는 입력 검증 실패, 취소, 외부 원인, 또는 패키지 sentinel/typed error 계약을 보존한다.
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

// Clear는 Clear 공개 API의 동작을 수행한다.
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
