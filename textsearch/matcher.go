package textsearch

import (
	"sort"
	"unicode"
)

type compiledPattern struct {
	Pattern
	runes  []rune
	length int
	order  int
}

type output struct {
	pattern int
	length  int
	order   int
}

type node struct {
	next    map[rune]int
	fail    int
	outputs []output
}

// Matcher textsearch language image example에서 caller-visible 상태와 의미를 설명한다.
type Matcher struct {
	cfg      Config
	patterns []compiledPattern
	nodes    []node
}

// Compile textsearch language image example에서 생성과 초기화 계약을 설명한다.
func Compile(patterns []Pattern, cfg Config) (*Matcher, error) {
	if len(patterns) == 0 {
		return nil, ErrNoPatterns
	}

	m := &Matcher{
		cfg:   cfg,
		nodes: []node{{next: make(map[rune]int)}},
	}
	m.patterns = make([]compiledPattern, 0, len(patterns))
	for index, pattern := range patterns {
		compiled, err := normalizedPatternText(pattern, index, cfg)
		if err != nil {
			return nil, err
		}
		m.addPattern(len(m.patterns), compiled)
		m.patterns = append(m.patterns, compiled)
	}
	m.buildFailures()
	return m, nil
}

// CompileStrings textsearch language image example에서 생성과 초기화 계약을 설명한다.
func CompileStrings(patterns []string, cfg Config) (*Matcher, error) {
	entries := make([]Pattern, len(patterns))
	for i, pattern := range patterns {
		entries[i] = Pattern{Text: pattern}
	}
	return Compile(entries, cfg)
}

func (m *Matcher) addPattern(index int, pattern compiledPattern) {
	state := 0
	for _, r := range pattern.runes {
		next, ok := m.nodes[state].next[r]
		if !ok {
			next = len(m.nodes)
			m.nodes[state].next[r] = next
			m.nodes = append(m.nodes, node{next: make(map[rune]int)})
		}
		state = next
	}
	m.nodes[state].outputs = append(m.nodes[state].outputs, output{
		pattern: index,
		length:  pattern.length,
		order:   pattern.order,
	})
}

func (m *Matcher) buildFailures() {
	queue := make([]int, 0, len(m.nodes))
	for _, child := range m.nodes[0].next {
		queue = append(queue, child)
	}

	for head := 0; head < len(queue); head++ {
		current := queue[head]
		for r, child := range m.nodes[current].next {
			queue = append(queue, child)
			failure := m.nodes[current].fail
			for failure != 0 {
				if next, ok := m.nodes[failure].next[r]; ok {
					failure = next
					break
				}
				failure = m.nodes[failure].fail
			}
			if failure == 0 {
				if next, ok := m.nodes[0].next[r]; ok && next != child {
					failure = next
				}
			}
			m.nodes[child].fail = failure
			m.nodes[child].outputs = append(m.nodes[child].outputs, m.nodes[failure].outputs...)
		}
	}
}

// Contains textsearch language image example에서 반환값과 오류 의미를 설명한다.
func (m *Matcher) Contains(input string) bool {
	_, ok := m.First(input)
	return ok
}

// First textsearch language image example에서 반환값과 오류 의미를 설명한다.
// 세부 조건은 language script, tokenizer, normalization, example ownership 계약을 따른다.
func (m *Matcher) First(input string) (Match, bool) {
	matches := m.find(input, true)
	if len(matches) == 0 {
		return Match{}, false
	}
	return matches[0], true
}

// FindAll textsearch language image example에서 반환값과 오류 의미를 설명한다.
func (m *Matcher) FindAll(input string) []Match {
	return m.find(input, false)
}

func (m *Matcher) find(input string, firstOnly bool) []Match {
	if m == nil {
		return nil
	}
	normalized := normalizeString(input, m.cfg)
	if len(normalized.runes) == 0 {
		return nil
	}

	state := 0
	matches := make([]rankedMatch, 0)
	for index, r := range normalized.runes {
		for state != 0 {
			if _, ok := m.nodes[state].next[r]; ok {
				break
			}
			state = m.nodes[state].fail
		}
		if next, ok := m.nodes[state].next[r]; ok {
			state = next
		}
		for _, out := range m.nodes[state].outputs {
			startRune := index - out.length + 1
			if startRune < 0 || !m.acceptBoundary(normalized.runes, startRune, index+1) {
				continue
			}
			match := Match{
				Pattern: m.patterns[out.pattern].Pattern,
				Start:   normalized.starts[startRune],
				End:     normalized.ends[index],
				Text:    input[normalized.starts[startRune]:normalized.ends[index]],
			}
			matches = append(matches, rankedMatch{Match: match, startRune: startRune, endRune: index + 1, order: out.order})
		}
	}
	if len(matches) == 0 {
		return nil
	}
	sortRankedMatches(matches)
	if m.cfg.Overlap == OverlapLeftmostLongest {
		matches = selectLeftmostLongest(matches)
	}
	if firstOnly && len(matches) > 1 {
		matches = matches[:1]
	}
	result := make([]Match, len(matches))
	for i, match := range matches {
		result[i] = match.Match
	}
	return result
}

type rankedMatch struct {
	Match
	startRune int
	endRune   int
	order     int
}

func sortRankedMatches(matches []rankedMatch) {
	sort.SliceStable(matches, func(i, j int) bool {
		left := matches[i]
		right := matches[j]
		if left.Start != right.Start {
			return left.Start < right.Start
		}
		leftLen := left.End - left.Start
		rightLen := right.End - right.Start
		if leftLen != rightLen {
			return leftLen > rightLen
		}
		return left.order < right.order
	})
}

func selectLeftmostLongest(matches []rankedMatch) []rankedMatch {
	selected := make([]rankedMatch, 0, len(matches))
	nextStart := 0
	for _, match := range matches {
		if match.Start < nextStart {
			continue
		}
		selected = append(selected, match)
		nextStart = match.End
	}
	return selected
}

func (m *Matcher) acceptBoundary(runes []rune, start int, end int) bool {
	switch m.cfg.Boundary {
	case BoundaryASCIIWord:
		return isBoundary(runes, start, end, isASCIIWordRune)
	case BoundaryUnicodeWord:
		return isBoundary(runes, start, end, isUnicodeWordRune)
	default:
		return true
	}
}

func isBoundary(runes []rune, start int, end int, word func(rune) bool) bool {
	before := start == 0 || !word(runes[start-1])
	after := end == len(runes) || !word(runes[end])
	return before && after
}

func isASCIIWordRune(r rune) bool {
	return r == '_' || ('0' <= r && r <= '9') || ('A' <= r && r <= 'Z') || ('a' <= r && r <= 'z')
}

func isUnicodeWordRune(r rune) bool {
	return r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r)
}
