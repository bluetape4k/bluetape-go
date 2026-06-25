package collections

import "fmt"

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
