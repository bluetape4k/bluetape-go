package redislock

import "errors"

// ErrNotAcquired 는 다른 owner가 lock을 보유 중일 때 반환된다.
var ErrNotAcquired = errors.New("redis lock not acquired")
