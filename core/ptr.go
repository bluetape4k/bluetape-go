package core

// Ptr Ptr 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - value: Ptr 동작에 필요한 value 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
func Ptr[T any](value T) *T {
	return &value
}

// ValueOr ValueOr 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - ptr: ValueOr 동작에 필요한 ptr 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//   - fallback: ValueOr 동작에 필요한 fallback 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
func ValueOr[T any](ptr *T, fallback T) T {
	if ptr == nil {
		return fallback
	}
	return *ptr
}

// ValueOrZero ValueOrZero 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - ptr: ValueOrZero 동작에 필요한 ptr 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
func ValueOrZero[T any](ptr *T) T {
	var zero T
	return ValueOr(ptr, zero)
}
