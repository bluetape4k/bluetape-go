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
func TruncateUTF8Bytes(value string, maxBytes int) (string, error) {
	if maxBytes < 0 {
		return "", fmt.Errorf("maxBytes[%d] must be non-negative", maxBytes)
	}
	if len(value) <= maxBytes {
		return value, nil
	}

	for maxBytes > 0 && !utf8.RuneStart(value[maxBytes]) {
		maxBytes--
	}
	return value[:maxBytes], nil
}
