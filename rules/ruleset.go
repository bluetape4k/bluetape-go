package rules

import (
	"sort"
	"sync"
)

// RuleSet struct 공개 타입이다.
// 값의 zero value, nil 허용 여부, 동시성 소유권은 생성자와 메서드의 한국어 주석 및 테스트 계약을 따른다.
type RuleSet struct {
	mu      sync.RWMutex
	nextSeq uint64
	byName  map[string]ruleEntry
}

type ruleEntry struct {
	rule     Rule
	name     string
	priority int
	seq      uint64
}

// NewRuleSet NewRuleSet 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - rules: NewRuleSet 동작에 필요한 rules 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//
// 반환 오류는 입력 검증 실패, 취소, 외부 원인, 또는 패키지 sentinel/typed error 계약을 보존한다.
func NewRuleSet(rules ...Rule) (*RuleSet, error) {
	set := &RuleSet{byName: make(map[string]ruleEntry)}
	for _, rule := range rules {
		if err := set.Add(rule); err != nil {
			return nil, err
		}
	}
	return set, nil
}

// Add Add 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - rule: Add 동작에 필요한 rule 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//
// 반환 오류는 입력 검증 실패, 취소, 외부 원인, 또는 패키지 sentinel/typed error 계약을 보존한다.
func (s *RuleSet) Add(rule Rule) error {
	if s == nil {
		return ErrNilRuleSet
	}
	if rule == nil {
		return ErrNilRule
	}
	name := normalizeKey(rule.Name())
	if name == "" {
		return ErrBlankKey
	}
	priority := rule.Priority()

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.byName == nil {
		s.byName = make(map[string]ruleEntry)
	}
	if _, exists := s.byName[name]; exists {
		return ErrDuplicateRule
	}
	s.byName[name] = ruleEntry{rule: rule, name: name, priority: priority, seq: s.nextSeq}
	s.nextSeq++
	return nil
}

// Remove Remove 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - name: Remove가 해석하거나 검증하는 문자열 값이다. 빈 문자열과 공백 처리 의미는 함수 계약을 따른다.
func (s *RuleSet) Remove(name string) bool {
	if s == nil {
		return false
	}
	name = normalizeKey(name)
	if name == "" {
		return false
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	_, ok := s.byName[name]
	delete(s.byName, name)
	return ok
}

// Get Get 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - name: Get가 해석하거나 검증하는 문자열 값이다. 빈 문자열과 공백 처리 의미는 함수 계약을 따른다.
func (s *RuleSet) Get(name string) (Rule, bool) {
	if s == nil {
		return nil, false
	}
	name = normalizeKey(name)
	if name == "" {
		return nil, false
	}

	s.mu.RLock()
	defer s.mu.RUnlock()
	entry, ok := s.byName[name]
	return entry.rule, ok
}

// Len Len 공개 API의 동작을 수행한다.
func (s *RuleSet) Len() int {
	if s == nil {
		return 0
	}

	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.byName)
}

// Rules Rules 공개 API의 동작을 수행한다.
func (s *RuleSet) Rules() []Rule {
	entries := s.entries()
	rules := make([]Rule, 0, len(entries))
	for _, entry := range entries {
		rules = append(rules, entry.rule)
	}
	return rules
}

func (s *RuleSet) entries() []ruleEntry {
	if s == nil {
		return nil
	}

	s.mu.RLock()
	entries := make([]ruleEntry, 0, len(s.byName))
	for _, entry := range s.byName {
		entries = append(entries, entry)
	}
	s.mu.RUnlock()

	sort.SliceStable(entries, func(i, j int) bool {
		left := entries[i]
		right := entries[j]
		if left.priority != right.priority {
			return left.priority < right.priority
		}
		if left.name != right.name {
			return left.name < right.name
		}
		return left.seq < right.seq
	})
	return entries
}
