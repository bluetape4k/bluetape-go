package core

import (
	"fmt"
	"strings"
)

// Clamp 값을 지정한 범위 안으로 제한한다.
//
// 매개변수:
//   - value: Clamp에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
//   - lower: Clamp에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
//   - upper: Clamp에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
//
// 반환 오류는 입력 검증 실패와 패키지에서 정의한 sentinel error/typed error를 그대로 드러낸다.
func Clamp[T Number](value, lower, upper T) (T, error) {
	if lower > upper {
		var zero T
		return zero, fmt.Errorf("%w: invalid range: lower %v must be <= upper %v", ErrInvalidArgument, lower, upper)
	}
	if value < lower {
		return lower, nil
	}
	if value > upper {
		return upper, nil
	}
	return value, nil
}

// IsHexDigit 값이 조건을 만족하는지 반환한다.
//
// 매개변수:
//   - r: IsHexDigit에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
func IsHexDigit(r rune) bool {
	return ('0' <= r && r <= '9') || ('a' <= r && r <= 'f') || ('A' <= r && r <= 'F')
}

// IsHexFormat 값이 조건을 만족하는지 반환한다.
//
// 매개변수:
//   - value: IsHexFormat가 해석할 문자열이다. 빈 문자열과 공백은 구현 검증을 따른다.
func IsHexFormat(value string) bool {
	s := strings.TrimSpace(value)
	s = strings.TrimPrefix(s, "-")
	if s == "" {
		return false
	}

	switch {
	case strings.HasPrefix(s, "0x") || strings.HasPrefix(s, "0X"):
		s = s[2:]
	case strings.HasPrefix(s, "#"):
		s = s[1:]
	default:
		return false
	}

	if s == "" {
		return false
	}
	for _, r := range s {
		if !IsHexDigit(r) {
			return false
		}
	}
	return true
}
