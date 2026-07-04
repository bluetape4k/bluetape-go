package rules

import (
	"sort"
	"strings"
	"sync"
)

// Facts stores rule input and output values by key.
//
// Facts is safe for concurrent access to the container itself. Stored values
// are caller-owned: Clone and Snapshot copy the map, not the values inside it.
type Facts struct {
	mu     sync.RWMutex
	values map[string]any
}

// NewFacts creates an empty facts container.
func NewFacts() *Facts {
	return &Facts{values: make(map[string]any)}
}

// NewFactsFrom creates facts from values.
func NewFactsFrom(values map[string]any) (*Facts, error) {
	facts := NewFacts()
	for key, value := range values {
		if err := facts.Set(key, value); err != nil {
			return nil, err
		}
	}
	return facts, nil
}

// Set stores value under key.
func (f *Facts) Set(key string, value any) error {
	if f == nil {
		return ErrNilFacts
	}
	key = normalizeKey(key)
	if key == "" {
		return ErrBlankKey
	}

	f.mu.Lock()
	defer f.mu.Unlock()
	if f.values == nil {
		f.values = make(map[string]any)
	}
	f.values[key] = value
	return nil
}

// Get returns the value stored under key.
func (f *Facts) Get(key string) (any, bool) {
	if f == nil {
		return nil, false
	}
	key = normalizeKey(key)
	if key == "" {
		return nil, false
	}

	f.mu.RLock()
	defer f.mu.RUnlock()
	value, ok := f.values[key]
	return value, ok
}

// Delete removes key and reports whether a value existed.
func (f *Facts) Delete(key string) bool {
	if f == nil {
		return false
	}
	key = normalizeKey(key)
	if key == "" {
		return false
	}

	f.mu.Lock()
	defer f.mu.Unlock()
	if f.values == nil {
		return false
	}
	_, ok := f.values[key]
	delete(f.values, key)
	return ok
}

// Has reports whether key exists.
func (f *Facts) Has(key string) bool {
	_, ok := f.Get(key)
	return ok
}

// Len returns the number of stored facts.
func (f *Facts) Len() int {
	if f == nil {
		return 0
	}

	f.mu.RLock()
	defer f.mu.RUnlock()
	return len(f.values)
}

// Keys returns stored keys in ascending lexical order.
func (f *Facts) Keys() []string {
	if f == nil {
		return nil
	}

	f.mu.RLock()
	defer f.mu.RUnlock()
	keys := make([]string, 0, len(f.values))
	for key := range f.values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

// Snapshot returns a shallow copy of the stored key/value map.
func (f *Facts) Snapshot() map[string]any {
	if f == nil {
		return nil
	}

	f.mu.RLock()
	defer f.mu.RUnlock()
	copied := make(map[string]any, len(f.values))
	for key, value := range f.values {
		copied[key] = value
	}
	return copied
}

// Clone returns a new Facts container with the same shallow-copied values.
func (f *Facts) Clone() *Facts {
	return &Facts{values: f.Snapshot()}
}

func normalizeKey(key string) string {
	return strings.TrimSpace(key)
}
