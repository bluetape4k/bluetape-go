package textsearch

import "errors"

var (
	// ErrEmptyPattern reports an empty pattern in the dictionary.
	ErrEmptyPattern = errors.New("textsearch: pattern must not be empty")
	// ErrNoPatterns reports an empty dictionary.
	ErrNoPatterns = errors.New("textsearch: at least one pattern is required")
)
