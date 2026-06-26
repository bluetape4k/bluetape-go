package sqlkit

import "errors"

var (
	// ErrInvalidArgument reports invalid caller-provided SQL helper input.
	ErrInvalidArgument = errors.New("sqlkit: invalid argument")

	// ErrNoRows reports that a one-row query returned no rows.
	ErrNoRows = errors.New("sqlkit: no rows")

	// ErrTooManyRows reports that a one-row query returned more than one row.
	ErrTooManyRows = errors.New("sqlkit: too many rows")
)
