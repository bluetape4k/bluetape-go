package core

import (
	"fmt"
	"regexp"

	googleuuid "github.com/google/uuid"
)

// ZeroUUID is the all-zero UUID text value.
const ZeroUUID = "00000000-0000-0000-0000-000000000000"

var uuidTextPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

// IsUUID reports whether value is hyphenated UUID text.
func IsUUID(value string) bool {
	if !uuidTextPattern.MatchString(value) {
		return false
	}
	_, err := googleuuid.Parse(value)
	return err == nil
}

// CanonicalUUID validates value as UUID text and returns lowercase canonical text.
func CanonicalUUID(value string) (string, error) {
	if !IsUUID(value) {
		return "", fmt.Errorf("%w: UUID[%q] must be hyphenated UUID text", ErrInvalidArgument, value)
	}
	parsed, err := googleuuid.Parse(value)
	if err != nil {
		return "", fmt.Errorf("%w: UUID[%q] parse failed: %w", ErrInvalidArgument, value, err)
	}
	return parsed.String(), nil
}

// RequireUUID returns an error when value is not hyphenated UUID text.
func RequireUUID(name, value string) error {
	if !IsUUID(value) {
		return fmt.Errorf("%w: %s[%q] must be hyphenated UUID text", ErrInvalidArgument, name, value)
	}
	return nil
}

// IsZeroUUID reports whether value is valid UUID text equal to ZeroUUID.
func IsZeroUUID(value string) bool {
	canonical, err := CanonicalUUID(value)
	return err == nil && canonical == ZeroUUID
}
