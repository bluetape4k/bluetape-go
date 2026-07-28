package ratelimit

import (
	"context"
	"time"
)

// Limiter는 interface 공개 타입이며 token bucket, limiter option, HTTP boundary, result quota 계약을 보존한다.
// 필드와 zero value, nil 허용 여부, 동시성 소유권은 생성자와 메서드의 한국어 주석 및 테스트 계약을 따른다.
type Limiter interface {
	Allow(ctx context.Context, key string, tokens int64) (Result, error)
}

// Result는 struct 공개 타입이며 token bucket, limiter option, HTTP boundary, result quota 계약을 보존한다.
// 필드와 zero value, nil 허용 여부, 동시성 소유권은 생성자와 메서드의 한국어 주석 및 테스트 계약을 따른다.
type Result struct {
	Allowed    bool
	Requested  int64
	Remaining  int64
	RetryAfter time.Duration
	ResetAfter time.Duration
}
