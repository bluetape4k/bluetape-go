package redislock

import "errors"

// ErrNotAcquired indicates that another owner currently holds the lock.
var ErrNotAcquired = errors.New("redis fenced lock not acquired")
