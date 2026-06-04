# Issue #25 Token-Bucket Rate Limiter Research

Issue: #25
Milestone: 0.3.0
Date: 2026-06-05
Work type: Type A - Full Feature

## Research Question

How should `bluetape-go` add practical token-bucket rate limiting with local and
Redis-backed implementations, without adding a new dependency or weakening the
existing Redis coordination package boundaries?

## Current Repository Evidence

- `go.mod` already depends on `github.com/redis/go-redis/v9 v9.20.0`.
- `lock/redis` uses `redis.Cmdable`, validates options before command
  execution, and uses `Eval` for owner-token compare-and-delete.
- `cache/rediscoord` uses explicit Redis coordination packages instead of
  hiding cross-process behavior inside the local cache contract.
- `resilience/http.go` exposes standard-library HTTP adapters without binding
  to any router framework.
- `testing/concurrency` provides `GoroutineStressTester` and `AsyncJobTester`,
  which #25 explicitly requires for stress/cancellation coverage.

## External Evidence

| Source | Relevant point | Decision impact |
|---|---|---|
| Redis `EVAL`, https://redis.io/docs/latest/commands/eval/ | Redis runs a Lua script server-side and receives keys through `KEYS` plus arguments through `ARGV`. | Use one script per consume attempt so refill, consume, and expiration are atomic for concurrent clients. |
| Redis `TIME`, https://redis.io/docs/latest/commands/time/ | Redis exposes server time as seconds and microseconds. | Redis limiter should use Redis server time inside the script to avoid client clock skew across processes. |
| Redis `PEXPIRE`, https://redis.io/docs/latest/commands/pexpire/ | Redis can assign millisecond expiration to a key. | Bucket state keys should expire automatically after an idle window. |
| go-redis v9 package docs, https://pkg.go.dev/github.com/redis/go-redis/v9 | `Client` and `Cmdable` expose `Eval(ctx, script, keys, args...)`. | Reuse the existing `redis.Cmdable` boundary and avoid client lifecycle ownership. |
| Go `x/time/rate`, https://pkg.go.dev/golang.org/x/time/rate | The standard ecosystem has a mature local token-bucket limiter API. | Borrow conceptual semantics, but do not add the dependency because the workflow forbids new dependencies without explicit request. |

## Token-Bucket Semantics

A token bucket holds up to `Burst` tokens. Tokens refill over time at
`RatePerSecond`, and a request consumes `Tokens` tokens only when enough tokens
are available. This permits short bursts up to `Burst` while bounding long-term
rate to `RatePerSecond`.

Returned diagnostics should include:

- whether the request was allowed;
- requested token count;
- remaining whole tokens after the attempt;
- retry-after duration for rejected attempts;
- reset-after duration until the bucket is full.

Rejection is normal control flow and should not be returned as an error.
Errors are reserved for validation, context cancellation, and backend failures.

## Package Boundary Options

| Option | Summary | Pros | Cons | Decision |
|---|---|---|---|---|
| Add limiter under `resilience` | Treat rate limiting as another policy. | Reuses existing resilience concept and HTTP adapters. | Rate limiting needs keyed local state and Redis backend packages; it is more than one in-process policy. | Reject for primary package; HTTP middleware can still compose with resilience handlers. |
| Add limiter under `cache` | Treat token buckets as cached counters. | Milestone is cache/coordination focused. | Rate limiting is not a cache contract; local state and Redis scripts would blur package purpose. | Reject. |
| Add root `ratelimit` and `ratelimit/redis` | Root package defines shared API, local keyed limiter, HTTP middleware; Redis subpackage owns Redis state. | Clear package purpose, mirrors `lock/redis`, avoids router dependencies, supports local and distributed implementations. | New public API surface and docs required. | Adopt. |
| Import `golang.org/x/time/rate` for local limiter | Use mature existing local limiter. | Battle-tested local behavior. | Adds a new dependency and does not solve Redis backend or keyed cleanup. | Reject for now; borrow semantics only. |

## Redis State Model

Use one Redis key per logical bucket:

```text
bluetape:ratelimit:<namespace>:bucket:<key>
```

Store state as a hash:

- `tokens`: remaining microtokens;
- `updated_ms`: Redis server time in milliseconds for the last refill.

The Redis script should:

1. validate positive requested tokens, burst microtokens, and refill rate;
2. read Redis `TIME`;
3. read the bucket hash or initialize it as full;
4. refill based on elapsed milliseconds;
5. consume requested tokens only if enough tokens remain;
6. store updated state with `HSET`;
7. set idle expiration with `PEXPIRE`;
8. return allowed flag, remaining whole tokens, retry-after milliseconds,
   reset-after milliseconds, and current time.

Representing tokens as integer microtokens avoids most fractional-token drift
while still supporting non-integer `RatePerSecond` values.

## Concurrency And Consistency

- Local limiter uses one mutex-protected map of per-key bucket state.
- Redis limiter relies on one `EVAL` script per attempt, so concurrent clients
  cannot interleave refill and consume for the same key.
- Redis server time is authoritative for distributed buckets.
- Redis key expiration bounds idle bucket memory.
- Neither implementation is a fairness queue. A burst of concurrent callers is
  arbitrated by mutex or Redis command order.

## Public API Direction

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

Redis package direction:

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

## HTTP Middleware Direction

The root package should provide standard-library middleware:

- accepts any `Limiter` implementation;
- defaults to one token per request;
- defaults keying to `Request.RemoteAddr` host only;
- does not trust proxy headers by default;
- writes `429 Too Many Requests` on rejection;
- sets `Retry-After` when `RetryAfter` is positive;
- supports a custom rejection/error handler.

No router-specific adapters should be added in #25.

## Test Requirements

- Local unit tests for option validation, burst, refill, rejection diagnostics,
  idle cleanup, context cancellation, and concurrent access.
- Redis Testcontainers tests for script defaults, burst, refill, concurrent
  clients, key expiration, context cancellation, and namespace separation.
- HTTP middleware tests for pass-through, rejection, custom key function, custom
  handler, and `Retry-After` header.
- Stress tests with `GoroutineStressTester` for both local and Redis burst
  contention.
- Cancellation tests with `AsyncJobTester`.
- Race-targeted local tests and Redis package tests.

## Benchmark Boundary

#25 should add focused benchmarks for the new package because rate limiting is
hot-path infrastructure:

- local allowed path;
- local rejected path;
- HTTP middleware allowed path.

Redis benchmarks should be opt-in and documented, not part of `make ci`.
If Redis benchmark scope grows, create a follow-up issue instead of expanding
the first implementation.

## Decision

Implement a new root `ratelimit` package for shared API, local keyed token
bucket, HTTP middleware, and local benchmarks. Implement a new
`ratelimit/redis` package for Redis-backed distributed buckets using a single
Lua `EVAL` script with Redis server time and key TTL. Reuse `go-redis/v9` and
Testcontainers already present in the repository; do not add dependencies.
