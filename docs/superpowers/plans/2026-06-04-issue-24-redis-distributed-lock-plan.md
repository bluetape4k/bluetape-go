# Issue 24 Redis Distributed Lock Implementation Plan

Issue: #24
Milestone: 0.3.0
Date: 2026-06-04
Spec: `docs/superpowers/specs/2026-06-04-issue-24-redis-distributed-lock-spec.md`

## Objective

Implement a small single-Redis-instance distributed lock package with owner
tokens, TTL acquisition, owner-safe unlock, Testcontainers proof, stress tests,
and documentation.

## Task Plan

| Task | Scope | Details | Validation |
|---|---|---|---|
| T1 Package scaffold | `lock/redis` | Add `doc.go`, `options.go`, `errors.go`, and `mutex.go` with package name `redislock`. | `go test -count=1 ./lock/redis` compile |
| T2 Acquire path | `lock/redis` | Validate options, generate random token when needed, call `SetNX`, return `ErrNotAcquired` on contention, preserve context errors. | Tests for acquire, contention, invalid options, canceled context. |
| T3 Owner-safe unlock | `lock/redis` | Add Lua compare-and-delete script; `Lease.Unlock` returns true only when it deletes its own key and false when expired/lost. | Tests for owner unlock, non-owner safety, expired lease unlock. |
| T4 TTL expiration | `lock/redis` tests | Prove Redis TTL is set and another owner can acquire after expiration. | Testcontainers test with short TTL and `Eventually`. |
| T5 Stress/cancellation | `lock/redis` tests | Use `GoroutineStressTester` for same-key contention and `AsyncJobTester` for canceled attempts. | `go test -count=1 ./lock/redis -run 'Stress|Async|Cancellation'` |
| T6 Examples/docs | `lock/redis`, README pair, CHANGELOG, research index | Add compile-checked example, README.md/README.ko.md package row and section, CHANGELOG Unreleased entry, research links. | `go test -count=1 ./lock/redis -run Example`; targeted `rg`. |
| T7 Verification/review/lessons | docs | Add implementation review and lessons, run targeted tests/race/diff/GNO. | Review P0=0/P1=0; lessons committed. |

## Test Matrix

| Behavior | Test | Assertion |
|---|---|---|
| Missing client | unit | `New(nil, ...)` fails. |
| Missing key / bad TTL / blank token | unit | validation error before Redis command. |
| Acquire success | Testcontainers | `TryLock` returns lease, Redis key stores lease token, PTTL is positive. |
| Contention | Testcontainers | Second mutex gets `ErrNotAcquired`. |
| Owner unlock | Testcontainers | `Unlock` returns true and key is gone. |
| Non-owner safety | Testcontainers | Stale lease unlock returns false and current owner token remains. |
| Expiration | Testcontainers | After TTL expiry, another mutex acquires same key. |
| Context cancellation | Testcontainers/unit | canceled acquire/unlock preserve `context.Canceled` through `errors.Is`. |
| Stress | Testcontainers | concurrent contenders do not report more than one success per round. |
| Async cancellation | Testcontainers | canceled attempts complete and key is not leaked. |
| Example | compile test | example imports `lock/redis` as `redislock` and compiles. |

## Validation Commands

Run Testcontainers-backed commands serially.

```bash
go test -count=1 ./lock/redis
go test -race -count=1 ./lock/redis
go test -count=1 ./...
go test -race -count=1 ./...
go test -count=1 ./lock/redis -run Example
git diff --check
gno update
```

If full repo race is too slow after targeted race passes, record the gap and
wait for GitHub CI.

## Documentation Tasks

- README.md: package row and Redis distributed lock usage section.
- README.ko.md: synchronized Korean section.
- CHANGELOG.md: Unreleased added entry.
- `docs/research/README.md`: link #24 research.
- `docs/research/2026-06-01-milestone-0.3.0-cache-coordination-research.md`:
  name #24 lock decision.

## Step 3 Checklist Completion Report

| Item | Status | Notes |
|---|---|---|
| Every spec requirement mapped | Done | T1-T7 and test matrix cover owner token, TTL, owner-safe unlock, contention, expiration, cleanup, stress. |
| Task order implementable | Done | Scaffold -> acquire -> unlock -> tests -> docs -> review. |
| Testcontainers handled serially | Done | Validation commands explicitly run serially. |
| Verification commands concrete | Done | Targeted package, race, full repo, example, diff, GNO. |
| Public docs impact assigned | Done | README pair and CHANGELOG included. |
