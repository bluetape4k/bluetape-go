# WIP

Snapshot: 2026-06-21 KST
Scope: `0.6.1` portable utility hardening after the `v0.6.0` release.

## Current Target Release

`v0.6.1` - Patch release for portable utility hardening after the `v0.6.0`
foundation. The release includes Redis-backed probabilistic Bloom filters,
distributed JWT KeyChain repositories, optional JWT provider cache adapters,
provider-backed money exchange rates, CLDR-backed locale currency lookup,
FastMoney benchmark evidence, ID generator performance evidence, codec
compatibility hardening, JWT compression/JWE boundary documentation, and the
0.1.0-0.6.1 retrospective audit gate.

## Current State

- `0.1.0`, `0.1.1`, `0.2.0`, `0.3.0`, `0.4.0`, `0.5.0`, `v0.5.1`, and
  `v0.6.0` are tagged and released.
- Milestone `0.6.1` has zero open issues and is ready for release promotion
  after local CI and GitHub release-PR CI pass.
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

## Release Checklist

1. Verify milestone `0.6.1` has zero open issues and no open PRs.
2. Keep Redis-backed Cuckoo/HyperLogLog constructors, IMF and Bloomberg money
   providers, and any future explicit JWE compression scope deferred in docs and
   issue tracking.
3. Run local `make ci` on `develop`.
4. Close milestone `0.6.1`.
5. Promote `develop` to `main` through a release PR, then tag `main` as
   `v0.6.1` and create the GitHub Release.

## Next Milestone Queue

Milestone `0.7.0` is the next planned line after `v0.6.1` release promotion.

The planned portable-utilities scope is represented in the roadmap and should
be rechecked against current GitHub issues before each new package starts.

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
