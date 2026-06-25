# WIP

Snapshot: 2026-06-25 KST
Scope: `0.6.8` release closure after the 7-tier P0/P1 remediation pass.

## Current Target Release

`v0.6.8` - P0/P1 remediation from the 7-tier review of modules delivered
through `v0.6.7`.

The milestone hardens release docs, bounded decompression and ECB XML fetches,
MongoDB JWT cursor cleanup, Redis leader shutdown, redisnear reporter shutdown,
Docker-backed test startup contexts, copyable Redis/AWS examples, and
core/collections validation error contracts.

## Current State

- `0.1.0`, `0.1.1`, `0.2.0`, `0.3.0`, `0.4.0`, `0.5.0`, `v0.5.1`,
  `v0.6.0`, and `v0.6.1` through `v0.6.7` are tagged and released.
- Milestone `0.6.8` has no open issues after PR #303.
- Issue #290 corrected root README release status for `v0.6.7`.
- Issue #291 bounded MongoDB JWT trim cursor cleanup.
- Issue #293 capped ECB exchange-rate response bodies before XML decode.
- Issue #294 added bounded decompression for untrusted compressed payloads.
- Issue #295 bounded Redis leader renewal IO and `Resign` shutdown waits.
- Issue #298 updated Redis leader and lock examples to use bounded contexts.
- Issue #299 updated AWS examples to use bounded contexts and preserve SDK
  errors.
- Issue #300 made redisnear reporter shutdown visible and bounded.
- Issue #301 bounded Testcontainers startup contexts in Docker-backed tests.
- Issue #302 added caller-detectable validation errors for core and collections
  helpers.

## Release Checklist

1. `0.6.8` issue-linked PRs are merged on `develop`.
2. Milestone `0.6.8` is closed with `open_issues=0`.
3. `CHANGELOG.md` has a `v0.6.8` section dated `2026-06-25`.
4. `git diff --check`, local `make ci`, and GitHub CI are release gates.
5. Promote `develop` to `main`, tag `v0.6.8` on `main`, and publish the GitHub
   Release.
