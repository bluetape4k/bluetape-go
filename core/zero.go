package core

// Zero Zero 공개 API의 동작을 수행한다.
func Zero[T any]() T {
	var zero T
	return zero
}

// IsZero IsZero 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - value: IsZero 동작에 필요한 value 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
func IsZero[T comparable](value T) bool {
	var zero T
	return value == zero
}

// DefaultIfZero DefaultIfZero 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - value: DefaultIfZero 동작에 필요한 value 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//   - fallback: DefaultIfZero 동작에 필요한 fallback 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
func DefaultIfZero[T comparable](value, fallback T) T {
	if IsZero(value) {
		return fallback
	}
	return value
}

// IfZeroOrDefault IfZeroOrDefault 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - value: IfZeroOrDefault 동작에 필요한 value 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//   - fallback: IfZeroOrDefault 동작에 필요한 fallback 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
func IfZeroOrDefault[T comparable](value T, fallback T) T {
	if IsZero(value) {
		return fallback
	}
	return value
}

// FirstNonZero FirstNonZero 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - values: FirstNonZero 동작에 필요한 values 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
func FirstNonZero[T comparable](values ...T) T {
	for _, value := range values {
		if !IsZero(value) {
			return value
		}
	}
	var zero T
	return zero
}
