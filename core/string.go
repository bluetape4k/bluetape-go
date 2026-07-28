package core

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

// HasLength HasLength 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - value: HasLength가 해석하거나 검증하는 문자열 값이다. 빈 문자열과 공백 처리 의미는 함수 계약을 따른다.
func HasLength(value string) bool {
	return value != ""
}

// NoLength NoLength 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - value: NoLength가 해석하거나 검증하는 문자열 값이다. 빈 문자열과 공백 처리 의미는 함수 계약을 따른다.
func NoLength(value string) bool {
	return value == ""
}

// HasText HasText 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - value: HasText가 해석하거나 검증하는 문자열 값이다. 빈 문자열과 공백 처리 의미는 함수 계약을 따른다.
func HasText(value string) bool {
	return strings.TrimSpace(value) != ""
}

// NoText NoText 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - value: NoText가 해석하거나 검증하는 문자열 값이다. 빈 문자열과 공백 처리 의미는 함수 계약을 따른다.
func NoText(value string) bool {
	return !HasText(value)
}

// EmptyToDefault EmptyToDefault 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - value: EmptyToDefault가 해석하거나 검증하는 문자열 값이다. 빈 문자열과 공백 처리 의미는 함수 계약을 따른다.
//   - fallback: EmptyToDefault가 해석하거나 검증하는 문자열 값이다. 빈 문자열과 공백 처리 의미는 함수 계약을 따른다.
func EmptyToDefault(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

// BlankToDefault BlankToDefault 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - value: BlankToDefault가 해석하거나 검증하는 문자열 값이다. 빈 문자열과 공백 처리 의미는 함수 계약을 따른다.
//   - fallback: BlankToDefault가 해석하거나 검증하는 문자열 값이다. 빈 문자열과 공백 처리 의미는 함수 계약을 따른다.
func BlankToDefault(value, fallback string) string {
	if !HasText(value) {
		return fallback
	}
	return value
}

// EmptyToNil EmptyToNil 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - value: EmptyToNil가 해석하거나 검증하는 문자열 값이다. 빈 문자열과 공백 처리 의미는 함수 계약을 따른다.
func EmptyToNil(value string) *string {
	if NoLength(value) {
		return nil
	}
	return &value
}

// BlankToNil BlankToNil 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - value: BlankToNil가 해석하거나 검증하는 문자열 값이다. 빈 문자열과 공백 처리 의미는 함수 계약을 따른다.
func BlankToNil(value string) *string {
	if NoText(value) {
		return nil
	}
	return &value
}

// Mask Mask 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - value: Mask가 해석하거나 검증하는 문자열 값이다. 빈 문자열과 공백 처리 의미는 함수 계약을 따른다.
//   - mask: Mask 동작에 필요한 mask 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
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

// CommonPrefix CommonPrefix 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - a: CommonPrefix가 해석하거나 검증하는 문자열 값이다. 빈 문자열과 공백 처리 의미는 함수 계약을 따른다.
//   - b: CommonPrefix가 해석하거나 검증하는 문자열 값이다. 빈 문자열과 공백 처리 의미는 함수 계약을 따른다.
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

// CommonSuffix CommonSuffix 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - a: CommonSuffix가 해석하거나 검증하는 문자열 값이다. 빈 문자열과 공백 처리 의미는 함수 계약을 따른다.
//   - b: CommonSuffix가 해석하거나 검증하는 문자열 값이다. 빈 문자열과 공백 처리 의미는 함수 계약을 따른다.
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

// TruncateUTF8Bytes TruncateUTF8Bytes 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - value: TruncateUTF8Bytes가 해석하거나 검증하는 문자열 값이다. 빈 문자열과 공백 처리 의미는 함수 계약을 따른다.
//   - maxBytes: TruncateUTF8Bytes 동작에 필요한 maxBytes 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//
// 반환 오류는 입력 검증 실패, 취소, 외부 원인, 또는 패키지 sentinel/typed error 계약을 보존한다.
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
