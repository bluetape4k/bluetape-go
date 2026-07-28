package workreport

// Status는 string 공개 타입이며 work report 상태, failure policy, child report 계약을 보존한다.
// 필드와 zero value, nil 허용 여부, 동시성 소유권은 생성자와 메서드의 한국어 주석 및 테스트 계약을 따른다.
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
