# WIP

Snapshot: 2026-06-09 KST
Scope: `0.6.0` portable utilities after the `v0.5.1` patch release.

## Current Target Release

`v0.6.0` - Portable service utilities, including the `id` package for UUID, ULID,
standard KSUID, and Snowflake identifiers, the `jwt` package for
explicit-algorithm JWT signing, parsing, validation, and local key rotation, and
the `measure` package for typed units, measured values, compound units, parsing,
formatting, and temperature helpers, and the `money` package for ISO currency
and decimal-backed amount operations, and the `probabilistic` package for
in-memory Bloom filters.

## Current State

- `0.1.0`, `0.1.1`, `0.2.0`, `0.3.0`, `0.4.0`, `0.5.0`, and `v0.5.1` are
  tagged and released.
- Milestone `0.6.0` implementation is complete; release promotion is in
  progress.
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

## Release Checklist

1. Issue #36 is complete with implementation, tests, docs, and Step 6-R subagent
   7-Tier code review.
2. Keep Flake, Hashids, distributed JWT repositories (#173), future JWE
   compression scope after #174, provider cache adapters (#175),
   provider-backed money exchange rates (#178), full locale mapping (#179),
   FastMoney evaluation (#180), and Redis-backed Cuckoo/HyperLogLog filters
   after the Bloom subset of #182 deferred in docs and issue tracking.
3. Local `make ci`, PR review, and GitHub CI passed for the final #36 merge.
4. Recheck GitHub milestone state, close epic #6, close milestone `0.6.0`, then
   promote `develop` to `main` and tag `v0.6.0`.

## Next Milestone Queue

Milestone `0.6.1` is the next patch queue after `v0.6.0` release promotion.

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
