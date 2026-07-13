package ratelimit

import "errors"

// ErrCommitUnknown indicates that a dispatched debit may have committed.
var ErrCommitUnknown = errors.New("ratelimit: commit outcome unknown")

// OperationError exposes provider-neutral redacted failure diagnostics.
// KeyID is for sampled diagnostic correlation and must not be used as a metric label.
type OperationError interface {
	error
	Family() string
	Operation() string
	KeyID() string
}
