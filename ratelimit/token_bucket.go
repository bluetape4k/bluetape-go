package ratelimit

import (
	"context"
	"fmt"
	"math"
	"strings"
	"sync"
	"time"
)

type clockFunc func() time.Time

type bucketState struct {
	tokens     float64
	updatedAt  time.Time
	lastSeenAt time.Time
}

// TokenBucket는 struct 공개 타입이며 token bucket, limiter option, HTTP boundary, result quota 계약을 보존한다.
// 필드와 zero value, nil 허용 여부, 동시성 소유권은 생성자와 메서드의 한국어 주석 및 테스트 계약을 따른다.
type TokenBucket struct {
	mu       sync.Mutex
	opts     options
	now      clockFunc
	buckets  map[string]bucketState
	testHook func(context.Context, string, tokenBucketTestPhase) error
}

type tokenBucketTestPhase uint8

const (
	tokenBucketBeforeLinearize tokenBucketTestPhase = iota + 1
	tokenBucketAfterLinearize
)

var _ Limiter = (*TokenBucket)(nil)

// New는 New 공개 API의 동작을 수행하며 token bucket, limiter option, HTTP boundary, result quota 계약을 보존한다.
//
// 매개변수:
//   - options: New 동작에 필요한 options 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//
// 반환 오류는 입력 검증 실패, 취소, Redis/backend 실패, lock ownership 불일치, quota 거절, 또는 package sentinel/typed error 계약을 보존한다.
func New(options Options) (*TokenBucket, error) {
	return newWithClock(options, time.Now)
}

func newWithClock(options Options, now clockFunc) (*TokenBucket, error) {
	normalized, err := options.normalize()
	if err != nil {
		return nil, err
	}
	if now == nil {
		now = time.Now
	}
	return &TokenBucket{
		opts:    normalized,
		now:     now,
		buckets: make(map[string]bucketState),
	}, nil
}

// Allow는 Allow 공개 API의 동작을 수행하며 token bucket, limiter option, HTTP boundary, result quota 계약을 보존한다.
//
// 매개변수:
//   - ctx: 호출자가 소유한 취소, deadline, 요청 범위를 전달한다.
//   - key: 동기화 또는 quota를 식별하는 caller-owned key다. namespace와 normalization 의미는 package 계약을 따른다.
//   - tokens: lock owner 또는 safe unlock 비교에 사용하는 token이다.
//
// 반환 오류는 입력 검증 실패, 취소, Redis/backend 실패, lock ownership 불일치, quota 거절, 또는 package sentinel/typed error 계약을 보존한다.
func (l *TokenBucket) Allow(ctx context.Context, key string, tokens int64) (Result, error) {
	ctx = normalizeContext(ctx)
	if err := ctx.Err(); err != nil {
		return Result{}, err
	}
	if l == nil || l.now == nil || l.opts.ratePerSecond <= 0 {
		return Result{}, fmt.Errorf("rate limiter is not initialized")
	}
	key, err := normalizeKey(key)
	if err != nil {
		return Result{}, err
	}
	if tokens <= 0 {
		return Result{}, fmt.Errorf("tokens must be positive")
	}
	if tokens > l.opts.burst {
		return Result{}, fmt.Errorf("tokens must not exceed burst")
	}

	now := l.now()
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.testHook != nil {
		if err := l.testHook(ctx, key, tokenBucketBeforeLinearize); err != nil {
			return Result{}, err
		}
	}

	l.pruneIdle(now)

	state, ok := l.buckets[key]
	if !ok {
		state = bucketState{tokens: float64(l.opts.burst), updatedAt: now}
	} else {
		state = l.refill(state, now)
	}
	state.lastSeenAt = now

	requested := float64(tokens)
	result := Result{
		Requested: tokens,
		Remaining: wholeTokens(state.tokens),
		ResetAfter: durationUntilFull(
			float64(l.opts.burst),
			state.tokens,
			l.opts.ratePerSecond,
		),
	}

	if state.tokens >= requested {
		state.tokens -= requested
		result.Allowed = true
		result.Remaining = wholeTokens(state.tokens)
		result.RetryAfter = 0
		result.ResetAfter = durationUntilFull(float64(l.opts.burst), state.tokens, l.opts.ratePerSecond)
	} else {
		result.Allowed = false
		result.RetryAfter = durationForTokens(requested-state.tokens, l.opts.ratePerSecond)
	}

	l.buckets[key] = state
	if l.testHook != nil {
		if err := l.testHook(context.Background(), key, tokenBucketAfterLinearize); err != nil {
			return Result{}, err
		}
	}
	return result, nil
}

func (l *TokenBucket) refill(state bucketState, now time.Time) bucketState {
	elapsed := now.Sub(state.updatedAt)
	if elapsed <= 0 {
		return state
	}
	state.tokens = math.Min(float64(l.opts.burst), state.tokens+elapsed.Seconds()*l.opts.ratePerSecond)
	state.updatedAt = now
	return state
}

func (l *TokenBucket) pruneIdle(now time.Time) {
	if l.opts.idleTTL <= 0 {
		return
	}
	for key, state := range l.buckets {
		if now.Sub(state.lastSeenAt) >= l.opts.idleTTL {
			delete(l.buckets, key)
		}
	}
}

func normalizeContext(ctx context.Context) context.Context {
	if ctx == nil {
		return context.Background()
	}
	return ctx
}

func normalizeKey(key string) (string, error) {
	trimmed := strings.TrimSpace(key)
	if trimmed == "" {
		return "", fmt.Errorf("key must not be empty")
	}
	return trimmed, nil
}

func wholeTokens(tokens float64) int64 {
	if tokens <= 0 {
		return 0
	}
	if tokens >= float64(math.MaxInt64) {
		return math.MaxInt64
	}
	return int64(math.Floor(tokens))
}

func durationUntilFull(burst, tokens, ratePerSecond float64) time.Duration {
	return durationForTokens(math.Max(0, burst-tokens), ratePerSecond)
}

func durationForTokens(tokens, ratePerSecond float64) time.Duration {
	if tokens <= 0 || ratePerSecond <= 0 {
		return 0
	}
	seconds := tokens / ratePerSecond
	if seconds >= float64(math.MaxInt64)/float64(time.Second) {
		return time.Duration(math.MaxInt64)
	}
	return time.Duration(math.Ceil(seconds * float64(time.Second)))
}
