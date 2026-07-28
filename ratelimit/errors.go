package ratelimit

import "errors"

// ErrCommitUnknown는 변수 공개 값이며 token bucket, limiter option, HTTP boundary, result quota 계약을 보존한다.
// 호출자는 이 식별자를 lock 오류, limiter 옵션, result, 또는 conformance 계약을 비교할 때 사용한다.
var ErrCommitUnknown = errors.New("ratelimit: commit outcome unknown")

// OperationError는 interface 공개 타입이며 token bucket, limiter option, HTTP boundary, result quota 계약을 보존한다.
// 필드와 zero value, nil 허용 여부, 동시성 소유권은 생성자와 메서드의 한국어 주석 및 테스트 계약을 따른다.
type OperationError interface {
	error
	Family() string
	Operation() string
	KeyID() string
}
