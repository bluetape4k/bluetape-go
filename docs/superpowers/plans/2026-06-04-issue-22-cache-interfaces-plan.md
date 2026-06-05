# Issue 22 Cache Interfaces Implementation Plan

Spec: `docs/superpowers/specs/2026-06-04-issue-22-cache-interfaces-spec.md`
Spec review: `docs/superpowers/reviews/2026-06-04-issue-22-cache-interfaces-spec-review.md`
Issue: #22
Milestone: 0.3.0

## Execution Principles

- Implement only the #22 local/framework-neutral cache surface.
- Do not add Redis near-cache behavior or new cache-storage dependencies.
- Keep public docs and examples in English.
- Keep Go source comments Korean, short, and Go-doc compatible.
- Make tests deterministic enough to run in `go test -count=1 ./cache` and
  `go test -race -count=1 ./cache`.

## Tasks

| Task | Description | Spec mapping | Verification |
|---|---|---|---|
| T1 | Add package skeleton: `cache/doc.go`, `cache/errors.go`, `cache/cache.go`. Define `ErrCacheMiss`, `Loader`, `Cache`, `LoadingCache`, and `NewMemory` API. | Generic cache interface and loader pattern. | `go test -count=1 ./cache` compiles. |
| T2 | Implement `memoryCache[K,V]` storage with mutex-protected `map[K]entry[V]`, expiration timestamp, and nil-context normalization at public entry points. | TTL, miss, delete, clear, context behavior. | Unit tests for hit/miss/delete/clear/context pre-cancel. |
| T3 | Add TTL validation and expiration cleanup-on-access. Treat zero TTL as no expiration, positive TTL as insertion-relative expiry, and negative TTL as validation error. | TTL behavior and cache-miss tests. | Unit tests for zero/positive/negative TTL and expired miss removal. |
| T4 | Implement `GetOrLoad` with loader validation, cache-first lookup, `singleflight.Group.Do`, write-after-success only, and no cache mutex held during loader execution. | Loader pattern and failure/cancellation behavior. | Tests for loader success, loader error not cached, nil loader, and cancellation not cached. |
| T5 | Implement collision-free `singleflight` key namespace for generic `K comparable` keys. Use cache-instance-scoped key IDs or an equivalent collision-free mapping rather than ad hoc stringification. Clean up auxiliary key metadata on `Delete`/`Clear` where it is not needed for an active load. | Step 2-R P1 fix; same-key duplicate suppression correctness. | Test with distinct comparable keys whose naive string form would collide or be ambiguous. |
| T6 | Add same-key stampede stress test using `GoroutineStressTester`. Many concurrent cold-key `GetOrLoad` calls must complete with one loader invocation and identical values. | Duplicate concurrent loads prevented. | `go test -count=1 ./cache -run Test.*SameKey.*` |
| T7 | Add different-key concurrency test. Distinct keys must load independently and must not receive another key's value. | Prevent accidental over-sharing. | `go test -count=1 ./cache -run Test.*DifferentKeys.*` |
| T8 | Add cancellation/deadline stress test using `AsyncJobTester`. Loader must observe context cancellation and failed/canceled results must not be cached. | Go stress test requirement and context behavior. | `go test -count=1 ./cache -run Test.*Cancellation.*` |
| T9 | Add compile-checked examples: `ExampleNewMemory_getOrLoad` and package-level docs covering TTL, `ErrCacheMiss`, context, concurrent-call safety, local-only stampede scope, and the fact that `Delete`/`Clear` do not cancel in-flight loaders. | Package docs and user/caller clarity. | `go test -count=1 ./cache -run Example` |
| T10 | Update `README.md` and `README.ko.md` package table plus concise cache example. Keep locale meaning aligned. | Documentation DoD. | Grep README for `cache`, `GetOrLoad`, and `ErrCacheMiss`. |
| T11 | Run formatting and targeted verification: `gofmt`, `go test -count=1 ./cache`, `go test -race -count=1 ./cache`, `git diff --check`. | Local package quality and race safety. | Commands pass or blocker recorded. |
| T12 | Run broader validation: `make ci` after targeted checks pass. | Pre-PR DoD. | `make ci` pass or precise blocker. |
| T13 | Run Step 6-R code review and verifier against spec/plan. | Workflow gate. | Review artifact has `P0 = 0`, `P1 = 0`. |
| T14 | Add lessons, commit with Lore trailers, push branch, create PR, check CI status. | Workflow closure. | Lessons committed before PR; PR body DoD final section present. |

## Test Matrix

| Behavior | Unit | Stress | Race | Docs/example |
|---|---:|---:|---:|---:|
| absent key returns `ErrCacheMiss` | Yes | N/A | Yes | Yes |
| set/get/delete/clear | Yes | N/A | Yes | Yes |
| zero TTL no expiration | Yes | N/A | Yes | Mention |
| positive TTL expires | Yes | N/A | Yes | Mention |
| negative TTL validation | Yes | N/A | Yes | Mention |
| loader success caches value | Yes | N/A | Yes | Yes |
| loader error not cached | Yes | N/A | Yes | Mention |
| canceled loader not cached | Yes | `AsyncJobTester` | Yes | Mention |
| same-key duplicate load suppression | Yes | `GoroutineStressTester` | Yes | Yes |
| different-key independent loads | Yes | Focused concurrency | Yes | Mention |
| flight-key collision resistance | Yes | N/A | Yes | N/A |

## Verification Commands

Run from the feature worktree:

```bash
gofmt -w cache
go test -count=1 ./cache
go test -race -count=1 ./cache
git diff --check
make ci
```

Use `rtk` prefixes for shell execution in this environment:

```bash
rtk proxy gofmt -w cache
rtk test go test -count=1 ./cache
rtk test go test -race -count=1 ./cache
rtk git diff --check
rtk test make ci
```

## Rollback And Scope Control

- Removing the `cache` directory and README additions reverts #22 without
  affecting Redis leader/resilience packages.
- Do not touch `go.mod` unless implementation unexpectedly proves the existing
  direct `golang.org/x/sync` dependency is missing or version-incompatible.
- Do not add Redis/Testcontainers work in #22; create/follow #23 for near-cache.

## Step 3 Checklist Completion Report

| Item | Status | Notes |
|---|---|---|
| Spec requirements mapped | Done | All issue criteria and Step 2-R fixes map to T1-T14. |
| Task order implementable | Done | API and memory behavior precede tests/docs; verification follows implementation. |
| Hidden subtasks split out | Done | Flight-key collision and loader-lock behavior have explicit tasks. |
| Stress/cancellation tests assigned | Done | `GoroutineStressTester` and `AsyncJobTester` tasks are separate. |
| Docs locale pair assigned | Done | README English/Korean update in T10. |
| Verification commands concrete | Done | Targeted tests, race test, diff check, and `make ci` listed. |
