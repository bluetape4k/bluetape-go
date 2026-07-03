package core

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

// HasLength reports whether value is not empty.
func HasLength(value string) bool {
	return value != ""
}

// NoLength reports whether value is empty.
func NoLength(value string) bool {
	return value == ""
}

// HasText reports whether value contains at least one non-whitespace rune.
func HasText(value string) bool {
	return strings.TrimSpace(value) != ""
}

// NoText reports whether value is empty or only whitespace.
func NoText(value string) bool {
	return !HasText(value)
}

// EmptyToDefault returns fallback when value is empty.
func EmptyToDefault(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

// BlankToDefault returns fallback when value is empty or only whitespace.
func BlankToDefault(value, fallback string) string {
	if !HasText(value) {
		return fallback
	}
	return value
}

// EmptyToNil returns nil when value is empty; otherwise it returns a pointer to value.
func EmptyToNil(value string) *string {
	if NoLength(value) {
		return nil
	}
	return &value
}

// BlankToNil returns nil when value is empty or only whitespace; otherwise it returns a pointer to value.
func BlankToNil(value string) *string {
	if NoText(value) {
		return nil
	}
	return &value
}

// Mask returns value with every rune replaced by mask.
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

// CommonPrefix returns the shared rune prefix of a and b.
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

// CommonSuffix returns the shared rune suffix of a and b.
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

// TruncateUTF8Bytes truncates value to at most maxBytes without splitting a UTF-8 rune.
//
// It returns an error wrapping ErrInvalidUTF8 when value is not valid UTF-8.
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
