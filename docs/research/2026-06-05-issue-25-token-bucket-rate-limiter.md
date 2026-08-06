# Issue #25 Token-Bucket Rate Limiter 연구

Issue: #25
Milestone: 0.3.0
Date: 2026-06-05
Work type: Type A - Full Feature

## 연구 질문

`bluetape-go`는 새 dependency를 추가하거나 기존 Redis coordination package 경계를
흐리지 않으면서, local 및 Redis-backed 구현을 갖춘 실용적인 token-bucket rate
limiting을 어떻게 추가해야 하는가?

## 현재 Repository 근거

- `go.mod`는 이미 `github.com/redis/go-redis/v9 v9.20.0`에 의존한다.
- `lock/redis`는 `redis.Cmdable`을 사용하고, command 실행 전에 options를 검증하며,
  owner-token compare-and-delete에 `Eval`을 사용한다.
- `cache/rediscoord`는 cross-process behavior를 local cache contract 안에 숨기지
  않고 명시적인 Redis coordination package를 사용한다.
- `resilience/http.go`는 특정 router framework에 묶이지 않는 standard-library HTTP
  adapter를 노출한다.
- `testing/concurrency`는 #25가 stress/cancellation coverage에 명시적으로 요구한
  `GoroutineStressTester`와 `AsyncJobTester`를 제공한다.

## 외부 근거

| Source | Relevant point | Decision impact |
|---|---|---|
| Redis `EVAL`, https://redis.io/docs/latest/commands/eval/ | Redis runs a Lua script server-side and receives keys through `KEYS` plus arguments through `ARGV`. | Use one script per consume attempt so refill, consume, and expiration are atomic for concurrent clients. |
| Redis `TIME`, https://redis.io/docs/latest/commands/time/ | Redis exposes server time as seconds and microseconds. | Redis limiter should use Redis server time inside the script to avoid client clock skew across processes. |
| Redis `PEXPIRE`, https://redis.io/docs/latest/commands/pexpire/ | Redis can assign millisecond expiration to a key. | Bucket state keys should expire automatically after an idle window. |
| go-redis v9 package docs, https://pkg.go.dev/github.com/redis/go-redis/v9 | `Client` and `Cmdable` expose `Eval(ctx, script, keys, args...)`. | Reuse the existing `redis.Cmdable` boundary and avoid client lifecycle ownership. |
| Go `x/time/rate`, https://pkg.go.dev/golang.org/x/time/rate | The standard ecosystem has a mature local token-bucket limiter API. | Borrow conceptual semantics, but do not add the dependency because the workflow forbids new dependencies without explicit request. |

## Token-Bucket 의미

token bucket은 최대 `Burst` tokens를 보관한다. token은 시간이 지나며
`RatePerSecond` 속도로 refill되고, request는 충분한 token이 있을 때만 `Tokens`
tokens를 소비한다. 따라서 짧은 burst는 `Burst`까지 허용하면서 장기 rate는
`RatePerSecond`로 제한한다.

반환 diagnostics에는 다음이 포함되어야 한다.

- request 허용 여부;
- 요청한 token 수;
- 시도 이후 남은 whole token 수;
- rejected attempt에 대한 retry-after duration;
- bucket이 full이 될 때까지의 reset-after duration.

rejection은 정상 control flow이며 error로 반환하지 않는다. error는 validation,
context cancellation, backend failure에만 사용한다.

## Package 경계 선택지

| Option | Summary | Pros | Cons | Decision |
|---|---|---|---|---|
| Add limiter under `resilience` | Treat rate limiting as another policy. | Reuses existing resilience concept and HTTP adapters. | Rate limiting needs keyed local state and Redis backend packages; it is more than one in-process policy. | Reject for primary package; HTTP middleware can still compose with resilience handlers. |
| Add limiter under `cache` | Treat token buckets as cached counters. | Milestone is cache/coordination focused. | Rate limiting is not a cache contract; local state and Redis scripts would blur package purpose. | Reject. |
| Add root `ratelimit` and `ratelimit/redis` | Root package defines shared API, local keyed limiter, HTTP middleware; Redis subpackage owns Redis state. | Clear package purpose, mirrors `lock/redis`, avoids router dependencies, supports local and distributed implementations. | New public API surface and docs required. | Adopt. |
| Import `golang.org/x/time/rate` for local limiter | Use mature existing local limiter. | Battle-tested local behavior. | Adds a new dependency and does not solve Redis backend or keyed cleanup. | Reject for now; borrow semantics only. |

## Redis State Model

logical bucket마다 Redis key 하나를 사용한다.

```text
bluetape:ratelimit:<namespace>:bucket:<key>
```

state는 hash로 저장한다.

- `tokens`: remaining microtokens;
- `updated_ms`: Redis server time in milliseconds for the last refill.

Redis script는 다음을 수행해야 한다.

1. positive requested tokens, burst microtokens, refill rate를 검증한다;
2. Redis `TIME`을 읽는다;
3. bucket hash를 읽거나 full 상태로 초기화한다;
4. elapsed milliseconds 기준으로 refill한다;
5. token이 충분할 때만 requested tokens를 소비한다;
6. 갱신된 state를 `HSET`으로 저장한다;
7. idle expiration을 `PEXPIRE`로 설정한다;
8. allowed flag, remaining whole tokens, retry-after milliseconds,
   reset-after milliseconds, current time을 반환한다.

token을 integer microtokens로 표현하면 non-integer `RatePerSecond` 값을 지원하면서도
대부분의 fractional-token drift를 피할 수 있다.

## Concurrency 및 Consistency

- local limiter는 key별 bucket state를 담는 mutex-protected map 하나를 사용한다.
- Redis limiter는 시도마다 `EVAL` script 하나에 의존하므로, concurrent client가 같은
  key의 refill과 consume을 interleave할 수 없다.
- Redis server time은 distributed bucket의 authoritative time이다.
- Redis key expiration은 idle bucket memory를 제한한다.
- 어느 구현도 fairness queue가 아니다. concurrent caller burst는 mutex 또는 Redis
  command order로 중재된다.

## Public API 방향

```go
package ratelimit

type Limiter interface {
    Allow(ctx context.Context, key string, tokens int64) (Result, error)
}

type Options struct {
    RatePerSecond float64
    Burst         int64
    IdleTTL       time.Duration
}

type Result struct {
    Allowed    bool
    Requested  int64
    Remaining  int64
    RetryAfter time.Duration
    ResetAfter time.Duration
}

func New(options Options) (*TokenBucket, error)
```

Redis package 방향:

```go
package redisratelimit

type Options struct {
    Client        redis.Cmdable
    Namespace     string
    RatePerSecond float64
    Burst         int64
    IdleTTL       time.Duration
    MaxKeyBytes   int
}

func New(options Options) (*Limiter, error)
```

## HTTP Middleware 방향

root package는 standard-library middleware를 제공해야 한다.

- 모든 `Limiter` 구현을 받는다;
- request당 token 하나를 기본값으로 둔다;
- keying 기본값은 `Request.RemoteAddr` host만 사용한다;
- 기본적으로 proxy header를 신뢰하지 않는다;
- rejection 시 `429 Too Many Requests`를 쓴다;
- `RetryAfter`가 양수이면 `Retry-After`를 설정한다;
- custom rejection/error handler를 지원한다.

#25에서는 router-specific adapter를 추가하지 않는다.

## 테스트 요구사항

- option validation, burst, refill, rejection diagnostics, idle cleanup,
  context cancellation, concurrent access에 대한 local unit test.
- script defaults, burst, refill, concurrent clients, key expiration,
  context cancellation, namespace separation에 대한 Redis Testcontainers test.
- pass-through, rejection, custom key function, custom handler,
  `Retry-After` header에 대한 HTTP middleware test.
- local 및 Redis burst contention 모두에 대한 `GoroutineStressTester` stress test.
- `AsyncJobTester` cancellation test.
- race-targeted local test와 Redis package test.

## Benchmark 경계

#25는 rate limiting이 hot-path infrastructure이므로 새 package에 대한 focused
benchmark를 추가해야 한다.

- local allowed path;
- local rejected path;
- HTTP middleware allowed path.

Redis benchmark는 `make ci`의 일부가 아니라 opt-in이고 문서화된 형태여야 한다.
Redis benchmark scope가 커지면 첫 구현을 확장하지 말고 follow-up issue를 만든다.

## 결정

shared API, local keyed token bucket, HTTP middleware, local benchmark를 위해 새 root
`ratelimit` package를 구현한다. Redis server time과 key TTL을 사용하는 단일 Lua
`EVAL` script 기반 Redis-backed distributed bucket을 위해 새 `ratelimit/redis`
package를 구현한다. repository에 이미 있는 `go-redis/v9`와 Testcontainers를
재사용하고 dependency는 추가하지 않는다.
