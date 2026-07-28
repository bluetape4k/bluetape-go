package core

// Zero 타입 T의 zero value를 반환한다.
func Zero[T any]() T {
	var zero T
	return zero
}

// IsZero 값이 zero value인지 반환한다.
//
// 매개변수:
//   - value: IsZero에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
func IsZero[T comparable](value T) bool {
	var zero T
	return value == zero
}

// DefaultIfZero value가 zero value이면 fallback을 반환한다.
//
// 매개변수:
//   - value: DefaultIfZero에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
//   - fallback: DefaultIfZero에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
func DefaultIfZero[T comparable](value, fallback T) T {
	if IsZero(value) {
		return fallback
	}
	return value
}

// IfZeroOrDefault value가 zero value이면 fallback을 반환한다.
//
// 매개변수:
//   - value: IfZeroOrDefault에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
//   - fallback: IfZeroOrDefault에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
func IfZeroOrDefault[T comparable](value T, fallback T) T {
	if IsZero(value) {
		return fallback
	}
	return value
}

// FirstNonZero 처음 만나는 non-zero 값을 반환한다.
//
// 매개변수:
//   - values: FirstNonZero에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
func FirstNonZero[T comparable](values ...T) T {
	for _, value := range values {
		if !IsZero(value) {
			return value
		}
	}
	var zero T
	return zero
}
