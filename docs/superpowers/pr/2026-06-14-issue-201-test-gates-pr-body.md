## Summary

Fixes #201.

This PR upgrades the missing failure, cancellation, race, and cleanup gates that were identified as part of the `0.6.2` corrective milestone. The main goal is to make local CI and stress validation reliable before continuing broader parity expansion work.

## Background

Several Redis/Testcontainers-backed packages were individually healthy but unstable under full-suite local CI. The failures were not production behavior regressions; they were test-gate weaknesses:

- cleanup used unbounded `context.Background()` termination paths;
- Redis tests repeatedly created one container per test in heavy packages;
- some Redis fixtures overrode the Testcontainers module readiness strategy with a log-only wait;
- repo-wide `make test` and `make race` let Testcontainers-backed packages run concurrently.

## What This Solves

- Adds a bounded internal Testcontainers cleanup helper that ignores parent cancellation while preserving context values.
- Routes Testcontainers wrappers and direct Redis fixture tests through bounded cleanup.
- Uses package-shared Redis fixtures with per-test `FlushDB` isolation in heavy Redis suites.
- Keeps Redis module default readiness behavior instead of replacing it with log-only waits.
- Runs repo-wide test, race, and coverage gates with serial package scheduling.
- Adds missing `GoroutineStressTester` failure/cancellation coverage and records targeted stress/race evidence.

## Work Done

- Added `internal/testcleanup` with unit coverage for parent cancellation, timeout, and nil terminator handling.
- Updated Redis, Kafka, MySQL, NATS, and Postgres Testcontainers wrappers to use bounded cleanup.
- Stabilized Redis-backed `jwt`, `leader/redis`, and `probabilistic/redis` tests with package-shared fixtures and bounded readiness checks.
- Removed direct `container.Terminate(context.Background())` usage from Go sources.
- Updated root README locale pair to document `-p 1` test/race behavior.
- Added Step 6-R and lessons artifacts.

## Validation

- `make ci`: PASS.
- `git diff --check`: PASS.
- `go test -count=1 ./testing/concurrency ./cache/rediscoord ./jwt ./leader/redis ./probabilistic/redis -run 'GoroutineStressTester|Stress'`: PASS.
- `go test -race -count=1 ./testing/concurrency ./cache/rediscoord ./jwt ./leader/redis ./probabilistic/redis -run 'GoroutineStressTester|Stress'`: PASS.
- `rg -n "container\\.Terminate\\(context\\.Background\\(\\)|WithWaitStrategy\\(|testcontainers/internal/cleanup|go test -race -count=1 ./\\.\\.\\.|go test -count=1 ./\\.\\.\\." --glob '*.go' README.md README.ko.md Makefile`: PASS, no hits.

## Review Notes

- Step 2-R: `docs/superpowers/reviews/2026-06-14-issue-201-test-gates-step-2r-spec-review.md`, P0=0 P1=0.
- Step 3-R: `docs/superpowers/reviews/2026-06-14-issue-201-test-gates-step-3r-plan-review.md`, P0=0 P1=0.
- Step 6-R: `docs/superpowers/reviews/2026-06-14-issue-201-test-gates-step-6r-code-review.md`, P0=0 P1=0.
- Step 7-R: `docs/superpowers/reviews/2026-06-14-issue-201-test-gates-step-7r-pr-review.md`, P0=0 P1=0.
- Subagent note: native subagent lanes were unstable in this session, so 7-Tier lanes were executed by main-session role switching and documented as fallback.

## Metadata

- Issue: #201.
- Milestone: `0.6.2`.
- Base: `develop`.
- Head: `issue-201-test-gates`.

## DoD Status

| Step | Status | Evidence |
|---|---|---|
| Step 0 - Issue/worktree setup | PASS | Issue #201 inspected; worktree `.worktrees/issue-201-test-gates`, branch `issue-201-test-gates`. |
| Step 2 - Spec | PASS | `docs/superpowers/specs/2026-06-14-issue-201-test-gates-design.md`; diagram asset generated under `docs/images/readme-diagrams/`. |
| Step 2-R - Spec review | PASS | `docs/superpowers/reviews/2026-06-14-issue-201-test-gates-step-2r-spec-review.md`, P0=0 P1=0. |
| Step 3 - Plan | PASS | `docs/superpowers/plans/2026-06-14-issue-201-test-gates-plan.md`. |
| Step 3-R - Plan review | PASS | `docs/superpowers/reviews/2026-06-14-issue-201-test-gates-step-3r-plan-review.md`, P0=0 P1=0. |
| Step 4 - Implementation | PASS | Bounded cleanup helper, Redis fixture stabilization, serial test gates, and GoroutineStressTester edge coverage committed. |
| Step 4-T - Tests | PASS | `make ci`; targeted GoroutineStressTester normal and race commands above. |
| Step 6-R - 7-Tier code review | PASS | `docs/superpowers/reviews/2026-06-14-issue-201-test-gates-step-6r-code-review.md`, P0=0 P1=0. |
| Step 7 - Lessons | PASS | `docs/lessons/2026-06-14-issue-201-test-gates.md`. |
| Step 7-P - PR creation | PASS | PR #237 created against `develop`; milestone `0.6.2`, assignee `debop`; live body verified. |
| Step 7-R - Post-PR review | PASS | `docs/superpowers/reviews/2026-06-14-issue-201-test-gates-step-7r-pr-review.md`, P0=0 P1=0; PR comment and formal review posted. |
| Step 8 - CI gate | PASS | GitHub Actions `ci` passed: https://github.com/bluetape4k/bluetape-go/actions/runs/27494105916/job/81264830021. |
