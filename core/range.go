package core

import (
	"cmp"
	"fmt"
	"math"
)

// Range represents an ordered interval with independently open or closed
// bounds.
//
// The zero value is safe to use and behaves as an empty open-open range. Use
// the constructor functions to create non-empty ranges with validated bounds.
type Range[T cmp.Ordered] struct {
	lower          T
	upper          T
	lowerInclusive bool
	upperInclusive bool
}

// ClosedRange returns a range containing both lower and upper.
func ClosedRange[T cmp.Ordered](lower, upper T) (Range[T], error) {
	return newRange(lower, upper, true, true)
}

// ClosedOpenRange returns a range containing lower and excluding upper.
func ClosedOpenRange[T cmp.Ordered](lower, upper T) (Range[T], error) {
	return newRange(lower, upper, true, false)
}

// OpenClosedRange returns a range excluding lower and containing upper.
func OpenClosedRange[T cmp.Ordered](lower, upper T) (Range[T], error) {
	return newRange(lower, upper, false, true)
}

// OpenOpenRange returns a range excluding both lower and upper.
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

// Lower returns the lower endpoint.
func (r Range[T]) Lower() T {
	return r.lower
}

// Upper returns the upper endpoint.
func (r Range[T]) Upper() T {
	return r.upper
}

// LowerInclusive reports whether the lower endpoint is included.
func (r Range[T]) LowerInclusive() bool {
	return r.lowerInclusive
}

// UpperInclusive reports whether the upper endpoint is included.
func (r Range[T]) UpperInclusive() bool {
	return r.upperInclusive
}

// Contains reports whether value is inside the range.
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

// ContainsRange reports whether other is fully inside r.
//
// Empty ranges contain no ranges; this method returns false when either side is
// empty.
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

// Overlaps reports whether r and other share at least one value.
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

// Empty reports whether the range contains no values.
func (r Range[T]) Empty() bool {
	if r.lower > r.upper {
		return true
	}
	if r.lower == r.upper {
		return !r.lowerInclusive || !r.upperInclusive
	}
	return false
}

// String returns mathematical interval notation such as [1,5) or (1,5].
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
