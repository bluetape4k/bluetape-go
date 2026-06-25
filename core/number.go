package core

import (
	"fmt"
	"strings"
)

// Clamp returns value constrained to the inclusive [lower, upper] range.
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

// IsHexDigit reports whether r is an ASCII hexadecimal digit.
func IsHexDigit(r rune) bool {
	return ('0' <= r && r <= '9') || ('a' <= r && r <= 'f') || ('A' <= r && r <= 'F')
}

// IsHexFormat reports whether value uses 0x, 0X, or # hexadecimal notation.
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
