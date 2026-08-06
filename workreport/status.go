package workreport

// Status work report 상태, failure policy, child report에서 사용하는 문자열 타입이다.
type Status string

const (
	// StatusCompleted means work finished without failed children.
	StatusCompleted Status = "completed"
	// StatusFailed means work failed with a caller-visible error.
	StatusFailed Status = "failed"
	// StatusPartial means aggregated work has at least one failed child.
	StatusPartial Status = "partial"
	// StatusAborted means execution stopped for a policy or caller-defined reason.
	StatusAborted Status = "aborted"
	// StatusCancelled means caller cancellation or deadline stopped the work.
	StatusCancelled Status = "cancelled"
)

func (s Status) known() bool {
	switch s {
	case StatusCompleted, StatusFailed, StatusPartial, StatusAborted, StatusCancelled:
		return true
	default:
		return false
	}
}
