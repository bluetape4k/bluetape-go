package concurrency

import (
	"fmt"
	"sync/atomic"
)

// RoundRobin 패키지에서 공개하는 구조체다.
type RoundRobin struct {
	maximum uint64
	counter atomic.Uint64
}

// NewRoundRobin RoundRobin 인스턴스를 생성한다.
//
// 매개변수:
//   - maximum: NewRoundRobin에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
//
// 반환 오류는 입력 검증 실패와 패키지에서 정의한 sentinel error/typed error를 그대로 드러낸다.
func NewRoundRobin(maximum int) (*RoundRobin, error) {
	if maximum <= 0 {
		return nil, fmt.Errorf("maximum must be positive")
	}
	return &RoundRobin{maximum: uint64(maximum)}, nil
}

// Get key에 해당하는 값을 조회한다.
func (r *RoundRobin) Get() int {
	if r == nil || r.maximum == 0 {
		return 0
	}
	return int(r.counter.Load())
}

// Set key에 값을 저장한다.
//
// 매개변수:
//   - value: Set에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
//
// 반환 오류는 입력 검증 실패와 패키지에서 정의한 sentinel error/typed error를 그대로 드러낸다.
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

// Next 다음 항목을 round-robin 순서로 반환한다.
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
