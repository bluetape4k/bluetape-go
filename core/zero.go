package core

// Zero returns the zero value for T.
func Zero[T any]() T {
	var zero T
	return zero
}

// IsZero reports whether value equals the zero value for T.
func IsZero[T comparable](value T) bool {
	var zero T
	return value == zero
}

// DefaultIfZero returns fallback when value is the zero value for T.
func DefaultIfZero[T comparable](value, fallback T) T {
	if IsZero(value) {
		return fallback
	}
	return value
}

// IfZeroOrDefault returns fallback when value is the zero value for T.
func IfZeroOrDefault[T comparable](value T, fallback T) T {
	if IsZero(value) {
		return fallback
	}
	return value
}

// FirstNonZero returns the first non-zero value, or the zero value when all values are zero.
func FirstNonZero[T comparable](values ...T) T {
	for _, value := range values {
		if !IsZero(value) {
			return value
		}
	}
	var zero T
	return zero
}
