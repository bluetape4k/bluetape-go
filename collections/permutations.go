package collections

import "iter"

// Permutations returns a lazy sequence of positional permutations.
//
// The input is copied when Permutations is called, and each yielded permutation
// is a fresh shallow snapshot. The number of possible results grows
// factorially; callers should stop iteration early for large inputs.
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
