package redissem

import "errors"

// ErrNotAcquired indicates that all semaphore permits are currently held.
var ErrNotAcquired = errors.New("redis semaphore permit not acquired")
