package language

import "unicode"

// ContainsKorean는 textsearch language image example에서 반환값과 오류 의미를 설명한다.
func ContainsKorean(text string) bool {
	return containsAny(text, unicode.Hangul)
}

// ContainsJapanese는 textsearch language image example에서 반환값과 오류 의미를 설명한다.
func ContainsJapanese(text string) bool {
	return containsAny(text, unicode.Hiragana, unicode.Katakana)
}

// ContainsChinese는 textsearch language image example에서 반환값과 오류 의미를 설명한다.
func ContainsChinese(text string) bool {
	return containsAny(text, unicode.Han)
}

// ContainsThai는 textsearch language image example에서 반환값과 오류 의미를 설명한다.
func ContainsThai(text string) bool {
	return containsAny(text, unicode.Thai)
}

// ContainsLatin는 textsearch language image example에서 반환값과 오류 의미를 설명한다.
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
