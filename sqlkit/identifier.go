package sqlkit

import (
	"fmt"
	"strings"
)

func quoteIdentifier(identifier string) (string, error) {
	if identifier == "" {
		return "", fmt.Errorf("%w: identifier is empty", ErrInvalidArgument)
	}

	segments := strings.Split(identifier, ".")
	quoted := make([]string, 0, len(segments))
	for _, segment := range segments {
		if !isIdentifierSegment(segment) {
			return "", fmt.Errorf("%w: invalid identifier %q", ErrInvalidArgument, identifier)
		}
		quoted = append(quoted, `"`+segment+`"`)
	}
	return strings.Join(quoted, "."), nil
}

func quoteIdentifiers(identifiers []string) ([]string, error) {
	quoted := make([]string, 0, len(identifiers))
	for _, identifier := range identifiers {
		value, err := quoteIdentifier(identifier)
		if err != nil {
			return nil, err
		}
		quoted = append(quoted, value)
	}
	return quoted, nil
}

func isIdentifierSegment(segment string) bool {
	if segment == "" {
		return false
	}
	for index, r := range segment {
		if index == 0 {
			if !isIdentifierStart(r) {
				return false
			}
			continue
		}
		if !isIdentifierPart(r) {
			return false
		}
	}
	return true
}

func isIdentifierStart(r rune) bool {
	return r == '_' || (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z')
}

func isIdentifierPart(r rune) bool {
	return isIdentifierStart(r) || (r >= '0' && r <= '9')
}
