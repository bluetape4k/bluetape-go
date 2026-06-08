# WIP

Snapshot: 2026-06-08 KST
Scope: `v0.5.1` patch release preparation after the checkpoint-safe batch
writer skip fix merged.

## Current Target Release

`0.5.1` - Patch release for checkpoint-safe skipped writer chunks in
`batch.Step`.

## Current State

- `0.1.0`, `0.1.1`, `0.2.0`, `0.3.0`, `0.4.0`, and `0.5.0` are tagged and
  released.
- Milestone `0.5.0` implementation is merged on `develop`.
- Issue #29 delivered the `batch` reader/processor/writer core and sequential
  job model.
- Issue #30 delivered retry/skip policies, checkpoint storage, restart
  behavior, and the root README architecture diagram refresh.
- Issue #31 delivered leader-guarded scheduler and migration batch examples
  with Redis Testcontainers runnable commands.
- Issue #158 delivered the checkpoint-safe writer skip fix after the `v0.5.0`
  tag existed, so the release target is `v0.5.1` without rewriting `v0.5.0`.
- Epic #5 and milestone `0.5.0` are ready for closure after the patch release
  is published.

## Release Checklist

1. Merge the `v0.5.1` release-prep documentation PR into `develop`.
2. Close epic #5 after verifying child issues #29, #30, #31, and #158 are
   closed.
3. Close milestone `0.5.0` after its open issue count reaches zero.
4. Promote `develop` to `main` through the `v0.5.1` release PR.
5. Tag `main` as `v0.5.1`.
6. Create GitHub Release `v0.5.1` from the changelog section.

## Next Milestone Queue

Milestone `0.6.0` is the next release queue after `v0.5.1` ships.

The planned portable-utilities scope is represented in the roadmap and should
be rechecked against current GitHub issues before implementation starts.

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
- Keep 0.5.0 `batch` first-party and narrow: chunk steps, retry/skip policies,
  checkpoints, reports, and examples are in scope; durable external checkpoint
  adapters are deferred.
