package workflow

import "errors"

var (
	// ErrNilWork reports a nil Work passed to a runner.
	ErrNilWork = errors.New("workflow work must not be nil")
	// ErrNilPredicate reports a nil Predicate passed to Conditional.
	ErrNilPredicate = errors.New("workflow predicate must not be nil")
	// ErrTooManyFalseBranches reports more than one false branch for Conditional.
	ErrTooManyFalseBranches = errors.New("workflow conditional accepts at most one false branch")
	// ErrUnknownReportStatus reports a Work result with an unknown status.
	ErrUnknownReportStatus = errors.New("workflow work returned unknown report status")
)
