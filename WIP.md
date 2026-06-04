# WIP

Snapshot: 2026-06-05 KST
Scope: open GitHub issues assigned to `debop`.
Open count: 30 issues.

## Current Milestone

`0.3.0` - Cache and distributed coordination primitives.

## Current State

- `0.1.0`, `0.1.1`, and `0.2.0` are tagged and released.
- Foundation packages, Redis leader election, resilience policies, and the first
  cache contracts are merged on `develop`.
- Issue #22 is closed with Type A research/spec/plan/review/lessons artifacts,
  generic cache interfaces, in-process TTL memory cache, `GetOrLoad`
  same-key stampede protection, and stress/cancellation coverage.
- Issue #23 is closed with Redis Pub/Sub near-cache invalidation and
  Testcontainers peer invalidation coverage.
- Issue #107 is closed with opt-in cache benchmark baselines, so performance
  measurements stay outside ordinary package tests.
- Issue #24 is closed with Redis owner-token distributed lock implementation,
  expiration/contention coverage, and stress/cancellation tests.
- Issue #117 is closed with Type A research/spec/plan/review artifacts and a
  Redis coordination wrapper for cross-process cache stampede protection.
- Issue #123 is closed with package-specific usage, behavior, and benchmark
  notes moved into package-level README files.
- Issue #125 is closed with Go coverage reports published from CI and Nightly.
- Issue #25 is in progress with Type A research/spec/plan/review artifacts and
  local plus Redis-backed token-bucket rate limiter implementation work.
- Regular CI and Nightly workflows continue to run Testcontainers-backed tests
  against real containers and now publish Go coverage artifacts.

## Next Feature Issues

1. #25 Finish token-bucket rate limiter validation and PR.
2. #86 Add pluggable leader election strategies.
3. #3 Close the 0.3.0 epic after remaining issues land.

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
- Use Go native coverage reports for bluetape-go CI/Nightly first; defer
  third-party coverage SaaS uploads or threshold enforcement until the baseline
  is stable.
