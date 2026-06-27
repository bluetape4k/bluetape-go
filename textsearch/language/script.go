package language

import "unicode"

// ContainsKorean reports whether text contains a Hangul rune.
func ContainsKorean(text string) bool {
	return containsAny(text, unicode.Hangul)
}

// ContainsJapanese reports whether text contains Hiragana or Katakana.
func ContainsJapanese(text string) bool {
	return containsAny(text, unicode.Hiragana, unicode.Katakana)
}

// ContainsChinese reports whether text contains a Han rune.
func ContainsChinese(text string) bool {
	return containsAny(text, unicode.Han)
}

// ContainsThai reports whether text contains a Thai rune.
func ContainsThai(text string) bool {
	return containsAny(text, unicode.Thai)
}

// ContainsLatin reports whether text contains a Latin rune.
func ContainsLatin(text string) bool {
	return containsAny(text, unicode.Latin)
}

func containsAny(text string, tables ...*unicode.RangeTable) bool {
	for _, r := range text {
		for _, table := range tables {
			if unicode.Is(table, r) {
				return true
			}
		}
	}
	return false
}
