package workreport

import (
	"errors"
	"fmt"
)

// ErrUnknownFailurePolicy는 변수 공개 값이며 work report 상태, failure policy, child report 계약을 보존한다.
// 호출자는 이 식별자를 오류, 상태, 이벤트, 옵션, 또는 기본값 계약을 비교할 때 사용한다.
var ErrUnknownFailurePolicy = errors.New("unknown failure policy")

// FailurePolicyError는 struct 공개 타입이며 work report 상태, failure policy, child report 계약을 보존한다.
// 필드와 zero value, nil 허용 여부, 동시성 소유권은 생성자와 메서드의 한국어 주석 및 테스트 계약을 따른다.
type FailurePolicyError struct {
	Policy FailurePolicy
}

func (e FailurePolicyError) Error() string {
	return fmt.Sprintf("%v: %d", ErrUnknownFailurePolicy, e.Policy)
}

// Is는 Is 공개 API의 동작을 수행하며 work report 상태, failure policy, child report 계약을 보존한다.
//
// 매개변수:
//   - target: 검사하거나 감쌀 오류 값이다.
func (e FailurePolicyError) Is(target error) bool {
	return target == ErrUnknownFailurePolicy
}
