package ratelimit

import (
	"context"
	"time"
)

// Limiter key별 token 소비를 시도한다.
type Limiter interface {
	Allow(ctx context.Context, key string, tokens int64) (Result, error)
}

// Result rate limit 판정 결과다.
type Result struct {
	Allowed    bool
	Requested  int64
	Remaining  int64
	RetryAfter time.Duration
	ResetAfter time.Duration
}
