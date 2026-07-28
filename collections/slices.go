package collections

import "fmt"

// Indexed 패키지에서 공개하는 구조체다.
type Indexed[T any] struct {
	Index int
	Value T
}

// Chunk 값 목록을 지정한 크기의 묶음으로 나눈다.
//
// 매개변수:
//   - values: 처리할 값 목록이다. nil과 빈 슬라이스는 함수별 입력 규칙에 따라 빈 입력으로 다룬다.
//   - size: Chunk에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
//
// 반환 오류는 입력 검증 실패와 패키지에서 정의한 sentinel error/typed error를 그대로 드러낸다.
func Chunk[T any](values []T, size int) ([][]T, error) {
	if size <= 0 {
		return nil, fmt.Errorf("%w: chunk size[%d] must be positive", ErrInvalidArgument, size)
	}
	if values == nil {
		return nil, nil
	}
	if len(values) == 0 {
		return [][]T{}, nil
	}

	chunks := make([][]T, 0, (len(values)+size-1)/size)
	for start := 0; start < len(values); start += size {
		end := start + size
		if end > len(values) {
			end = len(values)
		}
		chunks = append(chunks, values[start:end])
	}
	return chunks, nil
}

// Sliding 값 목록을 sliding window로 나눈다.
//
// 매개변수:
//   - values: 처리할 값 목록이다. nil과 빈 슬라이스는 함수별 입력 규칙에 따라 빈 입력으로 다룬다.
//   - size: Sliding에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
//   - partialWindows: Sliding에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
//
// 반환 오류는 입력 검증 실패와 패키지에서 정의한 sentinel error/typed error를 그대로 드러낸다.
func Sliding[T any](values []T, size int, partialWindows bool) ([][]T, error) {
	if size <= 0 {
		return nil, fmt.Errorf("%w: sliding size[%d] must be positive", ErrInvalidArgument, size)
	}
	if values == nil {
		return nil, nil
	}
	if len(values) == 0 {
		return [][]T{}, nil
	}

	windows := make([][]T, 0, len(values))
	for start := range values {
		end := start + size
		if end > len(values) {
			if !partialWindows {
				break
			}
			end = len(values)
		}
		windows = append(windows, values[start:end])
	}
	return windows, nil
}

// ChunkBy startsNew 함수가 true를 반환하는 지점마다 새 묶음을 시작한다.
//
// 매개변수:
//   - values: 처리할 값 목록이다. nil과 빈 슬라이스는 함수별 입력 규칙에 따라 빈 입력으로 다룬다.
//   - startsNew: ChunkBy에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
//
// 반환 오류는 입력 검증 실패와 패키지에서 정의한 sentinel error/typed error를 그대로 드러낸다.
func ChunkBy[T any](values []T, startsNew func(T) bool) ([][]T, error) {
	if startsNew == nil {
		return nil, fmt.Errorf("%w: startsNew must not be nil", ErrInvalidArgument)
	}
	if values == nil {
		return nil, nil
	}
	if len(values) == 0 {
		return [][]T{}, nil
	}

	chunks := make([][]T, 0)
	start := 0
	for index := 1; index < len(values); index++ {
		if startsNew(values[index]) {
			chunks = append(chunks, values[start:index])
			start = index
		}
	}
	chunks = append(chunks, values[start:])
	return chunks, nil
}

// SafeSubslice 범위를 벗어난 index를 잘라 안전한 subslice를 반환한다.
//
// 매개변수:
//   - values: 처리할 값 목록이다. nil과 빈 슬라이스는 함수별 입력 규칙에 따라 빈 입력으로 다룬다.
//   - from: SafeSubslice에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
//   - to: SafeSubslice에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
func SafeSubslice[T any](values []T, from, to int) []T {
	if values == nil {
		return nil
	}
	if from < 0 {
		from = 0
	}
	if from > len(values) {
		from = len(values)
	}
	if to < from {
		to = from
	}
	if to > len(values) {
		to = len(values)
	}
	return values[from:to]
}

// PadTo 목록 길이가 newSize에 도달할 때까지 item을 채운다.
//
// 매개변수:
//   - values: 처리할 값 목록이다. nil과 빈 슬라이스는 함수별 입력 규칙에 따라 빈 입력으로 다룬다.
//   - newSize: PadTo에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
//   - item: 처리할 단일 항목이다.
//
// 반환 오류는 입력 검증 실패와 패키지에서 정의한 sentinel error/typed error를 그대로 드러낸다.
func PadTo[T any](values []T, newSize int, item T) ([]T, error) {
	if newSize < 0 {
		return nil, fmt.Errorf("%w: pad size[%d] must be non-negative", ErrInvalidArgument, newSize)
	}
	if len(values) >= newSize {
		return values, nil
	}

	padded := make([]T, newSize)
	copy(padded, values)
	for index := len(values); index < newSize; index++ {
		padded[index] = item
	}
	return padded, nil
}

// Distinct 중복 값을 제거한 목록을 반환한다.
//
// 매개변수:
//   - values: 처리할 값 목록이다. nil과 빈 슬라이스는 함수별 입력 규칙에 따라 빈 입력으로 다룬다.
func Distinct[T comparable](values []T) []T {
	if values == nil {
		return nil
	}
	if len(values) == 0 {
		return []T{}
	}

	seen := make(map[T]struct{}, len(values))
	result := make([]T, 0, len(values))
	for _, value := range values {
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

// Count predicate를 만족하는 값의 개수를 반환한다.
//
// 매개변수:
//   - values: 처리할 값 목록이다. nil과 빈 슬라이스는 함수별 입력 규칙에 따라 빈 입력으로 다룬다.
func Count[T comparable](values []T) map[T]int {
	if values == nil {
		return nil
	}
	counts := make(map[T]int, len(values))
	for _, value := range values {
		counts[value]++
	}
	return counts
}

// DistinctBy key 함수 결과가 중복되는 값을 제거한다.
//
// 매개변수:
//   - values: 처리할 값 목록이다. nil과 빈 슬라이스는 함수별 입력 규칙에 따라 빈 입력으로 다룬다.
//   - key: DistinctBy에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
//
// 반환 오류는 입력 검증 실패와 패키지에서 정의한 sentinel error/typed error를 그대로 드러낸다.
func DistinctBy[T any, K comparable](values []T, key func(T) K) ([]T, error) {
	if key == nil {
		return nil, fmt.Errorf("%w: key must not be nil", ErrInvalidArgument)
	}
	if values == nil {
		return nil, nil
	}
	if len(values) == 0 {
		return []T{}, nil
	}

	seen := make(map[K]struct{}, len(values))
	result := make([]T, 0, len(values))
	for _, value := range values {
		k := key(value)
		if _, ok := seen[k]; ok {
			continue
		}
		seen[k] = struct{}{}
		result = append(result, value)
	}
	return result, nil
}

// ZipWithIndex 값 목록에 index를 붙여 반환한다.
//
// 매개변수:
//   - values: 처리할 값 목록이다. nil과 빈 슬라이스는 함수별 입력 규칙에 따라 빈 입력으로 다룬다.
func ZipWithIndex[T any](values []T) []Indexed[T] {
	if values == nil {
		return nil
	}
	indexed := make([]Indexed[T], len(values))
	for index, value := range values {
		indexed[index] = Indexed[T]{
			Index: index,
			Value: value,
		}
	}
	return indexed
}

// MapErr 각 값을 변환하고 첫 오류에서 중단한다.
//
// 매개변수:
//   - values: 처리할 값 목록이다. nil과 빈 슬라이스는 함수별 입력 규칙에 따라 빈 입력으로 다룬다.
//   - mapper: MapErr에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
//
// 반환 오류는 입력 검증 실패와 패키지에서 정의한 sentinel error/typed error를 그대로 드러낸다.
func MapErr[T any, R any](values []T, mapper func(T) (R, error)) ([]R, error) {
	if mapper == nil {
		return nil, fmt.Errorf("%w: mapper must not be nil", ErrInvalidArgument)
	}
	if values == nil {
		return nil, nil
	}

	result := make([]R, 0, len(values))
	for _, value := range values {
		mapped, err := mapper(value)
		if err != nil {
			return nil, err
		}
		result = append(result, mapped)
	}
	return result, nil
}

// ForEachErr 각 값에 action을 적용하고 첫 오류에서 중단한다.
//
// 매개변수:
//   - values: 처리할 값 목록이다. nil과 빈 슬라이스는 함수별 입력 규칙에 따라 빈 입력으로 다룬다.
//   - action: ForEachErr에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
//
// 반환 오류는 입력 검증 실패와 패키지에서 정의한 sentinel error/typed error를 그대로 드러낸다.
func ForEachErr[T any](values []T, action func(T) error) error {
	if action == nil {
		return fmt.Errorf("%w: action must not be nil", ErrInvalidArgument)
	}
	for _, value := range values {
		if err := action(value); err != nil {
			return err
		}
	}
	return nil
}

// FilterErr predicate가 true인 값만 남기고 첫 오류에서 중단한다.
//
// 매개변수:
//   - values: 처리할 값 목록이다. nil과 빈 슬라이스는 함수별 입력 규칙에 따라 빈 입력으로 다룬다.
//   - predicate: FilterErr에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
//
// 반환 오류는 입력 검증 실패와 패키지에서 정의한 sentinel error/typed error를 그대로 드러낸다.
func FilterErr[T any](values []T, predicate func(T) (bool, error)) ([]T, error) {
	if predicate == nil {
		return nil, fmt.Errorf("%w: predicate must not be nil", ErrInvalidArgument)
	}
	if values == nil {
		return nil, nil
	}

	result := make([]T, 0, len(values))
	for _, value := range values {
		keep, err := predicate(value)
		if err != nil {
			return nil, err
		}
		if keep {
			result = append(result, value)
		}
	}
	return result, nil
}

// FilterMap mapper가 반환한 값 중 유효한 값만 모은다.
//
// 매개변수:
//   - values: 처리할 값 목록이다. nil과 빈 슬라이스는 함수별 입력 규칙에 따라 빈 입력으로 다룬다.
//   - mapper: FilterMap에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
//
// 반환 오류는 입력 검증 실패와 패키지에서 정의한 sentinel error/typed error를 그대로 드러낸다.
func FilterMap[T any, R any](values []T, mapper func(T) (R, bool)) ([]R, error) {
	if mapper == nil {
		return nil, fmt.Errorf("%w: mapper must not be nil", ErrInvalidArgument)
	}
	if values == nil {
		return nil, nil
	}

	result := make([]R, 0, len(values))
	for _, value := range values {
		mapped, ok := mapper(value)
		if ok {
			result = append(result, mapped)
		}
	}
	return result, nil
}
