package collections

import "fmt"

// GroupBy groups values by a derived comparable key.
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

// CountBy counts values by a derived comparable key.
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
