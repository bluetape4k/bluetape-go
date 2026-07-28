package collections

import "fmt"

// GroupBy 값 목록을 key 함수 결과별로 묶는다.
//
// 매개변수:
//   - values: 처리할 값 목록이다. nil과 빈 슬라이스는 함수별 입력 규칙에 따라 빈 입력으로 다룬다.
//   - key: GroupBy에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
//
// 반환 오류는 입력 검증 실패와 패키지에서 정의한 sentinel error/typed error를 그대로 드러낸다.
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

// CountBy 값 목록을 key 함수 결과별로 세어 반환한다.
//
// 매개변수:
//   - values: 처리할 값 목록이다. nil과 빈 슬라이스는 함수별 입력 규칙에 따라 빈 입력으로 다룬다.
//   - key: CountBy에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
//
// 반환 오류는 입력 검증 실패와 패키지에서 정의한 sentinel error/typed error를 그대로 드러낸다.
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
