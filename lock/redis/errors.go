package redislock

import "errors"

// ErrNotAcquired는 변수 공개 값이며 Redis lock key, owner token, TTL, unlock safety 계약을 보존한다.
// 호출자는 이 식별자를 lock 오류, limiter 옵션, result, 또는 conformance 계약을 비교할 때 사용한다.
var ErrNotAcquired = errors.New("redis lock not acquired")
