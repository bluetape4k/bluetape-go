package concurrency

import (
	"fmt"
	"sync/atomic"
)

// RoundRobin struct 공개 타입이다.
// 값의 zero value, nil 허용 여부, 동시성 소유권은 생성자와 메서드의 한국어 주석 및 테스트 계약을 따른다.
type RoundRobin struct {
	maximum uint64
	counter atomic.Uint64
}

// NewRoundRobin NewRoundRobin 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - maximum: NewRoundRobin 동작에 필요한 maximum 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//
// 반환 오류는 입력 검증 실패, 취소, 외부 원인, 또는 패키지 sentinel/typed error 계약을 보존한다.
func NewRoundRobin(maximum int) (*RoundRobin, error) {
	if maximum <= 0 {
		return nil, fmt.Errorf("maximum must be positive")
	}
	return &RoundRobin{maximum: uint64(maximum)}, nil
}

// Get Get 공개 API의 동작을 수행한다.
func (r *RoundRobin) Get() int {
	if r == nil || r.maximum == 0 {
		return 0
	}
	return int(r.counter.Load())
}

// Set Set 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - value: Set 동작에 필요한 value 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//
// 반환 오류는 입력 검증 실패, 취소, 외부 원인, 또는 패키지 sentinel/typed error 계약을 보존한다.
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

// Next Next 공개 API의 동작을 수행한다.
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
