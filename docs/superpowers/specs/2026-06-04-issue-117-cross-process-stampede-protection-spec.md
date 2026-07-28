# Issue #117 Cross-Process Stampede Protection Spec

> 한국어 요구사항 경계: 이 spec/design/test-spec 문서는 한국어 독자가 요구사항을 추적할 수 있도록 목적과 검증 경계를 한국어로 보강한다. API 이름, command, code identifier, issue/PR 번호, compatibility matrix, acceptance keyword, DoD/test evidence는 요구사항 약화를 막기 위해 원문 그대로 보존한다. 변경자는 아래 literal contract를 삭제하거나 의미를 약하게 바꾸지 않아야 한다.
> 추가 한국어 검증 메모: 영어로 남은 항목은 대부분 code/API/evidence literal이다. 구현 전에는 한국어 경계 문장과 원문 acceptance checklist를 함께 읽고, 검증 gate가 줄어들지 않았는지 확인한다.\n

Issue: #117
Milestone: 0.3.0
Date: 2026-06-04
Work type: Type A - Full Feature
Research: `docs/research/2026-06-04-issue-117-cross-process-stampede-protection.md`

## Problem

`cache.Memory.GetOrLoad` prevents duplicate same-key loads only inside one
process. `cache/redisnear.NearCache` invalidates peer local caches, but it does
not coordinate reloads after a peer invalidation. Multiple processes can
therefore stampede the backend for the same cold key.

#117 adds an opt-in Redis coordination wrapper that collapses one cold burst
across processes without changing the root `cache.LoadingCache` interface or the
default `redisnear` behavior.

## 목표s

- Add `cache/rediscoord` as a public package.
- Implement an opt-in `StampedeCache[V]` wrapper for
  `cache.LoadingCache[string,V]`.
- Use the #24 Redis owner-token/TTL lock primitive for load ownership.
- Share a short-lived encoded result envelope through Redis so waiters can fill
  local caches without calling their user loaders.
- Keep `cache/redisnear.NewPubSub` invalidation-only by default.
- Prove two or more Redis NearCache instances collapse the same cold key after
  invalidation.
- Add cancellation, timeout, lease-expiry, and stress coverage.
- Document consistency/failure semantics in `README.md` and `README.ko.md`.
- Keep benchmark execution opt-in and linked with #107.

## Non-Goals

- Do not change `cache.Cache`, `cache.LoadingCache`, or `cache.Loader`.
- Do not add coordination to `redisnear.NearCache.GetOrLoad` by default.
- Do not implement a durable Redis L2 cache.
- Do not implement Redlock, lock renewal, or fencing tokens.
- Do not guarantee mutual exclusion after a loader exceeds `LockTTL`.
- Do not add coordinator benchmarks to `make ci`.

## Public API

Package path:

- `github.com/bluetape4k/bluetape-go/cache/rediscoord`

Core API:

```go
type Codec[V any] interface {
    Marshal(V) ([]byte, error)
    Unmarshal([]byte) (V, error)
}

type JSONCodec[V any] struct{}

type Options[V any] struct {
    Client       redis.Cmdable
    Cache        cache.LoadingCache[string, V]
    Namespace    string
    Codec        Codec[V]
    LockTTL      time.Duration
    ResultTTL    time.Duration
    PollInterval time.Duration
}

type StampedeCache[V any] struct { ... }

func NewStampedeCache[V any](options Options[V]) (*StampedeCache[V], error)
func (c *StampedeCache[V]) Get(ctx context.Context, key string) (V, error)
func (c *StampedeCache[V]) Set(ctx context.Context, key string, value V, ttl time.Duration) error
func (c *StampedeCache[V]) Delete(ctx context.Context, key string) error
func (c *StampedeCache[V]) Clear(ctx context.Context) error
func (c *StampedeCache[V]) GetOrLoad(ctx context.Context, key string, ttl time.Duration, loader cache.Loader[string, V]) (V, error)
```

`StampedeCache[V]` implements `cache.LoadingCache[string,V]`.

## Option Defaults And Validation

Required:

- `Client` must not be nil.
- `Cache` must not be nil.
- `Codec` must not be nil.

Defaults:

- empty `Namespace` becomes `default`;
- zero `LockTTL` becomes `5s`;
- zero `ResultTTL` becomes `1s`;
- zero `PollInterval` becomes `10ms`.

Invalid:

- blank `Namespace` after trimming when provided;
- negative `LockTTL`;
- negative `ResultTTL`;
- negative `PollInterval`.

## Redis Keys

Default prefix:

```text
bluetape:cache:coord:<namespace>
```

Per cache key:

```text
<prefix>:lock:<key>
<prefix>:result:<key>
```

The lock key is owned by `lock/redis` and stores the owner token. The result key
stores a short-lived JSON envelope.

## Result Envelope

```json
{
  "version": 1,
  "token": "owner-token",
  "payload": "base64-codec-output"
}
```

Rules:

- only version `1` is accepted;
- `token` must match the owner token observed by the waiter for the current
  lock attempt;
- payload is produced and consumed by `Codec[V]`;
- malformed or mismatched envelopes are ignored as unusable coordination
  artifacts;
- matching envelopes that fail codec unmarshal return an error because the
  shared result for the current owner is corrupt or incompatible.

## GetOrLoad Behavior

1. Normalize nil context to `context.Background()`.
2. Return immediately if the context is already canceled.
3. Reject nil loader.
4. Check the wrapped cache with `Get`.
5. If the key is hot, return the value.
6. If the key is cold, try to acquire the Redis load lock through #24
   `lock/redis`.
7. If this process wins:
   - re-check and fill through the wrapped cache's `GetOrLoad`;
   - marshal the winning value with `Codec[V]`;
   - publish a result envelope with `ResultTTL`;
   - release the lock using a background cleanup context so caller cancellation
     does not skip unlock.
8. If another process owns the lock:
   - observe the current owner token;
   - poll the matching result envelope at `PollInterval`;
   - when a matching envelope appears, decode the value and call the wrapped
     cache's `GetOrLoad` with a local loader that returns that decoded value;
   - if the lock disappears without a matching result, retry acquisition;
   - if the caller context expires, return the context error.

The wrapper calls the user loader only in the winning process. Waiters must not
call their user loader when a matching result envelope is available.

## Consistency And Failure Semantics

- Successful results are stored in the wrapped local cache by `GetOrLoad`; the
  result envelope is not a durable Redis cache entry.
- Loader errors are returned and are not cached.
- Codec marshal errors are returned and are not cached by the winning loader
  path.
- Codec unmarshal errors on a matching owner result are returned to the waiter
  because the current shared result is corrupt or incompatible.
- Redis command errors are returned unless the operation already has a valid
  local cache hit.
- Unlock uses owner-token compare-and-delete through #24. Unlock errors after a
  successful load are reported as operation errors only when the successful
  value cannot be safely returned without hiding a coordination failure. If a
  cleanup attempt fails because Redis is unavailable, the lock TTL remains the
  deadlock guard.
- If a winning loader runs longer than `LockTTL`, another process may acquire
  the lock and run a loader. This is documented as lease expiry behavior, not a
  bug.
- `Set`, `Delete`, and `Clear` delegate to the wrapped cache. They do not turn
  the result envelope into a durable invalidation protocol.

## Tests

Unit tests:

- missing `Client`, missing `Cache`, and missing `Codec` are rejected;
- defaults are applied;
- invalid durations are rejected;
- `JSONCodec[V]` round-trips a simple value;
- result envelopes require a matching token;
- `StampedeCache[V]` implements `cache.LoadingCache[string,V]`.

Redis Testcontainers:

- two `redisnear.NearCache` instances under one namespace collapse a cold
  post-invalidation key to one user loader invocation;
- waiters fill their wrapped local cache from the shared result;
- caller cancellation while waiting returns `context.Canceled` or
  `context.DeadlineExceeded`;
- lock/lease expiry lets a peer make progress when a winner never publishes a
  result;
- stress burst across two wrappers and one key completes with one loader for
  the first cold burst.

Validation commands:

```bash
go test -count=1 ./cache/rediscoord
go test -race -count=1 ./cache/rediscoord
go test -count=1 ./cache ./cache/redisnear ./cache/rediscoord ./lock/redis
make ci
```

## Documentation

- Add `cache/rediscoord/doc.go`.
- Add a compile-checked example.
- Update `README.md` and `README.ko.md` package/status sections and cache usage
  notes.
- Update `CHANGELOG.md` under Unreleased.
- Add a lessons note after implementation.
- Keep benchmark text linked to #107 and avoid adding benchmark execution to
  normal CI.

## Acceptance Criteria Mapping

| Issue criterion | Spec requirement |
|---|---|
| Research package placement | Research adopts explicit `cache/rediscoord` wrapper. |
| Public API shape without weakening `LoadingCache` | Wrapper implements existing interface; root interface unchanged. |
| Owner tokens/TTL to avoid deadlock | #24 lock reused; `LockTTL` bounds abandoned owners. |
| Two or more near-cache instances contending | Testcontainers test wraps two `redisnear.NearCache` peers. |
| Timeout/cancellation and lease expiry | Cancellation and expiry tests required. |
| README docs if public API added | README pair and package docs required. |
| Benchmark linked to #107 | Docs link opt-in benchmark work; no `make ci` benchmark. |

## Definition Of Done

- `cache/rediscoord` builds and implements `cache.LoadingCache[string,V]`.
- Testcontainers prove cross-process result collapse after near-cache
  invalidation.
- Cancellation and lease-expiry tests pass.
- Stress test uses `GoroutineStressTester`.
- Cancellation test uses `AsyncJobTester`.
- README English/Korean and CHANGELOG are updated.
- `make ci` passes.
- Step 2-R, Step 3-R, and Step 6-R reviews record `P0 = 0` and `P1 = 0`.
