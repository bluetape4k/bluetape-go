# WIP

Snapshot: 2026-06-05 KST
Scope: milestone `0.4.0` start.
Open count: 8 issues in milestone `0.4.0`.

## Current Release

`0.4.0` - State machine and lightweight workflow primitives.

## Current State

- `0.1.0`, `0.1.1`, `0.2.0`, and `0.3.0` are tagged and released.
- Issue #3 and milestone `0.3.0` are closed.
- Epic #4 is open for state machine and lightweight workflow primitives.
- Issue #135 refreshed source-grounded research, milestone spec, and
  implementation sequencing before package work began.
- Issue #26 delivered the independent `state` finite state machine package.
- Issue #28 delivered the shared `workreport` result and failure-policy model.
- Issue #27 owns `workflow` sequential, parallel, and conditional runners after
  `workreport` exists.
- Issue #136 tracks stress/cancellation coverage for 0.4.0 concurrency and
  timing contracts.
- Issue #137 tracks compile-checked runnable examples for new 0.4.0 APIs.
- Issues #132, #133, and #134 track package README and diagram completion.

## Release Checklist

1. Implement #27 `workflow`.
2. Close the coverage, example, README, and diagram issues.
3. Refresh README, WIP, and CHANGELOG before releasing `v0.4.0`.

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
