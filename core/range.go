package core

import (
	"cmp"
	"fmt"
	"math"
)

// Range는 struct 공개 타입이다.
// 값의 zero value, nil 허용 여부, 동시성 소유권은 생성자와 메서드의 한국어 주석 및 테스트 계약을 따른다.
type Range[T cmp.Ordered] struct {
	lower          T
	upper          T
	lowerInclusive bool
	upperInclusive bool
}

// ClosedRange는 ClosedRange 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - lower: ClosedRange 동작에 필요한 lower 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//   - upper: ClosedRange 동작에 필요한 upper 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//
// 반환 오류는 입력 검증 실패, 취소, 외부 원인, 또는 패키지 sentinel/typed error 계약을 보존한다.
func ClosedRange[T cmp.Ordered](lower, upper T) (Range[T], error) {
	return newRange(lower, upper, true, true)
}

// ClosedOpenRange는 ClosedOpenRange 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - lower: ClosedOpenRange 동작에 필요한 lower 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//   - upper: ClosedOpenRange 동작에 필요한 upper 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//
// 반환 오류는 입력 검증 실패, 취소, 외부 원인, 또는 패키지 sentinel/typed error 계약을 보존한다.
func ClosedOpenRange[T cmp.Ordered](lower, upper T) (Range[T], error) {
	return newRange(lower, upper, true, false)
}

// OpenClosedRange는 OpenClosedRange 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - lower: OpenClosedRange 동작에 필요한 lower 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//   - upper: OpenClosedRange 동작에 필요한 upper 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//
// 반환 오류는 입력 검증 실패, 취소, 외부 원인, 또는 패키지 sentinel/typed error 계약을 보존한다.
func OpenClosedRange[T cmp.Ordered](lower, upper T) (Range[T], error) {
	return newRange(lower, upper, false, true)
}

// OpenOpenRange는 OpenOpenRange 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - lower: OpenOpenRange 동작에 필요한 lower 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//   - upper: OpenOpenRange 동작에 필요한 upper 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//
// 반환 오류는 입력 검증 실패, 취소, 외부 원인, 또는 패키지 sentinel/typed error 계약을 보존한다.
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

// Lower는 Lower 공개 API의 동작을 수행한다.
func (r Range[T]) Lower() T {
	return r.lower
}

// Upper는 Upper 공개 API의 동작을 수행한다.
func (r Range[T]) Upper() T {
	return r.upper
}

// LowerInclusive는 LowerInclusive 공개 API의 동작을 수행한다.
func (r Range[T]) LowerInclusive() bool {
	return r.lowerInclusive
}

// UpperInclusive는 UpperInclusive 공개 API의 동작을 수행한다.
func (r Range[T]) UpperInclusive() bool {
	return r.upperInclusive
}

// Contains는 Contains 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - value: Contains 동작에 필요한 value 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
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

// ContainsRange는 ContainsRange 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - other: ContainsRange 동작에 필요한 other 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
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

// Overlaps는 Overlaps 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - other: Overlaps 동작에 필요한 other 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
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

// Empty는 Empty 공개 API의 동작을 수행한다.
func (r Range[T]) Empty() bool {
	if r.lower > r.upper {
		return true
	}
	if r.lower == r.upper {
		return !r.lowerInclusive || !r.upperInclusive
	}
	return false
}

// String는 String 공개 API의 동작을 수행한다.
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
