# Issue 23 Redis Near Cache Spec

> 한국어 요구사항 경계: 이 spec/design/test-spec 문서는 한국어 독자가 요구사항을 추적할 수 있도록 목적과 검증 경계를 한국어로 보강한다. API 이름, command, code identifier, issue/PR 번호, compatibility matrix, acceptance keyword, DoD/test evidence는 요구사항 약화를 막기 위해 원문 그대로 보존한다. 변경자는 아래 literal contract를 삭제하거나 의미를 약하게 바꾸지 않아야 한다.
> 추가 한국어 검증 메모: 영어로 남은 항목은 대부분 code/API/evidence literal이다. 구현 전에는 한국어 경계 문장과 원문 acceptance checklist를 함께 읽고, 검증 gate가 줄어들지 않았는지 확인한다.\n

Issue: #23
Milestone: 0.3.0
Date: 2026-06-04
Work type: Type A - Full Feature

## Problem

`bluetape-go` has a framework-neutral local cache contract and `cache.Memory`.
The next 0.3.0 slice needs a Redis-backed near cache that keeps each process
fast through a local L1 cache while invalidating peer process L1 entries when a
write, delete, or clear occurs.

The design must keep application-level Redis Pub/Sub invalidation separate from
Redis RESP3 server-assisted client-side caching. Pub/Sub is the first
implementation for #23. RESP3 `CLIENT TRACKING` remains a future strategy
tracked by #110.

## 목표s

- Add a public `cache/redisnear` package.
- Reuse `cache.Memory[string, V]` as the default local L1 cache.
- Implement application-level Redis Pub/Sub invalidation for peer processes.
- Keep the public contract compatible with future strategy constructors such as
  RESP3 tracking.
- Prove peer invalidation with Redis Testcontainers.
- Include stress and cancellation tests using `GoroutineStressTester` and
  `AsyncJobTester`.
- Link benchmark follow-up to #107 instead of making benchmark execution part of
  normal CI.

## Non-Goals

- Do not implement RESP3 `CLIENT TRACKING` in #23.
- Do not add Ristretto, BigCache, or another local-cache dependency in #23.
- Do not turn Redis into the value store; Redis is used for invalidation only.
- Do not guarantee invalidation for writes that bypass this near-cache contract.
- Do not add benchmarks to normal `make ci`; benchmark work stays opt-in under
  #107.

## Public API

Package path:

- `github.com/bluetape4k/bluetape-go/cache/redisnear`

Core shape:

```go
type Client interface {
    Publish(ctx context.Context, channel string, message any) *redis.IntCmd
    Subscribe(ctx context.Context, channels ...string) *redis.PubSub
}

type Options[V any] struct {
    Client    Client
    Namespace string
    Channel   string
    OriginID  string
    Local     cache.LoadingCache[string, V]
    OnError   func(context.Context, error)
}

type NearCache[V any] struct { ... }

var ErrClosed = errors.New("near cache is closed")

func NewPubSub[V any](ctx context.Context, opts Options[V]) (*NearCache[V], error)
func (c *NearCache[V]) Get(ctx context.Context, key string) (V, error)
func (c *NearCache[V]) Set(ctx context.Context, key string, value V, ttl time.Duration) error
func (c *NearCache[V]) Delete(ctx context.Context, key string) error
func (c *NearCache[V]) Clear(ctx context.Context) error
func (c *NearCache[V]) GetOrLoad(ctx context.Context, key string, ttl time.Duration, loader cache.Loader[string, V]) (V, error)
func (c *NearCache[V]) Close() error
```

`NewPubSub` names the strategy explicitly. A future RESP3 implementation should
use a distinct constructor such as `NewTracking`, not a hidden runtime switch.

## Option Defaults

- `Client` is required.
- `Namespace` defaults to `default`.
- `Channel` defaults to `bluetape:cache:near:<namespace>:invalidate`.
- `OriginID` defaults to a random process-local token.
- `Local` defaults to `cache.NewMemory[string, V]()`.
- `OnError` is optional. When absent, subscription errors are not reported to
  user code. When present, errors are delivered best-effort through a bounded
  asynchronous reporter so a slow handler does not block the subscriber loop.
  Handler panics are recovered.

Invalid options:

- nil `Client`;
- empty `Namespace` after trimming;
- empty `Channel` after trimming;
- nil `Local` if explicitly supplied.

## Invalidation Message

Messages are JSON to keep the wire contract inspectable:

```json
{
  "version": 1,
  "namespace": "orders",
  "originID": "process-token",
  "operation": "delete",
  "key": "catalog:42"
}
```

Operations:

- `set`: peer caches delete the key;
- `delete`: peer caches delete the key;
- `clear`: peer caches clear all local entries for the namespace.

Rules:

- Messages from the same `OriginID` are ignored.
- Messages from another namespace are ignored.
- Unknown versions or operations are reported through `OnError` and ignored.
- Malformed payloads are reported through `OnError` and ignored.

## Behavior

`Get` delegates to the local cache.

`GetOrLoad` delegates to the local cache and keeps same-key stampede protection
within one process. Cross-process duplicate load suppression is out of scope.

`Set` stores the value in the local cache and publishes a `set` invalidation so
peer caches evict their stale copy. Publish errors are returned to the caller.
The local write is not rolled back after a publish error.

`Delete` deletes the local value and publishes a `delete` invalidation. Publish
errors are returned to the caller. The local delete is not rolled back.

`Clear` clears the local cache and publishes a `clear` invalidation. Publish
errors are returned to the caller. The local clear is not rolled back.

`Close` closes the Pub/Sub subscription and stops the subscriber loop. Repeated
`Close` calls are safe.

After `Close`, public cache operations return `ErrClosed`. Calls that start
after `Close` wins the lifecycle gate are rejected; operations already in flight
may finish before `Close` returns. `Close` uses the same shutdown budget for
in-flight operations and reports an error if they do not stop. This avoids
serving stale local data after invalidation has stopped without allowing
unbounded shutdown waits.

Subscription setup waits for the initial subscribe acknowledgement before
`NewPubSub` returns. This avoids a race where the first invalidation is
published before a peer instance is listening.

If the subscriber loop observes a receive error before close, it clears local
state and reports the error through `OnError`. This protects against stale reads
after reconnect ambiguity. Receive retries use a bounded backoff so repeated
errors cannot create a tight loop.

Pub/Sub messages are invalidation commands, not a trust boundary. Deployments
must isolate channels with Redis ACL/TLS and namespace/channel conventions.

## Consistency Model

This is an invalidation-based near cache:

- local reads may be stale until invalidation is received;
- writes that bypass this package do not invalidate app-level Pub/Sub peers;
- missed Pub/Sub messages can cause stale local data, so receive errors clear
  the local cache;
- peer invalidation deletes local entries instead of copying remote values.

## Tests

Unit tests:

- option validation and defaults;
- message encode/decode;
- same-origin messages ignored;
- wrong namespace messages ignored;
- malformed/unknown messages call `OnError`;
- blocking or panicking `OnError` does not stop subscriber invalidation;
- `Close` is idempotent;
- operations after `Close` return `ErrClosed`;
- close waits for an already entered operation before returning, with a bounded
  shutdown error if the operation does not stop.

Redis Testcontainers tests:

- two near-cache instances observe `Set` invalidation;
- two near-cache instances observe `Delete` invalidation;
- two near-cache instances observe `Clear` invalidation;
- `GetOrLoad` repopulates after peer invalidation;
- subscription is active before constructor returns.

Stress/cancellation tests:

- `GoroutineStressTester` exercises concurrent `Set`, `Delete`, `GetOrLoad`,
  and peer invalidation without data races or panics.
- `AsyncJobTester` proves context cancellation during construction/subscription
  or loader paths is surfaced and does not leave goroutines hanging.

Benchmark handling:

- #23 does not add benchmark commands to CI.
- #107 remains the benchmark follow-up and should include Redis NearCache
  hit/miss, publish latency, peer invalidation latency, and concurrent
  `GetOrLoad` scenarios after #23 lands.

## Documentation

- Add package documentation and examples for `redisnear`.
- Update `README.md` and `README.ko.md` if the public package table or cache
  section needs a Redis near-cache row.
- Update `CHANGELOG.md` under Unreleased.
- Add a lessons note after implementation.

## Step 1 Checklist Completion Report

| Item | Status | Notes |
|---|---|---|
| Target repository confirmed | Done | `origin` is `git@github.com:bluetape4k/bluetape-go.git`. |
| Worktree created | Done | `.worktrees/feat-issue-23-near-cache` on `feat/issue-23-near-cache`. |
| Issue inspected | Done | #23 requires Pub/Sub peer invalidation and Testcontainers proof. |
| User intent clear | Done | Support strategy boundary, stress tests, and benchmark follow-up. |
| Review-only boundary | N/A | User requested implementation work. |

## Step 1-R Checklist Completion Report

| Item | Status | Notes |
|---|---|---|
| Official docs checked | Done | Redis client-side caching docs, `CLIENT TRACKING`, and go-redis Pub/Sub docs. |
| Current repo checked | Done | `cache.Memory`, Redis Testcontainers fixture, package layout policy, and leader Redis patterns. |
| Dependency assumptions checked | Done | `go-redis/v9` Pub/Sub supports subscribe ack, reconnect/resubscribe, and channel/receive APIs. |
| Adopt/borrow/skip decisions recorded | Done | Pub/Sub first; RESP3 #110; `cache.Memory` first L1; Ristretto/BigCache deferred to benchmarks. |
| Stress/benchmark requirements recorded | Done | Stress tests in #23, benchmark follow-up linked to #107. |
