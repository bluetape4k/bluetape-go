package textsearch

import "strings"

// Replacer는 textsearch language image example에서 반환값과 오류 의미를 설명한다.
type Replacer func(Match) string

// Replace는 textsearch language image example에서 반환값과 오류 의미를 설명한다.
// 세부 조건은 language script, tokenizer, normalization, example ownership 계약을 따른다.
// 이 주석은 textsearch language image example의 backend 요구사항, cancellation, timeout, 오류 처리 세부사항을 설명한다.
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

// Mask는 textsearch language image example에서 반환값과 오류 의미를 설명한다.
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
