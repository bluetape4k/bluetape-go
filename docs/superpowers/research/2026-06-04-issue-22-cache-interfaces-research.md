# Issue 22 Cache Interfaces Research

Issue: #22
Milestone: 0.3.0
Date: 2026-06-04

## Research Question

`bluetape-go` 0.3.0 cache 작업의 첫 단계로, Redis near cache(#23)나
외부 local-cache 구현체를 먼저 선택하지 않고도 안정적인 공개 cache 계약을
정의할 수 있는지 확인한다. 핵심 검증 대상은 generic cache interface,
loader pattern, TTL semantics, 그리고 동일 key concurrent load 중복 억제다.

## Primary Evidence

| Source | Evidence | Decision impact |
|---|---|---|
| GitHub issue #22 | generic cache interface, loader pattern, TTL/cache-miss tests, same-key duplicate-load prevention, `GoroutineStressTester`/`AsyncJobTester` requirement | Scope is public API plus deterministic concurrency tests. |
| `docs/research/2026-06-01-milestone-0.3.0-cache-coordination-research.md` | Go direction is small cache interfaces and loader/stampede-protection patterns; `singleflight` and `go-redis/v9` are candidates | Start with framework-neutral cache contracts and reuse existing direct dependency `golang.org/x/sync`. |
| `docs/package-layout.md` | new public packages need clear domain, package docs, examples, concurrency/context behavior, error semantics | `cache` can be a public top-level package only if docs/tests/examples ship with it. |
| `go.mod` | `golang.org/x/sync v0.20.0` is already a direct dependency | No new dependency is needed for duplicate-load suppression. |
| `resilience/policy.go` | `Operation[T] func(context.Context) (T, error)` and nil context normalization style | Cache loader should be context-aware and follow existing nil-context tolerance only at package entry points. |
| `testing/concurrency/*` | `GoroutineStressTester` and `AsyncJobTester` already collect concurrent/cancellation failures | Acceptance tests must use both helpers explicitly. |
| Go package docs: `golang.org/x/sync/singleflight` | `Group.Do` suppresses duplicate in-flight calls per key and shares the original result | Implement stampede protection with `singleflight.Group`, not ad hoc per-key locks. |
| Go package docs: `context` | contexts carry deadlines/cancellation across API boundaries; context should be passed explicitly | Public cache APIs should take `context.Context` as first parameter and propagate cancellation to loaders. |
| bluetape4k-wiki system-design cache notes | cache design must state strategy, eviction/TTL behavior, stale-read tolerance, invalidation, and two-tier boundaries | #22 must define TTL and miss semantics now, but leave cross-node invalidation to #23. |

## Current Repository Patterns

- Existing public packages are small top-level domains with `doc.go`, examples,
  and targeted tests.
- Context-aware APIs normalize nil context at the top of package entry points
  when this repository already permits it, but official Go docs still prefer
  non-nil caller contexts.
- Existing concurrency tests use package-local deterministic helpers rather than
  sleeping goroutines without failure aggregation.
- README already lists `cache` as a planned package for 0.3.0, so adding it is
  roadmap-consistent.

## Adopt / Borrow / Skip Decisions

| Candidate | Decision | Rationale |
|---|---|---|
| `golang.org/x/sync/singleflight` | Adopt | Already direct dependency; directly matches same-key in-flight duplicate suppression. |
| `github.com/dgraph-io/ristretto` | Skip for #22 | Useful as a future adapter candidate, but #22 needs a stable bluetape cache contract before choosing storage policy. |
| `github.com/allegro/bigcache` | Skip for #22 | Same reason as Ristretto; capacity/eviction implementation choice belongs to later adapter work. |
| Redis-backed near cache | Defer to #23 | #22 should expose contracts that #23 can implement, without locking Redis invalidation details into local cache behavior. |
| Custom per-key lock map | Reject | Higher lifecycle/cleanup risk than `singleflight`, and weaker evidence than the upstream duplicate-suppression package. |

## Technical Constraints

- The first cache package must stay framework-neutral and avoid Redis/Testcontainers.
- TTL semantics must be deterministic enough for unit tests without relying on
  wall-clock sleeps where a fake clock or short bounded wait can prove behavior.
- Cache-miss representation must distinguish absent/expired entries from loader
  errors.
- `GetOrLoad` must prevent duplicate concurrent loads for the same key inside
  one cache instance. Cross-process suppression is out of scope for #22.
- Loader cancellation must propagate the caller context and must not cache
  canceled or failed loads.
- Public docs and README content must be English; Go source comments must be
  Korean and short.

## Design Inputs for Spec

1. Public package: `cache`.
2. Minimal API shape:
   - `Cache[K comparable, V any]` with `Get`, `Set`, `Delete`, `Clear`.
   - `Loader[K, V] func(context.Context, K) (V, error)`.
   - `GetOrLoad(ctx, key, ttl, loader)` behavior on a concrete in-memory cache
     or wrapper.
3. TTL contract:
   - zero TTL means no expiration for that write;
   - positive TTL expires based on insertion time;
   - negative TTL is invalid and should return an error.
4. Miss contract:
   - expose `ErrCacheMiss` so `errors.Is` works;
   - expired entries count as miss and are removed on access.
5. Stampede contract:
   - same key shares one in-flight loader result;
   - different keys can load independently;
   - failed/canceled loader results are not cached.
6. Required tests:
   - cache hit/miss/delete/clear;
   - TTL hit before expiry and miss after expiry;
   - loader miss fills the cache;
   - loader error/cancellation is not cached;
   - `GoroutineStressTester` proves same-key duplicate loads collapse to one;
   - `AsyncJobTester` proves deadline/cancellation propagation.

## Step 1-R Checklist Completion Report

| Item | Status | Notes |
|---|---|---|
| Official docs checked | Done | `pkg.go.dev` for `singleflight` and `context`. |
| Current repo and ecosystem searched | Done | Issue #22, milestone research, package policy, README, `resilience`, and testing helpers inspected. |
| Third-party API assumptions checked | Done | `go doc` and official `pkg.go.dev` confirm `singleflight.Group.Do` behavior. |
| Adopt/borrow/skip decisions recorded | Done | See decision table above. |
| Technical constraints identified | Done | Public API, TTL, miss/error, cancellation, concurrency, docs language. |
| Research summary ready for Step 2 | Done | Design inputs listed. |
