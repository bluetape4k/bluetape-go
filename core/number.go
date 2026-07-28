package core

import (
	"fmt"
	"strings"
)

// Clamp는 Clamp 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - value: Clamp 동작에 필요한 value 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//   - lower: Clamp 동작에 필요한 lower 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//   - upper: Clamp 동작에 필요한 upper 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//
// 반환 오류는 입력 검증 실패, 취소, 외부 원인, 또는 패키지 sentinel/typed error 계약을 보존한다.
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

// IsHexDigit는 IsHexDigit 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - r: IsHexDigit 동작에 필요한 r 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
func IsHexDigit(r rune) bool {
	return ('0' <= r && r <= '9') || ('a' <= r && r <= 'f') || ('A' <= r && r <= 'F')
}

// IsHexFormat는 IsHexFormat 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - value: IsHexFormat가 해석하거나 검증하는 문자열 값이다. 빈 문자열과 공백 처리 의미는 함수 계약을 따른다.
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
