package core

import (
	"cmp"
	"fmt"
	"math"
)

// Range 패키지에서 공개하는 구조체다.
type Range[T cmp.Ordered] struct {
	lower          T
	upper          T
	lowerInclusive bool
	upperInclusive bool
}

// ClosedRange 양쪽 경계를 포함하는 range를 만든다.
//
// 매개변수:
//   - lower: ClosedRange에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
//   - upper: ClosedRange에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
//
// 반환 오류는 입력 검증 실패와 패키지에서 정의한 sentinel error/typed error를 그대로 드러낸다.
func ClosedRange[T cmp.Ordered](lower, upper T) (Range[T], error) {
	return newRange(lower, upper, true, true)
}

// ClosedOpenRange 하한만 포함하는 range를 만든다.
//
// 매개변수:
//   - lower: ClosedOpenRange에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
//   - upper: ClosedOpenRange에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
//
// 반환 오류는 입력 검증 실패와 패키지에서 정의한 sentinel error/typed error를 그대로 드러낸다.
func ClosedOpenRange[T cmp.Ordered](lower, upper T) (Range[T], error) {
	return newRange(lower, upper, true, false)
}

// OpenClosedRange 상한만 포함하는 range를 만든다.
//
// 매개변수:
//   - lower: OpenClosedRange에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
//   - upper: OpenClosedRange에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
//
// 반환 오류는 입력 검증 실패와 패키지에서 정의한 sentinel error/typed error를 그대로 드러낸다.
func OpenClosedRange[T cmp.Ordered](lower, upper T) (Range[T], error) {
	return newRange(lower, upper, false, true)
}

// OpenOpenRange 양쪽 경계를 제외하는 range를 만든다.
//
// 매개변수:
//   - lower: OpenOpenRange에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
//   - upper: OpenOpenRange에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
//
// 반환 오류는 입력 검증 실패와 패키지에서 정의한 sentinel error/typed error를 그대로 드러낸다.
func OpenOpenRange[T cmp.Ordered](lower, upper T) (Range[T], error) {
	return newRange(lower, upper, false, false)
}

func newRange[T cmp.Ordered](lower, upper T, lowerInclusive, upperInclusive bool) (Range[T], error) {
	var zero Range[T]
	if isOrderedNaN(lower) || isOrderedNaN(upper) {
		return zero, fmt.Errorf("%w: range bounds must not be NaN", ErrInvalidArgument)
	}
	if lower > upper {
		return zero, fmt.Errorf("%w: invalid range: lower %v must be <= upper %v", ErrInvalidArgument, lower, upper)
	}
	if lower == upper && (!lowerInclusive || !upperInclusive) {
		return zero, fmt.Errorf("%w: invalid empty range: equal bounds require closed endpoints", ErrInvalidArgument)
	}
	return Range[T]{
		lower:          lower,
		upper:          upper,
		lowerInclusive: lowerInclusive,
		upperInclusive: upperInclusive,
	}, nil
}

func isOrderedNaN[T cmp.Ordered](value T) bool {
	switch v := any(value).(type) {
	case float32:
		return math.IsNaN(float64(v))
	case float64:
		return math.IsNaN(v)
	default:
		return false
	}
}

// Lower range 경계값과 포함 여부를 반환한다.
func (r Range[T]) Lower() T {
	return r.lower
}

// Upper range 경계값과 포함 여부를 반환한다.
func (r Range[T]) Upper() T {
	return r.upper
}

// LowerInclusive range 경계값과 포함 여부를 반환한다.
func (r Range[T]) LowerInclusive() bool {
	return r.lowerInclusive
}

// UpperInclusive range 경계값과 포함 여부를 반환한다.
func (r Range[T]) UpperInclusive() bool {
	return r.upperInclusive
}

// Contains 값이 포함되어 있는지 반환한다.
//
// 매개변수:
//   - value: Contains에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
func (r Range[T]) Contains(value T) bool {
	if r.Empty() {
		return false
	}
	if isOrderedNaN(value) {
		return false
	}
	if value < r.lower || value > r.upper {
		return false
	}
	if value == r.lower && !r.lowerInclusive {
		return false
	}
	if value == r.upper && !r.upperInclusive {
		return false
	}
	return true
}

// ContainsRange 다른 range를 완전히 포함하는지 반환한다.
//
// 매개변수:
//   - other: ContainsRange에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
func (r Range[T]) ContainsRange(other Range[T]) bool {
	if r.Empty() || other.Empty() {
		return false
	}
	if other.lower < r.lower || other.upper > r.upper {
		return false
	}
	if other.lower == r.lower && other.lowerInclusive && !r.lowerInclusive {
		return false
	}
	if other.upper == r.upper && other.upperInclusive && !r.upperInclusive {
		return false
	}
	return true
}

// Overlaps 두 range가 겹치는지 반환한다.
//
// 매개변수:
//   - other: Overlaps에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
func (r Range[T]) Overlaps(other Range[T]) bool {
	if r.Empty() || other.Empty() {
		return false
	}
	if r.upper < other.lower {
		return false
	}
	if r.upper == other.lower {
		return r.upperInclusive && other.lowerInclusive
	}
	if other.upper < r.lower {
		return false
	}
	if other.upper == r.lower {
		return other.upperInclusive && r.lowerInclusive
	}
	return true
}

// Empty 저장된 항목이 없는지 반환한다.
func (r Range[T]) Empty() bool {
	if r.lower > r.upper {
		return true
	}
	if r.lower == r.upper {
		return !r.lowerInclusive || !r.upperInclusive
	}
	return false
}

// String 값을 사람이 읽을 수 있는 문자열로 반환한다.
func (r Range[T]) String() string {
	left := "("
	if r.lowerInclusive {
		left = "["
	}
	right := ")"
	if r.upperInclusive {
		right = "]"
	}
	return fmt.Sprintf("%s%v,%v%s", left, r.lower, r.upper, right)
}
