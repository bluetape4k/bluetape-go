package workreport

// FailurePolicy는 int 공개 타입이며 work report 상태, failure policy, child report 계약을 보존한다.
// 필드와 zero value, nil 허용 여부, 동시성 소유권은 생성자와 메서드의 한국어 주석 및 테스트 계약을 따른다.
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
