package workreport

// FailurePolicy int 공개 타입이며 work report 상태, failure policy, child report 계약을 보존한다.
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
