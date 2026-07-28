package core

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

// HasLength 해당 상태가 존재하는지 반환한다.
//
// 매개변수:
//   - value: HasLength가 해석할 문자열이다. 빈 문자열과 공백은 구현 검증을 따른다.
func HasLength(value string) bool {
	return value != ""
}

// NoLength 길이가 없는 값인지 반환한다.
//
// 매개변수:
//   - value: NoLength가 해석할 문자열이다. 빈 문자열과 공백은 구현 검증을 따른다.
func NoLength(value string) bool {
	return value == ""
}

// HasText 해당 상태가 존재하는지 반환한다.
//
// 매개변수:
//   - value: HasText가 해석할 문자열이다. 빈 문자열과 공백은 구현 검증을 따른다.
func HasText(value string) bool {
	return strings.TrimSpace(value) != ""
}

// NoText 표시할 text가 없는 값인지 반환한다.
//
// 매개변수:
//   - value: NoText가 해석할 문자열이다. 빈 문자열과 공백은 구현 검증을 따른다.
func NoText(value string) bool {
	return !HasText(value)
}

// EmptyToDefault 빈 문자열이면 fallback을 반환한다.
//
// 매개변수:
//   - value: EmptyToDefault가 해석할 문자열이다. 빈 문자열과 공백은 구현 검증을 따른다.
//   - fallback: EmptyToDefault가 해석할 문자열이다. 빈 문자열과 공백은 구현 검증을 따른다.
func EmptyToDefault(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

// BlankToDefault 공백 문자열이면 fallback을 반환한다.
//
// 매개변수:
//   - value: BlankToDefault가 해석할 문자열이다. 빈 문자열과 공백은 구현 검증을 따른다.
//   - fallback: BlankToDefault가 해석할 문자열이다. 빈 문자열과 공백은 구현 검증을 따른다.
func BlankToDefault(value, fallback string) string {
	if !HasText(value) {
		return fallback
	}
	return value
}

// EmptyToNil 빈 문자열이면 nil 포인터를 반환한다.
//
// 매개변수:
//   - value: EmptyToNil가 해석할 문자열이다. 빈 문자열과 공백은 구현 검증을 따른다.
func EmptyToNil(value string) *string {
	if NoLength(value) {
		return nil
	}
	return &value
}

// BlankToNil 공백 문자열이면 nil 포인터를 반환한다.
//
// 매개변수:
//   - value: BlankToNil가 해석할 문자열이다. 빈 문자열과 공백은 구현 검증을 따른다.
func BlankToNil(value string) *string {
	if NoText(value) {
		return nil
	}
	return &value
}

// Mask 문자열 일부를 mask 문자로 가린다.
//
// 매개변수:
//   - value: Mask가 해석할 문자열이다. 빈 문자열과 공백은 구현 검증을 따른다.
//   - mask: Mask에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
func Mask(value string, mask rune) string {
	if value == "" {
		return ""
	}
	var builder strings.Builder
	for range value {
		builder.WriteRune(mask)
	}
	return builder.String()
}

// CommonPrefix 두 문자열의 공통 prefix를 반환한다.
//
// 매개변수:
//   - a: CommonPrefix가 해석할 문자열이다. 빈 문자열과 공백은 구현 검증을 따른다.
//   - b: CommonPrefix가 해석할 문자열이다. 빈 문자열과 공백은 구현 검증을 따른다.
func CommonPrefix(a, b string) string {
	if a == "" || b == "" {
		return ""
	}
	if a == b {
		return a
	}
	ar := []rune(a)
	br := []rune(b)
	limit := min(len(ar), len(br))
	for i := 0; i < limit; i++ {
		if ar[i] != br[i] {
			return string(ar[:i])
		}
	}
	return string(ar[:limit])
}

// CommonSuffix 두 문자열의 공통 suffix를 반환한다.
//
// 매개변수:
//   - a: CommonSuffix가 해석할 문자열이다. 빈 문자열과 공백은 구현 검증을 따른다.
//   - b: CommonSuffix가 해석할 문자열이다. 빈 문자열과 공백은 구현 검증을 따른다.
func CommonSuffix(a, b string) string {
	if a == "" || b == "" {
		return ""
	}
	if a == b {
		return a
	}
	ar := []rune(a)
	br := []rune(b)
	limit := min(len(ar), len(br))
	for i := 0; i < limit; i++ {
		if ar[len(ar)-i-1] != br[len(br)-i-1] {
			return string(ar[len(ar)-i:])
		}
	}
	return string(ar[len(ar)-limit:])
}

// TruncateUTF8Bytes UTF-8 문자열을 byte 길이 한도에 맞춰 자른다.
//
// 매개변수:
//   - value: TruncateUTF8Bytes가 해석할 문자열이다. 빈 문자열과 공백은 구현 검증을 따른다.
//   - maxBytes: TruncateUTF8Bytes에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
//
// 반환 오류는 입력 검증 실패와 패키지에서 정의한 sentinel error/typed error를 그대로 드러낸다.
func TruncateUTF8Bytes(value string, maxBytes int) (string, error) {
	if maxBytes < 0 {
		return "", fmt.Errorf("%w: maxBytes[%d] must be non-negative", ErrInvalidArgument, maxBytes)
	}
	if !utf8.ValidString(value) {
		return "", fmt.Errorf("truncate UTF-8 bytes: %w", ErrInvalidUTF8)
	}
	if len(value) <= maxBytes {
		return value, nil
	}

	for maxBytes > 0 && !utf8.RuneStart(value[maxBytes]) {
		maxBytes--
	}
	return value[:maxBytes], nil
}
