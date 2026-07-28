package rules

import (
	"sort"
	"strings"
	"sync"
)

// Facts 패키지에서 공개하는 구조체다.
type Facts struct {
	mu     sync.RWMutex
	values map[string]any
}

// NewFacts Facts 인스턴스를 생성한다.
func NewFacts() *Facts {
	return &Facts{values: make(map[string]any)}
}

// NewFactsFrom FactsFrom 인스턴스를 생성한다.
//
// 매개변수:
//   - values: NewFactsFrom에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
//
// 반환 오류는 입력 검증 실패와 패키지에서 정의한 sentinel error/typed error를 그대로 드러낸다.
func NewFactsFrom(values map[string]any) (*Facts, error) {
	facts := NewFacts()
	for key, value := range values {
		if err := facts.Set(key, value); err != nil {
			return nil, err
		}
	}
	return facts, nil
}

// Set key에 값을 저장한다.
//
// 매개변수:
//   - key: Set가 해석할 문자열이다. 빈 문자열과 공백은 구현 검증을 따른다.
//   - value: Set에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
//
// 반환 오류는 입력 검증 실패와 패키지에서 정의한 sentinel error/typed error를 그대로 드러낸다.
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

// Get key에 해당하는 값을 조회한다.
//
// 매개변수:
//   - key: Get가 해석할 문자열이다. 빈 문자열과 공백은 구현 검증을 따른다.
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

// Delete key에 해당하는 값을 제거한다.
//
// 매개변수:
//   - key: Delete가 해석할 문자열이다. 빈 문자열과 공백은 구현 검증을 따른다.
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

// Has 해당 상태가 존재하는지 반환한다.
//
// 매개변수:
//   - key: Has가 해석할 문자열이다. 빈 문자열과 공백은 구현 검증을 따른다.
func (f *Facts) Has(key string) bool {
	_, ok := f.Get(key)
	return ok
}

// Len 현재 항목 수를 반환한다.
func (f *Facts) Len() int {
	if f == nil {
		return 0
	}

	f.mu.RLock()
	defer f.mu.RUnlock()
	return len(f.values)
}

// Keys 저장된 key 목록을 반환한다.
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

// Snapshot 현재 상태를 복사해 반환한다.
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

// Clone 현재 facts를 복사한다.
func (f *Facts) Clone() *Facts {
	return &Facts{values: f.Snapshot()}
}

func normalizeKey(key string) string {
	return strings.TrimSpace(key)
}
