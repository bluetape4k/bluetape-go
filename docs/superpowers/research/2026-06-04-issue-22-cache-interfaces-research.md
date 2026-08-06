# Issue 22 Cache Interfaces Research

Issue: #22
Milestone: 0.3.0
Date: 2026-06-04

## Research Question

`bluetape-go` 0.3.0 cache 작업의 첫 단계로, Redis near cache(#23)나 외부 local-cache
구현체를 먼저 선택하지 않고도 안정적인 public cache contract를 정의할 수 있는지 확인한다.
핵심 검증 대상은 generic cache interface, loader pattern, TTL semantics, 동일 key concurrent
load duplicate suppression이다.

## Primary Evidence

| Source | Evidence | Decision impact |
|---|---|---|
| GitHub issue #22 | generic cache interface, loader pattern, TTL/cache-miss tests, same-key duplicate-load prevention, `GoroutineStressTester`/`AsyncJobTester` requirement | scope는 public API와 deterministic concurrency tests다. |
| `docs/research/2026-06-01-milestone-0.3.0-cache-coordination-research.md` | Go direction은 small cache interface와 loader/stampede-protection pattern이며 `singleflight`, `go-redis/v9`가 candidate다. | framework-neutral cache contract로 시작하고 기존 direct dependency `golang.org/x/sync`를 재사용한다. |
| `docs/package-layout.md` | 새 public package에는 clear domain, package docs, examples, concurrency/context behavior, error semantics가 필요하다. | docs/tests/examples가 함께 제공될 때만 `cache`를 public top-level package로 둘 수 있다. |
| `go.mod` | `golang.org/x/sync v0.20.0`이 이미 direct dependency다. | duplicate-load suppression에 새 dependency가 필요하지 않다. |
| `resilience/policy.go` | `Operation[T] func(context.Context) (T, error)`와 nil context normalization style이 있다. | cache loader는 context-aware여야 하며 nil-context tolerance는 package entry point에서만 기존 style을 따른다. |
| `testing/concurrency/*` | `GoroutineStressTester`와 `AsyncJobTester`가 concurrent/cancellation failure를 수집한다. | acceptance test는 두 helper를 명시적으로 사용해야 한다. |
| Go package docs: `golang.org/x/sync/singleflight` | `Group.Do`는 key별 duplicate in-flight call을 억제하고 original result를 공유한다. | ad hoc per-key lock 대신 `singleflight.Group`로 stampede protection을 구현한다. |
| Go package docs: `context` | context는 API boundary를 지나 deadline/cancellation을 전달하며 명시적으로 전달되어야 한다. | public cache API는 첫 parameter로 `context.Context`를 받고 loader에 cancellation을 전달한다. |
| bluetape4k-wiki system-design cache notes | cache design은 strategy, eviction/TTL behavior, stale-read tolerance, invalidation, two-tier boundary를 명시해야 한다. | #22는 TTL과 miss semantics를 지금 정의하고 cross-node invalidation은 #23으로 남긴다. |

## Current Repository Patterns

- 기존 public package는 `doc.go`, examples, targeted tests를 가진 작은 top-level domain이다.
- 이 repository가 허용하는 context-aware API는 package entry point에서 nil context를 normalize하지만,
  official Go docs는 caller context가 non-nil이어야 한다고 본다.
- 기존 concurrency test는 failure aggregation 없이 sleeping goroutine을 쓰지 않고 package-local
  deterministic helper를 사용한다.
- README는 이미 `cache`를 0.3.0 planned package로 나열하므로 추가는 roadmap-consistent하다.

## Adopt / Borrow / Skip Decisions

| Candidate | Decision | Rationale |
|---|---|---|
| `golang.org/x/sync/singleflight` | Adopt | 이미 direct dependency이며 same-key in-flight duplicate suppression과 직접 맞는다. |
| `github.com/dgraph-io/ristretto` | Skip for #22 | future adapter candidate로 유용하지만 #22는 storage policy 선택 전에 stable bluetape cache contract가 필요하다. |
| `github.com/allegro/bigcache` | Skip for #22 | Ristretto와 같은 이유다. capacity/eviction implementation choice는 later adapter work다. |
| Redis-backed near cache | Defer to #23 | #22는 #23이 구현할 contract를 노출하되 Redis invalidation detail을 local cache behavior에 고정하지 않는다. |
| Custom per-key lock map | Reject | `singleflight`보다 lifecycle/cleanup risk가 높고 upstream duplicate-suppression package보다 evidence가 약하다. |

## Technical Constraints

- 첫 cache package는 framework-neutral이어야 하며 Redis/Testcontainers를 피한다.
- TTL semantics는 fake clock 또는 짧고 bounded된 wait로 unit test가 가능할 만큼 deterministic해야 한다.
- cache miss 표현은 absent/expired entry와 loader error를 구분해야 한다.
- `GetOrLoad`는 하나의 cache instance 안에서 같은 key에 대한 duplicate concurrent load를 막아야 한다.
  cross-process suppression은 #22 밖이다.
- loader cancellation은 caller context를 전달해야 하며 canceled/failed load result를 cache하면 안 된다.
- public docs와 README content는 English다. Go source comments는 한국어로 짧게 작성한다.

## Design Inputs for Spec

1. Public package: `cache`.
2. Minimal API shape:
   - `Cache[K comparable, V any]` with `Get`, `Set`, `Delete`, `Clear`.
   - `Loader[K, V] func(context.Context, K) (V, error)`.
   - concrete in-memory cache 또는 wrapper의 `GetOrLoad(ctx, key, ttl, loader)`.
3. TTL contract:
   - zero TTL은 해당 write에 expiration이 없음을 뜻한다.
   - positive TTL은 insertion time 기준으로 expire한다.
   - negative TTL은 invalid이며 error를 반환해야 한다.
4. Miss contract:
   - `errors.Is`가 동작하도록 `ErrCacheMiss`를 노출한다.
   - expired entry는 miss로 취급하고 access 시 제거한다.
5. Stampede contract:
   - 같은 key는 하나의 in-flight loader result를 공유한다.
   - 다른 key는 독립적으로 load할 수 있다.
   - failed/canceled loader result는 cache하지 않는다.
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
| Current repo and ecosystem searched | Done | Issue #22, milestone research, package policy, README, `resilience`, testing helpers inspected. |
| Third-party API assumptions checked | Done | `go doc`와 official `pkg.go.dev`가 `singleflight.Group.Do` behavior를 확인한다. |
| Adopt/borrow/skip decisions recorded | Done | 위 decision table 참조. |
| Technical constraints identified | Done | public API, TTL, miss/error, cancellation, concurrency, docs language. |
| Research summary ready for Step 2 | Done | design input을 나열했다. |
