package core

import "errors"

var (
	// ErrInvalidQuarter reports a quarter value outside Q1 through Q4.
	ErrInvalidQuarter = errors.New("invalid quarter")

	// ErrInvalidTime reports invalid calendar/time helper input.
	ErrInvalidTime = errors.New("invalid time")

	// ErrInvalidUTF8 reports text input that is not valid UTF-8.
	ErrInvalidUTF8 = errors.New("invalid UTF-8 text")
)
