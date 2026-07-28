package collections

import "fmt"

// GroupBy GroupBy 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - values: GroupBy가 읽거나 복사하는 values 목록이다. nil과 빈 슬라이스 의미는 함수 계약을 따른다.
//   - key: GroupBy 동작에 필요한 key 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//
// 반환 오류는 입력 검증 실패, 취소, 외부 원인, 또는 패키지 sentinel/typed error 계약을 보존한다.
func GroupBy[T any, K comparable](values []T, key func(T) K) (map[K][]T, error) {
	if key == nil {
		return nil, fmt.Errorf("%w: key must not be nil", ErrInvalidArgument)
	}
	if values == nil {
		return nil, nil
	}

	groups := make(map[K][]T)
	for _, value := range values {
		k := key(value)
		groups[k] = append(groups[k], value)
	}
	return groups, nil
}

// CountBy CountBy 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - values: CountBy가 읽거나 복사하는 values 목록이다. nil과 빈 슬라이스 의미는 함수 계약을 따른다.
//   - key: CountBy 동작에 필요한 key 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//
// 반환 오류는 입력 검증 실패, 취소, 외부 원인, 또는 패키지 sentinel/typed error 계약을 보존한다.
func CountBy[T any, K comparable](values []T, key func(T) K) (map[K]int, error) {
	if key == nil {
		return nil, fmt.Errorf("%w: key must not be nil", ErrInvalidArgument)
	}
	if values == nil {
		return nil, nil
	}

	counts := make(map[K]int)
	for _, value := range values {
		counts[key(value)]++
	}
	return counts, nil
}
