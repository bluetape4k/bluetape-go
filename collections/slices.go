package collections

import "fmt"

// Indexed는 struct 공개 타입이다.
// 값의 zero value, nil 허용 여부, 동시성 소유권은 생성자와 메서드의 한국어 주석 및 테스트 계약을 따른다.
type Indexed[T any] struct {
	Index int
	Value T
}

// Chunk는 Chunk 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - values: Chunk가 읽거나 복사하는 values 목록이다. nil과 빈 슬라이스 의미는 함수 계약을 따른다.
//   - size: Chunk 동작에 필요한 size 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//
// 반환 오류는 입력 검증 실패, 취소, 외부 원인, 또는 패키지 sentinel/typed error 계약을 보존한다.
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

// Sliding는 Sliding 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - values: Sliding가 읽거나 복사하는 values 목록이다. nil과 빈 슬라이스 의미는 함수 계약을 따른다.
//   - size: Sliding 동작에 필요한 size 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//   - partialWindows: Sliding 동작에 필요한 partialWindows 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//
// 반환 오류는 입력 검증 실패, 취소, 외부 원인, 또는 패키지 sentinel/typed error 계약을 보존한다.
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

// ChunkBy는 ChunkBy 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - values: ChunkBy가 읽거나 복사하는 values 목록이다. nil과 빈 슬라이스 의미는 함수 계약을 따른다.
//   - startsNew: ChunkBy 동작에 필요한 startsNew 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//
// 반환 오류는 입력 검증 실패, 취소, 외부 원인, 또는 패키지 sentinel/typed error 계약을 보존한다.
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

// SafeSubslice는 SafeSubslice 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - values: SafeSubslice가 읽거나 복사하는 values 목록이다. nil과 빈 슬라이스 의미는 함수 계약을 따른다.
//   - from: SafeSubslice 동작에 필요한 from 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//   - to: SafeSubslice 동작에 필요한 to 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
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

// PadTo는 PadTo 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - values: PadTo가 읽거나 복사하는 values 목록이다. nil과 빈 슬라이스 의미는 함수 계약을 따른다.
//   - newSize: PadTo 동작에 필요한 newSize 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//   - item: PadTo 동작에 필요한 item 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//
// 반환 오류는 입력 검증 실패, 취소, 외부 원인, 또는 패키지 sentinel/typed error 계약을 보존한다.
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

// Distinct는 Distinct 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - values: Distinct가 읽거나 복사하는 values 목록이다. nil과 빈 슬라이스 의미는 함수 계약을 따른다.
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

// Count는 Count 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - values: Count가 읽거나 복사하는 values 목록이다. nil과 빈 슬라이스 의미는 함수 계약을 따른다.
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

// DistinctBy는 DistinctBy 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - values: DistinctBy가 읽거나 복사하는 values 목록이다. nil과 빈 슬라이스 의미는 함수 계약을 따른다.
//   - key: DistinctBy 동작에 필요한 key 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//
// 반환 오류는 입력 검증 실패, 취소, 외부 원인, 또는 패키지 sentinel/typed error 계약을 보존한다.
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

// ZipWithIndex는 ZipWithIndex 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - values: ZipWithIndex가 읽거나 복사하는 values 목록이다. nil과 빈 슬라이스 의미는 함수 계약을 따른다.
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

// MapErr는 MapErr 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - values: MapErr가 읽거나 복사하는 values 목록이다. nil과 빈 슬라이스 의미는 함수 계약을 따른다.
//   - mapper: MapErr 동작에 필요한 mapper 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//
// 반환 오류는 입력 검증 실패, 취소, 외부 원인, 또는 패키지 sentinel/typed error 계약을 보존한다.
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

// ForEachErr는 ForEachErr 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - values: ForEachErr가 읽거나 복사하는 values 목록이다. nil과 빈 슬라이스 의미는 함수 계약을 따른다.
//   - action: ForEachErr 동작에 필요한 action 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//
// 반환 오류는 입력 검증 실패, 취소, 외부 원인, 또는 패키지 sentinel/typed error 계약을 보존한다.
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

// FilterErr는 FilterErr 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - values: FilterErr가 읽거나 복사하는 values 목록이다. nil과 빈 슬라이스 의미는 함수 계약을 따른다.
//   - predicate: FilterErr 동작에 필요한 predicate 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//
// 반환 오류는 입력 검증 실패, 취소, 외부 원인, 또는 패키지 sentinel/typed error 계약을 보존한다.
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

// FilterMap는 FilterMap 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - values: FilterMap가 읽거나 복사하는 values 목록이다. nil과 빈 슬라이스 의미는 함수 계약을 따른다.
//   - mapper: FilterMap 동작에 필요한 mapper 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//
// 반환 오류는 입력 검증 실패, 취소, 외부 원인, 또는 패키지 sentinel/typed error 계약을 보존한다.
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
