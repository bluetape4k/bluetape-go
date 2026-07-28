package rules

import (
	"sort"
	"sync"
)

// RuleSet 패키지에서 공개하는 구조체다.
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

// NewRuleSet RuleSet 인스턴스를 생성한다.
//
// 매개변수:
//   - rules: NewRuleSet에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
//
// 반환 오류는 입력 검증 실패와 패키지에서 정의한 sentinel error/typed error를 그대로 드러낸다.
func NewRuleSet(rules ...Rule) (*RuleSet, error) {
	set := &RuleSet{byName: make(map[string]ruleEntry)}
	for _, rule := range rules {
		if err := set.Add(rule); err != nil {
			return nil, err
		}
	}
	return set, nil
}

// Add 현재 값에 입력 값을 더한 결과를 반환한다.
//
// 매개변수:
//   - rule: Add에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
//
// 반환 오류는 입력 검증 실패와 패키지에서 정의한 sentinel error/typed error를 그대로 드러낸다.
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

// Remove key에 해당하는 fact를 제거한다.
//
// 매개변수:
//   - name: Remove가 해석할 문자열이다. 빈 문자열과 공백은 구현 검증을 따른다.
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

// Get key에 해당하는 값을 조회한다.
//
// 매개변수:
//   - name: Get가 해석할 문자열이다. 빈 문자열과 공백은 구현 검증을 따른다.
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

// Len 현재 항목 수를 반환한다.
func (s *RuleSet) Len() int {
	if s == nil {
		return 0
	}

	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.byName)
}

// Rules ruleset에 등록된 rule 목록을 반환한다.
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
