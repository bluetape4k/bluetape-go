package workreport

// FailurePolicy controls how aggregators treat child failures.
type FailurePolicy int

const (
	// StopOnFailure stops aggregation at the first non-completed child.
	StopOnFailure FailurePolicy = iota
	// ContinueOnFailure aggregates every child and reports partial success when
	// any child is not completed.
	ContinueOnFailure
)

func (p FailurePolicy) valid() bool {
	return p == StopOnFailure || p == ContinueOnFailure
}
