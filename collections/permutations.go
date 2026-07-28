package collections

import "iter"

// Permutations Permutations 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - values: Permutations가 읽거나 복사하는 values 목록이다. nil과 빈 슬라이스 의미는 함수 계약을 따른다.
func Permutations[T any](values []T) iter.Seq[[]T] {
	source := copySlice(values)
	return func(yield func([]T) bool) {
		if len(source) == 0 {
			yield(make([]T, 0))
			return
		}

		work := copySlice(source)
		var generate func(int) bool
		generate = func(start int) bool {
			if start == len(work) {
				return yield(copySlice(work))
			}
			for index := start; index < len(work); index++ {
				work[start], work[index] = work[index], work[start]
				if !generate(start + 1) {
					work[start], work[index] = work[index], work[start]
					return false
				}
				work[start], work[index] = work[index], work[start]
			}
			return true
		}
		generate(0)
	}
}

func copySlice[T any](values []T) []T {
	if values == nil {
		return nil
	}
	result := make([]T, len(values))
	copy(result, values)
	return result
}
