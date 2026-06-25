package core

import (
	"cmp"
	"fmt"
	"strings"
)

// Number is the set of built-in integer and floating-point types.
type Number interface {
	~int | ~int8 | ~int16 | ~int32 | ~int64 |
		~uint | ~uint8 | ~uint16 | ~uint32 | ~uint64 | ~uintptr |
		~float32 | ~float64
}

// RequireNotBlank returns an error when value is empty or only whitespace.
func RequireNotBlank(name, value string) error {
	if strings.TrimSpace(value) == "" {
		return fmt.Errorf("%w: %s must not be blank", ErrInvalidArgument, name)
	}
	return nil
}

// RequireNotEmpty returns an error when value is empty.
func RequireNotEmpty(name, value string) error {
	if value == "" {
		return fmt.Errorf("%w: %s must not be empty", ErrInvalidArgument, name)
	}
	return nil
}

// RequireInRange returns an error when value is outside the inclusive range.
func RequireInRange[T cmp.Ordered](name string, value, lower, upper T) error {
	if lower > upper {
		return fmt.Errorf("%w: %s range is invalid: lower %v must be <= upper %v", ErrInvalidArgument, name, lower, upper)
	}
	if value < lower || value > upper {
		return fmt.Errorf("%w: %s[%v] must be in range [%v, %v]", ErrInvalidArgument, name, value, lower, upper)
	}
	return nil
}

// RequireInOpenRange returns an error when value is outside the half-open range [lower, upper).
func RequireInOpenRange[T cmp.Ordered](name string, value, lower, upper T) error {
	if lower >= upper {
		return fmt.Errorf("%w: %s range is invalid: lower %v must be < upper %v", ErrInvalidArgument, name, lower, upper)
	}
	if value < lower || value >= upper {
		return fmt.Errorf("%w: %s[%v] must be in range [%v, %v)", ErrInvalidArgument, name, value, lower, upper)
	}
	return nil
}

// RequirePositive returns an error when value is less than or equal to zero.
func RequirePositive[T Number](name string, value T) error {
	var zero T
	if value <= zero {
		return fmt.Errorf("%w: %s[%v] must be positive", ErrInvalidArgument, name, value)
	}
	return nil
}

// RequireNonNegative returns an error when value is less than zero.
func RequireNonNegative[T Number](name string, value T) error {
	var zero T
	if value < zero {
		return fmt.Errorf("%w: %s[%v] must be non-negative", ErrInvalidArgument, name, value)
	}
	return nil
}
