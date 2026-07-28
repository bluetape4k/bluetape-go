package ratelimit

import "errors"

// ErrCommitUnknown token bucket, limiter option, HTTP boundary, result quota에서 사용하는 공개 변수 값이다.
// 호출자는 이 식별자를 lock 오류, limiter 옵션, result, 또는 conformance 계약을 비교할 때 사용한다.
var ErrCommitUnknown = errors.New("ratelimit: commit outcome unknown")

// OperationError token bucket, limiter option, HTTP boundary, result quota에서 사용하는 인터페이스이다.
type OperationError interface {
	error
	Family() string
	Operation() string
	KeyID() string
}
