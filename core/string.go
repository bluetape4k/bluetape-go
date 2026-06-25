package core

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

// HasText reports whether value contains at least one non-whitespace rune.
func HasText(value string) bool {
	return strings.TrimSpace(value) != ""
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
