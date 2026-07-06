package rules

import (
	"sort"
	"sync"
)

// RuleSet stores rules by name and returns them in deterministic order.
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

// NewRuleSet creates an empty rule set.
func NewRuleSet(rules ...Rule) (*RuleSet, error) {
	set := &RuleSet{byName: make(map[string]ruleEntry)}
	for _, rule := range rules {
		if err := set.Add(rule); err != nil {
			return nil, err
		}
	}
	return set, nil
}

// Add registers rule and rejects duplicate names.
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

// Remove deletes a rule by name and reports whether it existed.
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

// Get returns a rule by name.
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

// Len returns the number of registered rules.
func (s *RuleSet) Len() int {
	if s == nil {
		return 0
	}

	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.byName)
}

// Rules returns rules ordered by priority, name, and registration sequence.
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
