package textsearch

import "strings"

// Replacer returns replacement text for a match.
type Replacer func(Match) string

// Replace returns input with every non-overlapping match replaced. When the
// matcher is configured with OverlapAll, replacement still uses
// leftmost-longest non-overlapping matches so output is deterministic.
func (m *Matcher) Replace(input string, replacer Replacer) string {
	if m == nil || replacer == nil {
		return input
	}
	matches := m.findForReplacement(input)
	if len(matches) == 0 {
		return input
	}

	var builder strings.Builder
	builder.Grow(len(input))
	offset := 0
	for _, match := range matches {
		builder.WriteString(input[offset:match.Start])
		builder.WriteString(replacer(match))
		offset = match.End
	}
	builder.WriteString(input[offset:])
	return builder.String()
}

// Mask returns input with every non-overlapping match replaced by mask.
func (m *Matcher) Mask(input string, mask rune) string {
	return m.Replace(input, func(match Match) string {
		count := 0
		for range match.Text {
			count++
		}
		return strings.Repeat(string(mask), count)
	})
}

func (m *Matcher) findForReplacement(input string) []Match {
	copied := *m
	copied.cfg.Overlap = OverlapLeftmostLongest
	return copied.find(input, false)
}
