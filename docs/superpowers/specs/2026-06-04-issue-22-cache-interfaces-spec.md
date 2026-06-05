# Issue 22 Cache Interfaces And Stampede Protection Spec

Issue: #22
Milestone: 0.3.0
Research: `docs/superpowers/research/2026-06-04-issue-22-cache-interfaces-research.md`

## Context

Issue #22 starts the 0.3.0 cache/coordination milestone. The issue asks for a
framework-neutral Go cache contract, a loader pattern, TTL behavior and
cache-miss tests, and same-key duplicate concurrent load prevention.

This is not the Redis near-cache implementation. Redis invalidation, Pub/Sub,
and cross-process coordination remain #23. This issue creates the local package
contract that later local and Redis-backed caches can implement.

## Goals

- Add a public top-level `cache` package with package docs, examples, and tests.
- Define generic cache and loader contracts for comparable keys.
- Provide a first in-memory implementation that supports TTL.
- Expose deterministic cache-miss semantics through `ErrCacheMiss`.
- Provide `GetOrLoad` behavior that fills misses through a context-aware loader.
- Suppress duplicate in-flight loads for the same key inside one cache instance.
- Use `GoroutineStressTester` and `AsyncJobTester` in the test suite.
- Update README package/status and add a copy-paste cache example.

## Non-Goals

- Do not implement Redis near-cache, Pub/Sub invalidation, or distributed cache
  stampede protection in this issue.
- Do not add Ristretto, BigCache, or another storage dependency before the
  bluetape-go cache contract stabilizes.
- Do not expose eviction policies beyond TTL in this issue.
- Do not promise Kotlin/JVM cache-key or Redis-key interoperability.
- Do not add metrics/tracing exporters; hooks can be designed later if needed.

## Public API

The public package is `github.com/bluetape4k/bluetape-go/cache`.

```go
var ErrCacheMiss = errors.New("cache miss")

type Loader[K comparable, V any] func(context.Context, K) (V, error)

type Cache[K comparable, V any] interface {
    Get(ctx context.Context, key K) (V, error)
    Set(ctx context.Context, key K, value V, ttl time.Duration) error
    Delete(ctx context.Context, key K) error
    Clear(ctx context.Context) error
}

type LoadingCache[K comparable, V any] interface {
    Cache[K, V]
    GetOrLoad(ctx context.Context, key K, ttl time.Duration, loader Loader[K, V]) (V, error)
}
```

`NewMemory[K comparable, V any]()` returns a `LoadingCache[K, V]` backed by an
in-process map protected by a mutex.

## Behavior Contract

- `Get` returns `ErrCacheMiss` when a key is absent or expired.
- `errors.Is(err, ErrCacheMiss)` must work for miss detection.
- Expired entries are removed on access.
- `Set` with `ttl == 0` stores a value without expiration.
- `Set` or `GetOrLoad` with `ttl < 0` returns an error and does not write.
- `Delete` is idempotent for absent keys.
- `Clear` removes all entries and is safe to call repeatedly.
- All public methods are safe for concurrent callers.
- `Delete` and `Clear` do not cancel an already in-flight loader. If a
  `GetOrLoad` call started before `Delete` or `Clear` and its loader succeeds
  later, it may repopulate the cache according to normal cache-aside ordering.
- Public methods accept `context.Context` as the first argument. A nil context is
  normalized to `context.Background()` to match existing repository entry-point
  tolerance, but examples use non-nil contexts.
- If `ctx.Err()` is already set before work starts, public methods return that
  context error and do not mutate cache state.
- Loader errors and context cancellation errors are returned to callers and are
  not cached.
- `GetOrLoad` first checks the cache. On miss, it runs the loader through
  `singleflight.Group.Do` using a stable key representation scoped to the cache
  instance.
- The `singleflight` key must be collision-free for `K` values inside the cache
  instance. Implementation must not rely on ad hoc `fmt.Sprint` stringification
  when distinct comparable keys could collapse to the same string.
- Concurrent `GetOrLoad` calls for the same key share one in-flight loader
  result.
- Concurrent `GetOrLoad` calls for different keys may run independently.
- The cache mutex must not be held while a loader is running.
- If a shared loader succeeds, all waiting callers observe the same value and
  the value is stored once with the requested TTL.
- If callers race with different TTL values for the same key while a load is in
  flight, the winning loader call's TTL defines the stored expiration. This is
  documented as a same-key concurrency contract, not a caller-specific guarantee.

## Architecture Options Considered

| Option | Description | Decision |
|---|---|---|
| Minimal interface plus memory implementation | Define `cache` contracts and a simple TTL memory cache with `singleflight` loading | Adopt. It proves the public API, TTL, miss, and stampede behavior without binding future adapters to one storage dependency. |
| Storage-wrapper only around Ristretto/BigCache | Pick a mature local cache implementation immediately and wrap it | Reject for #22. The milestone research says bluetape-go value is coordination and consistent behavior, not another generic cache library. |
| Loader helper separate from cache implementation | Provide only a stampede-protected function wrapper and no cache storage | Reject. It would not satisfy TTL/cache-miss acceptance and would leave #23 without a tested cache contract. |

## Failure Modes And Mitigations

| Risk | Mitigation |
|---|---|
| Cache stampede remains under race | Use `singleflight.Group.Do`; add stress test that asserts one loader execution for many same-key callers. |
| Distinct generic keys collapse into one `singleflight` key | Maintain a cache-instance-scoped flight-key mapping or equivalent collision-free key namespace. |
| Expired values are returned accidentally | Store expiration timestamp per entry; `Get` removes expired entries before returning miss. |
| Failed or canceled loader poisons cache | Write only after loader returns nil error and context is still valid. |
| Nil loader panic or silent zero-value cache | Validate loader before `singleflight` and return a normal error. |
| Loader blocks unrelated cache operations | Never hold the cache mutex while invoking the loader. |
| Caller expects `Delete`/`Clear` to cancel existing loads | Document cache-aside ordering; deletion does not cancel in-flight loaders. |
| TTL tests become flaky | Prefer injected clock or narrowly scoped time controls in memory cache tests. If real time is used, keep bounded `Eventually` checks. |
| Public API becomes Redis-specific too early | Keep Redis invalidation and cross-process suppression out of the `cache` root package for #22. |

## Tests

- Unit tests:
  - `Get` returns `ErrCacheMiss` for absent keys.
  - `Set` then `Get` returns the stored value.
  - `Delete` and `Clear` are idempotent.
  - positive TTL expires and returns `ErrCacheMiss`.
  - zero TTL does not expire during a bounded consistency check.
  - negative TTL returns validation error and does not write.
  - nil loader returns validation error.
  - loader error is not cached; a later successful loader can store the value.
  - canceled/deadline context propagates and does not cache.
  - distinct keys that stringify similarly do not share one loader result.
- Stress tests:
  - `GoroutineStressTester` runs many same-key `GetOrLoad` calls and verifies
    loader invocation count is exactly one for a cold key.
  - `GoroutineStressTester` or a focused concurrent test verifies different keys
    do not share one loader result.
  - `AsyncJobTester` verifies loader observes cancellation/deadline and all
    callers receive a context-compatible error.
- Examples:
  - compile-checked `ExampleNewMemory_getOrLoad`.

## Documentation

- Add `cache/doc.go` with package purpose, TTL contract, context behavior, and
  error semantics.
- Update `README.md` and `README.ko.md` package tables and add a concise cache
  example.
- Keep public docs and examples in English.
- Keep Go source comments in Korean and start exported comments with the
  declaration identifier.

## Acceptance Criteria Mapping

| Issue criterion | Spec requirement |
|---|---|
| Define generic cache interface and loader pattern | `Cache`, `LoadingCache`, `Loader`, `NewMemory`. |
| Add TTL behavior and cache-miss tests | TTL contract, `ErrCacheMiss`, unit tests. |
| Prevent duplicate concurrent loads for the same key | `singleflight.Group.Do` backed `GetOrLoad`, stress tests. |
| Use `GoroutineStressTester`, `AsyncJobTester` | Required stress/cancellation tests. |

## Definition Of Done

- `cache` package builds with docs, examples, and tests.
- All #22 acceptance criteria are covered by tests.
- `go test -count=1 ./cache` passes.
- `go test -race -count=1 ./cache` passes or a documented local blocker exists.
- `make ci` passes before PR.
- README English/Korean package status stays synchronized.
- Step 2-R, Step 3-R, Step 6-R gates record `P0 = 0` and `P1 = 0`.

## Step 2 Checklist Completion Report

| Item | Status | Notes |
|---|---|---|
| Architecture pre-design ran | Done | Three architecture options compared and failure modes listed. |
| Step 1-R research incorporated | Done | Research file linked and evidence reflected in API/behavior contracts. |
| Current-behavior claims cite evidence | Done | Existing package, context, dependency, and test-helper patterns cited in research. |
| Spec path confirmed inside feature worktree | Done | This file is under `.worktrees/feat-issue-22-cache-interfaces`. |
| Risks/failure modes included | Done | Six failure modes with mitigations listed. |
| Approach comparison and rejection rationale included | Done | Alternatives table included. |
| Brainstorming process applied | Done | Problem, options, trade-offs, risks, DoD, docs, and tests captured. |
| Acceptance criteria and DoD included | Done | Mapping and DoD sections included. |
