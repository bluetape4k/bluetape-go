package language

import "unicode"

// ContainsKorean textsearch language image example에서 반환값과 오류 의미를 설명한다.
func ContainsKorean(text string) bool {
	return containsAny(text, unicode.Hangul)
}

// ContainsJapanese textsearch language image example에서 반환값과 오류 의미를 설명한다.
func ContainsJapanese(text string) bool {
	return containsAny(text, unicode.Hiragana, unicode.Katakana)
}

// ContainsChinese textsearch language image example에서 반환값과 오류 의미를 설명한다.
func ContainsChinese(text string) bool {
	return containsAny(text, unicode.Han)
}

// ContainsThai textsearch language image example에서 반환값과 오류 의미를 설명한다.
func ContainsThai(text string) bool {
	return containsAny(text, unicode.Thai)
}

// ContainsLatin textsearch language image example에서 반환값과 오류 의미를 설명한다.
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
