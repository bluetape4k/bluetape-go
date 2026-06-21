package core

import "errors"

var (
	// ErrInvalidUTF8 reports text input that is not valid UTF-8.
	ErrInvalidUTF8 = errors.New("invalid UTF-8 text")
)
