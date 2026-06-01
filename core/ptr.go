package core

// Ptr returns a pointer to value.
func Ptr[T any](value T) *T {
	return &value
}

// ValueOr returns the pointed value, or fallback when ptr is nil.
func ValueOr[T any](ptr *T, fallback T) T {
	if ptr == nil {
		return fallback
	}
	return *ptr
}

// ValueOrZero returns the pointed value, or the zero value when ptr is nil.
func ValueOrZero[T any](ptr *T) T {
	var zero T
	return ValueOr(ptr, zero)
}
