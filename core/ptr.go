package core

// Ptr 값을 가리키는 포인터를 반환한다.
//
// 매개변수:
//   - value: Ptr에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
func Ptr[T any](value T) *T {
	return &value
}

// ValueOr 포인터가 nil일 때 fallback 값을 반환한다.
//
// 매개변수:
//   - ptr: ValueOr에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
//   - fallback: ValueOr에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
func ValueOr[T any](ptr *T, fallback T) T {
	if ptr == nil {
		return fallback
	}
	return *ptr
}

// ValueOrZero 포인터가 nil일 때 fallback 값을 반환한다.
//
// 매개변수:
//   - ptr: ValueOrZero에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
func ValueOrZero[T any](ptr *T) T {
	var zero T
	return ValueOr(ptr, zero)
}
