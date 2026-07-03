package collections

import "fmt"

// Indexed is a value paired with its 0-based index.
type Indexed[T any] struct {
	Index int
	Value T
}

// Chunk splits values into fixed-size chunks.
//
// The final chunk may be smaller than size. A nil input returns nil; an empty
// non-nil input returns an empty non-nil slice.
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

// Sliding returns one-step windows over values.
//
// When partialWindows is true, trailing partial windows are included. A nil
// input returns nil; an empty non-nil input returns an empty non-nil slice.
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

// ChunkBy splits values whenever startsNew returns true for the next value.
//
// The matching value starts the new chunk. The first value never creates an
// empty leading chunk.
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

// SafeSubslice returns values[from:to] after clamping indexes to valid bounds.
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

// PadTo returns values padded with item until it reaches newSize.
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

// Distinct returns values with duplicate comparable elements removed.
//
// The first occurrence is kept and input order is preserved.
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

// Count returns the number of occurrences for each comparable value.
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

// DistinctBy returns values with duplicate keys removed.
//
// The first occurrence for each key is kept and input order is preserved.
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

// ZipWithIndex returns values paired with their 0-based indexes.
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

// MapErr maps values and stops at the first mapper error.
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

// ForEachErr calls action for each value and stops at the first action error.
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

// FilterErr keeps values whose predicate result is true and stops at the first predicate error.
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

// FilterMap maps values and keeps only mapped results whose ok flag is true.
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
