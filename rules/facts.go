package rules

import (
	"sort"
	"strings"
	"sync"
)

// Facts struct 공개 타입이다.
// 값의 zero value, nil 허용 여부, 동시성 소유권은 생성자와 메서드의 한국어 주석 및 테스트 계약을 따른다.
type Facts struct {
	mu     sync.RWMutex
	values map[string]any
}

// NewFacts NewFacts 공개 API의 동작을 수행한다.
func NewFacts() *Facts {
	return &Facts{values: make(map[string]any)}
}

// NewFactsFrom NewFactsFrom 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - values: NewFactsFrom 동작에 필요한 values 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//
// 반환 오류는 입력 검증 실패, 취소, 외부 원인, 또는 패키지 sentinel/typed error 계약을 보존한다.
func NewFactsFrom(values map[string]any) (*Facts, error) {
	facts := NewFacts()
	for key, value := range values {
		if err := facts.Set(key, value); err != nil {
			return nil, err
		}
	}
	return facts, nil
}

// Set Set 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - key: Set가 해석하거나 검증하는 문자열 값이다. 빈 문자열과 공백 처리 의미는 함수 계약을 따른다.
//   - value: Set 동작에 필요한 value 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//
// 반환 오류는 입력 검증 실패, 취소, 외부 원인, 또는 패키지 sentinel/typed error 계약을 보존한다.
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

// Get Get 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - key: Get가 해석하거나 검증하는 문자열 값이다. 빈 문자열과 공백 처리 의미는 함수 계약을 따른다.
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

// Delete Delete 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - key: Delete가 해석하거나 검증하는 문자열 값이다. 빈 문자열과 공백 처리 의미는 함수 계약을 따른다.
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

// Has Has 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - key: Has가 해석하거나 검증하는 문자열 값이다. 빈 문자열과 공백 처리 의미는 함수 계약을 따른다.
func (f *Facts) Has(key string) bool {
	_, ok := f.Get(key)
	return ok
}

// Len Len 공개 API의 동작을 수행한다.
func (f *Facts) Len() int {
	if f == nil {
		return 0
	}

	f.mu.RLock()
	defer f.mu.RUnlock()
	return len(f.values)
}

// Keys Keys 공개 API의 동작을 수행한다.
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

// Snapshot Snapshot 공개 API의 동작을 수행한다.
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

// Clone Clone 공개 API의 동작을 수행한다.
func (f *Facts) Clone() *Facts {
	return &Facts{values: f.Snapshot()}
}

func normalizeKey(key string) string {
	return strings.TrimSpace(key)
}
