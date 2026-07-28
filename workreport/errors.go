package workreport

import (
	"errors"
	"fmt"
)

// ErrUnknownFailurePolicy work report 상태, failure policy, child report에서 사용하는 공개 변수 값이다.
// 호출자는 이 식별자를 오류, 상태, 이벤트, 옵션, 또는 기본값 계약을 비교할 때 사용한다.
var ErrUnknownFailurePolicy = errors.New("unknown failure policy")

// FailurePolicyError work report 상태, failure policy, child report에서 사용하는 구조체다.
type FailurePolicyError struct {
	Policy FailurePolicy
}

func (e FailurePolicyError) Error() string {
	return fmt.Sprintf("%v: %d", ErrUnknownFailurePolicy, e.Policy)
}

// Is errors.Is 비교를 지원한다.
//
// 매개변수:
//   - target: 검사하거나 감쌀 오류 값이다.
func (e FailurePolicyError) Is(target error) bool {
	return target == ErrUnknownFailurePolicy
}
