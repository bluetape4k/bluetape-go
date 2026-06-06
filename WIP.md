# WIP

Snapshot: 2026-06-06 KST
Scope: milestone `0.4.0` closure.
Remaining after #4 cleanup: 0 issues in milestone `0.4.0`.

## Current Release

`0.4.0` - State machine and lightweight workflow primitives.

## Current State

- `0.1.0`, `0.1.1`, `0.2.0`, and `0.3.0` are tagged and released.
- Issue #3 and milestone `0.3.0` are closed.
- Epic #4 completed the state machine and lightweight workflow primitive scope.
- Issue #135 refreshed source-grounded research, milestone spec, and
  implementation sequencing before package work began.
- Issue #26 delivered the independent `state` finite state machine package.
- Issue #28 delivered the shared `workreport` result and failure-policy model.
- Issue #27 delivered `workflow` sequential, parallel, and conditional runners
  after `workreport` exists.
- Issue #136 verifies the milestone stress/cancellation gate for 0.4.0
  concurrency and timing contracts.
- Issue #132 verifies package-level README coverage and root package indexes
  for the new 0.4.0 package surface.
- Issue #137 verifies compile-checked runnable examples for new 0.4.0 APIs.
- Issues #133 and #134 verify README diagram coverage for the 0.4.0 state and
  workflow primitives plus complex Redis coordination packages.

## Release Checklist

1. Close epic #4 and milestone `0.4.0` after this cleanup lands.
2. Release `v0.4.0`.

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
