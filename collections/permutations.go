package collections

import "iter"

// Permutations 값 목록의 모든 순열을 반환한다.
//
// 매개변수:
//   - values: 처리할 값 목록이다. nil과 빈 슬라이스는 함수별 입력 규칙에 따라 빈 입력으로 다룬다.
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
