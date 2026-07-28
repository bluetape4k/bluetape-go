# Issue #25 Token-Bucket Rate Limiter Spec

> 한국어 요구사항 경계: 이 spec/design/test-spec 문서는 한국어 독자가 요구사항을 추적할 수 있도록 목적과 검증 경계를 한국어로 보강한다. API 이름, command, code identifier, issue/PR 번호, compatibility matrix, acceptance keyword, DoD/test evidence는 요구사항 약화를 막기 위해 원문 그대로 보존한다. 변경자는 아래 literal contract를 삭제하거나 의미를 약하게 바꾸지 않아야 한다.
> 추가 한국어 검증 메모: 영어로 남은 항목은 대부분 code/API/evidence literal이다. 구현 전에는 한국어 경계 문장과 원문 acceptance checklist를 함께 읽고, 검증 gate가 줄어들지 않았는지 확인한다.\n

Issue: #25
Milestone: 0.3.0
Date: 2026-06-05
Work type: Type A - Full Feature
Research: `docs/research/2026-06-05-issue-25-token-bucket-rate-limiter.md`

## Problem

`bluetape-go` has local cache, Redis invalidation, Redis locks, and Redis cache
coordination packages, but it does not yet provide a reusable rate limiter.
Issue #25 asks for practical token-bucket behavior in both local and
Redis-backed forms, plus an HTTP middleware example.

The implementation must be safe under concurrent callers, expose enough
diagnostics for users to make HTTP decisions, and fit the current package
layout without adding a dependency.

## 목표s

- Add a root `ratelimit` package.
- Add a `ratelimit/redis` package using `github.com/redis/go-redis/v9`.
- Support refill rate, burst capacity, arbitrary requested token count, and
  normal rejection diagnostics.
- Make local limiter concurrency-safe.
- Make Redis limiter safe for concurrent clients through a single atomic script.
- Provide standard-library HTTP middleware in the root package.
- Add stress tests using `GoroutineStressTester`.
- Add cancellation tests using `AsyncJobTester`.
- Add focused local benchmarks and document Redis benchmark boundary.
- Update root README pair, package README files, `CHANGELOG.md`, and `WIP.md`.

## Non-Goals

- Do not add `golang.org/x/time/rate` or any new dependency.
- Do not implement router-specific middleware for chi, gin, echo, or Fiber.
- Do not implement Redis Cluster multi-key operations.
- Do not implement distributed fairness, queuing, reservations, or blocking
  wait APIs.
- Do not implement adaptive rate limits or per-user dynamic policy storage.
- Do not place Redis benchmarks in normal `make ci`.
- Do not change `resilience`, `cache`, `lock/redis`, or `cache/rediscoord`
  public contracts.

## Public API

Package path:

- `github.com/bluetape4k/bluetape-go/ratelimit`

Core API:

```go
type Limiter interface {
    Allow(ctx context.Context, key string, tokens int64) (Result, error)
}

type Result struct {
    Allowed    bool
    Requested  int64
    Remaining  int64
    RetryAfter time.Duration
    ResetAfter time.Duration
}

type Options struct {
    RatePerSecond float64
    Burst         int64
    IdleTTL       time.Duration
}

type TokenBucket struct { ... }

func New(options Options) (*TokenBucket, error)
func (l *TokenBucket) Allow(ctx context.Context, key string, tokens int64) (Result, error)
```

Rules:

- `Allow` normalizes nil context to `context.Background()`.
- Already-canceled context returns the context error.
- blank `key` returns a validation error.
- `tokens <= 0` returns a validation error.
- `tokens > Burst` returns a validation error because the bucket can never
  satisfy that request.
- rejection returns `Result{Allowed:false}` and `nil` error.
- `Remaining` is whole tokens after the attempt.
- `RetryAfter` is zero for allowed attempts.
- `ResetAfter` is the approximate duration until the bucket is full.

## Local Options

Required:

- `RatePerSecond` must be positive.
- `Burst` must be positive.

Defaults:

- `IdleTTL == 0` becomes `max(2 * fullRefillDuration, 1m)`.

Invalid:

- negative `IdleTTL`;
- an `IdleTTL` lower than one full refill duration.

The local limiter stores per-key bucket state in memory. `IdleTTL` bounds memory
growth for inactive keys.

Tests may use an unexported constructor that accepts a `func() time.Time` clock.
The public API must not expose clock configuration unless a later user-facing
need appears.

## Redis API

Package path:

- `github.com/bluetape4k/bluetape-go/ratelimit/redis`

Package name:

- `redisratelimit`

Core API:

```go
type Options struct {
    Client        redis.Cmdable
    Namespace     string
    RatePerSecond float64
    Burst         int64
    IdleTTL       time.Duration
    MaxKeyBytes   int
}

type Limiter struct { ... }

func New(options Options) (*Limiter, error)
func (l *Limiter) Allow(ctx context.Context, key string, tokens int64) (ratelimit.Result, error)
```

Required:

- `Client` must not be nil.
- `RatePerSecond` must be positive.
- `Burst` must be positive.

Defaults:

- empty `Namespace` becomes `default`;
- `IdleTTL == 0` becomes `max(2 * fullRefillDuration, 1m)`;
- `MaxKeyBytes == 0` becomes `512`.

Invalid:

- blank `Namespace` after trimming when provided;
- negative `IdleTTL`;
- `IdleTTL` lower than one full refill duration;
- negative `MaxKeyBytes`;
- key whose byte length exceeds `MaxKeyBytes`;
- `Burst * 1_000_000`, requested tokens, or `RatePerSecond * 1_000_000`
  cannot be represented safely as positive `int64` microtokens.
- per-call requested tokens greater than `Burst`.

## Redis Keys And Script

Default prefix:

```text
bluetape:ratelimit:<namespace>
```

Per bucket:

```text
<prefix>:bucket:<key>
```

The Redis bucket key is a hash with:

- `tokens`: remaining microtokens;
- `updated_ms`: last refill time in Redis server milliseconds.

`Allow` calls one Lua `EVAL` with exactly one key. The script:

1. obtains Redis server time with `TIME`;
2. initializes missing buckets as full;
3. refills by elapsed milliseconds and `RatePerSecond`;
4. consumes requested microtokens only when enough tokens exist;
5. writes the updated hash with `HSET`;
6. refreshes idle expiration with `PEXPIRE`;
7. returns allowed flag, remaining whole tokens, retry-after milliseconds,
   reset-after milliseconds, and current time.

Token values are converted to microtokens in Go before the script call:

```text
1 token = 1_000_000 microtokens
```

This keeps fractional refill rates deterministic enough for package behavior
without introducing floating-point state in Go.

## HTTP Middleware

Root package API:

```go
type KeyFunc func(*http.Request) string
type HandlerErrorHandler func(http.ResponseWriter, *http.Request, Result, error)

type HandlerOptions struct {
    Limiter      Limiter
    KeyFunc      KeyFunc
    Tokens       int64
    ErrorHandler HandlerErrorHandler
}

type Handler struct { ... }

func NewHandler(next http.Handler, options HandlerOptions) (*Handler, error)
```

Rules:

- `Limiter` is required.
- `Tokens == 0` defaults to `1`.
- `Tokens < 0` is invalid.
- nil `next` becomes `http.NotFoundHandler()`.
- nil `KeyFunc` uses request remote IP.
- blank key from `KeyFunc` returns `503 Service Unavailable`.
- rejected attempts return `429 Too Many Requests`.
- rejected attempts set `Retry-After` when `RetryAfter > 0`.
- custom `ErrorHandler` may override response behavior.

The default remote-IP key must parse only `Request.RemoteAddr`. It must not
trust `X-Forwarded-For`, `Forwarded`, or similar proxy headers. Production apps
behind trusted proxies should provide an explicit `KeyFunc`.

## Approach Comparison

| Approach | Summary | Pros | Cons | Decision |
|---|---|---|---|---|
| `resilience` policy only | Add rate limit as a policy around existing handlers. | Composes with existing package. | Does not cover Redis backend, per-key state, or public limiter API cleanly. | Reject. |
| Root package plus Redis subpackage | Shared API and local middleware in root; Redis state isolated under `ratelimit/redis`. | Clear user model, mirrors existing Redis subpackages, no new dependencies. | Requires new docs and review gates. | Adopt. |
| Directly wrap `x/time/rate` | Use ecosystem local limiter. | Mature local implementation. | Adds a dependency and still needs keyed cleanup/Redis script. | Reject for #25. |

## Failure Modes And Boundaries

- Local limiter state is process-local only. Multiple processes each get their
  own bucket unless they use `ratelimit/redis`.
- Redis limiter depends on Redis availability. Redis command errors are returned
  and must not be converted to normal rejection.
- Redis script order is fair only by Redis command execution order; no FIFO
  waiting is promised.
- Long idle namespaces create one key per logical bucket until `IdleTTL` expiry.
- Very high `Burst` and `RatePerSecond` values can exceed safe microtoken math;
  options should reject values that would overflow `int64`.
- Remote-IP default keying is only a convenience; production apps should provide
  an authenticated user/API-key tenant key.

## Tests

Unit/local:

- option defaults and validation;
- local first burst allows up to `Burst`;
- local rejection returns retry-after;
- local refill restores tokens after clock advance;
- local idle cleanup removes stale key state;
- local concurrent stress allows at most `Burst` immediate consumers;
- canceled context returns context error;
- `TokenBucket` implements `Limiter`.

Redis/Testcontainers:

- option defaults and validation;
- missing client rejected;
- first burst allows up to `Burst`;
- concurrent clients for one key allow no more than `Burst` immediate consumes;
- refill after waiting permits later consume;
- namespace separation isolates buckets;
- key expiration occurs after `IdleTTL`;
- context cancellation returns context error;
- script errors are wrapped as Redis limiter errors;
- `Limiter` implements root `ratelimit.Limiter`.

HTTP:

- allowed request delegates to next handler;
- rejected request returns 429 and `Retry-After`;
- custom key function is used;
- custom error handler is used;
- backend error returns 503 by default.

Benchmarks:

- local allowed path;
- local rejected path;
- HTTP middleware allowed path.

Validation commands:

```bash
go test -count=1 ./ratelimit
go test -race -count=1 ./ratelimit
go test -count=1 ./ratelimit/redis
go test -race -count=1 ./ratelimit/redis
go test -count=1 ./ratelimit ./ratelimit/redis ./lock/redis
make ci
go test -run '^$' -bench '^Benchmark' -benchmem ./ratelimit
```

## Documentation

- Add `ratelimit/doc.go`.
- Add `ratelimit/README.md`.
- Add `ratelimit/redis/doc.go`.
- Add `ratelimit/redis/README.md`.
- Add compile-checked examples for local limiter and HTTP middleware.
- Add a Redis example when it can run without external services in compile-only
  form; otherwise keep Redis usage in README.
- Update `README.md` and `README.ko.md` package tables.
- Update `CHANGELOG.md` under Unreleased.
- Update `WIP.md` to mark #25 in progress and #125 closed.
- Update `docs/research/README.md`.
- Preserve external official-doc evidence in `bluetape4k-wiki` research notes
  and refresh GNO embeddings.
- Add lessons after implementation.

## Acceptance Criteria Mapping

| Issue criterion | Spec requirement |
|---|---|
| Local limiter supports refill rate and burst capacity. | `ratelimit.TokenBucket` with `RatePerSecond`, `Burst`, refill tests, and benchmarks. |
| Redis limiter is safe for concurrent clients. | `ratelimit/redis` uses one atomic Lua `EVAL`; Testcontainers stress proves no over-admission for a cold burst. |
| Include an HTTP middleware example. | Root package `NewHandler` plus compile-checked example and README. |
| Stress test requirement. | Local and Redis stress use `GoroutineStressTester`; cancellation uses `AsyncJobTester`. |

## Definition Of Done

- `ratelimit` builds and exposes local keyed token-bucket behavior.
- `ratelimit/redis` builds and exposes Redis-backed token-bucket behavior.
- No new dependency is added.
- Local and Redis concurrent stress tests pass.
- HTTP middleware behavior is covered and documented.
- Local benchmarks are added and documented.
- README English/Korean and CHANGELOG are updated.
- `make ci` passes.
- Step 2-R, Step 3-R, and Step 6-R reviews record `P0 = 0` and `P1 = 0`.
