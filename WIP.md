# WIP

Snapshot: 2026-06-21 KST
Scope: `0.6.2` release closure after the `v0.6.1` release.

## Current Target Release

`v0.6.2` - Release-ready corrective milestone for source-parity evidence and
implementation hardening after the `v0.6.1` release. The milestone records the
parity matrix for `core`, `testing/junit5`, and `testing/testcontainers`,
normalizes public API/error-contract evidence, adds the IMF provider slice, and
documents the Bloomberg provider boundary before the `0.6.3` through `0.6.6`
corrective implementation series.

## Current State

- `0.1.0`, `0.1.1`, `0.2.0`, `0.3.0`, `0.4.0`, `0.5.0`, `v0.5.1`,
  `v0.6.0`, and `v0.6.1` are tagged and released. `v0.6.2` is prepared for
  release tagging after `develop` is promoted to `main`.
- Milestone `0.6.1` is closed. Milestone `0.6.2` child work is complete and is
  ready for epic and milestone closure.
- Issue #29 delivered the `batch` reader/processor/writer core and sequential
  job model.
- Issue #30 delivered retry/skip policies, checkpoint storage, restart
  behavior, and the root README architecture diagram refresh.
- Issue #31 delivered leader-guarded scheduler and migration batch examples
  with Redis Testcontainers runnable commands.
- Issue #32 delivered the `id` package foundation.
- Child issues #164, #165, and #167 are closed by UUID v4/v7, random and
  monotonic ULID, and Snowflake generation.
- Issue #166 delivered the standard seconds-precision KSUID generator family for
  `0.6.0`.
- Issue #33 delivered the `jwt` helper package with explicit algorithms,
  fixed and rotating in-memory KeyChains, typed claim readers, `kid` lookup,
  weak-secret rejection, and stress/race coverage.
- Issue #34 delivered the `measure` package with typed unit/measure values,
  family parsers, compound units, source-parity helpers, affine temperature, and
  stress/race coverage.
- Issue #35 delivered the `money` helper package with ISO 4217 currency
  wrappers, decimal-backed `Money` values, caller-supplied exchange-rate
  conversion, and stress/race coverage.
- Issue #36 delivered the final `0.6.0` package: goroutine-safe in-memory Bloom
  filters with deterministic config, merge compatibility, and stress/race
  coverage.
- Millisecond KSUID compatibility (#171) is tracked in `0.6.1`; Flake and
  Hashids remain deferred outside 0.6.0 closure.
- Distributed JWT KeyChain repositories (#173) and optional JWT provider cache
  adapters (#175) remain deferred outside #33. JWT compression/JOSE dependency
  scope (#174) is researched for `0.6.1`: signed JWT compression remains a
  non-goal, and any standards-compatible compression belongs to a future
  explicit JWE boundary.
- Provider-backed money exchange rates (#178), full locale-to-currency mapping
  (#179), and long-backed FastMoney evaluation (#180) remain deferred outside
  #35.
- Redis-backed Bloom support is delivered by #182 / PR #229; Redis-backed
  Cuckoo/HyperLogLog constructors remain separate follow-up scope after the
  Bloom API.
- Issue #200 completed the retrospective audit gate for `0.1.0` through
  `0.6.1` and recorded the final blocker state before release.
- Issue #201 completed the test-gate hardening lesson for Testcontainers and
  Redis-backed packages.
- Issue #202 is the shared source-parity matrix for the corrective `0.6.x`
  series.
- Issue #203 records the public API and error-contract audit for packages
  created or materially changed in `0.1.0` through `0.6.1`.
- Issue #231 adds the IMF Exchange Rates provider slice for `money`, limited to
  USD/EUR domestic pivot pairs with caller-visible source, freshness, stale,
  and failure metadata.
- Issue #232 evaluates Bloomberg-backed exchange rates as a licensed
  customer-owned integration boundary. No default `money` provider, Bloomberg
  dependency, credential path, or paid-access CI path is added in `0.6.2`.

## Release Checklist

1. `0.6.2` issue-linked PRs are merged on `develop`.
2. #202 is complete before the `0.6.3` core parity expansion.
3. High-value parity gaps are mapped to #204, #209, #215, or #221 unless a
   narrower follow-up issue is required.
4. `git diff --check`, local `make ci`, and GitHub CI are release gates.
5. Close `0.6.2` after epic #199 records the closed child issues and release
   validation evidence.

## Next Milestone Queue

Milestones `0.6.3` through `0.6.6` form the corrective implementation series
after `0.6.2`: core foundation parity, testing helper parity, Testcontainers
contract expansion, and integration/developer-experience closure.

From `0.8.0` onward, the roadmap order is relational SQL, AWS/Floci, text,
audit, then graph. SQL moved earlier because repository/database ergonomics are
foundational backend service infrastructure. Graph moved to the last planned
slot because driver maturity, backend abstraction, graph I/O, and domain
examples carry more research uncertainty.

## Decision Log

- Keep `README.md` and `README.ko.md` synchronized when package scope, roadmap,
  install guidance, or development commands change.
- Use `develop` as the default branch and run CI on `develop`, `main`, and pull
  requests.
- Keep public packages idiomatic to Go. Do not mechanically port Kotlin
  extension APIs.
- Prefer small packages with clear service value over catch-all utility bags.
- Use Testcontainers-backed smoke tests for infrastructure packages.
- Use `-count=1` in CI and Nightly test commands so Go's test cache cannot hide
  Testcontainers execution.
- Keep cache cross-process behavior out of the local memory cache; Redis
  near-cache invalidation belongs to `cache/redisnear`, while opt-in load-result
  coordination belongs to `cache/rediscoord`.
- Keep benchmarks opt-in and tracked under #107 before turning them into a
  release gate.
- Add stress/cancellation tests for new `0.3.0` coordination features when
  concurrency or timing semantics are part of the contract.
- Refresh README, WIP, and CHANGELOG after milestone merges, not only at tag
  time.
- Before starting a new milestone package, compare issue scope against the
  broader `bluetape4k-*` ecosystem so Go support is not accidentally too narrow.
- Keep package-level Korean README siblings synchronized with English package
  README changes.
- Use Go native coverage reports for bluetape-go CI/Nightly first; defer
  third-party coverage SaaS uploads or threshold enforcement until the baseline
  is stable.
- Start 0.4.0 with `state`, `workreport`, and `workflow` as separate packages;
  do not mechanically port Kotlin DSL, coroutine, reactive, or Virtual Thread
  layers.
- Keep workflow runners `context.Context`-driven and first-party; avoid durable
  orchestration engines in 0.4.0.
- Keep 0.5.0 `batch` first-party and narrow: chunk steps, retry/skip policies,
  checkpoints, reports, and examples are in scope; durable external checkpoint
  adapters are deferred.
