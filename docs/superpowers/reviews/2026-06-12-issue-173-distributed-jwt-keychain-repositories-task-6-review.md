# Issue #173 Task 6 Review

Issue: #173
Task: Redis Atomic Rotate, Forced Rotate, Capacity, and TTL
Date: 2026-06-12

## Scope

- `jwt/redis_repository.go`
- `jwt/redis_scripts.go`
- `jwt/redis_repository_test.go`

## TDD Evidence

| Phase | Evidence | Status |
| --- | --- | --- |
| RED | `go test -p 1 -count=1 ./jwt -run 'TestRepository(Rotate|ForcedRotate|Capacity|.*KeyTTL|ConfiguredKeyTTL|RejectsKeyTTL)'` failed before implementation on CAS convergence, capacity trim, configured TTL, short TTL rejection, and rotate current-hit command budget. | PASS |
| GREEN | Same targeted command passed after implementation. | PASS |
| Regression | `go test -p 1 -count=1 ./jwt ./jwt/redis` passed. | PASS |
| Race | `go test -race -p 1 -count=1 ./jwt ./jwt/redis` passed. | PASS |
| Whitespace | `git diff --check` passed. | PASS |

## Redis Command Budget

| Path | Expected Redis Commands | Evidence | Status |
| --- | --- | --- | --- |
| `Rotate` current-hit | one Lua read phase, no `create` call | `TestRepositoryRotateCurrentHitCommandBudget` records exactly `["eval"]`; `create` fails the test if called. | PASS |
| `Rotate` empty/expired | Lua read phase, one provider `create`, Lua CAS store phase | `Rotate` implementation calls `currentPayload`, `createWithContext`, then `storeCAS`. | PASS |
| `ForcedRotate` | one provider `create`, one Lua store phase | `ForcedRotate` calls `createWithContext`, then `store`. | PASS |
| Scan/list/all-key avoidance | no caller-visible `SCAN`, `KEYS`, `LRANGE`, `HGETALL` path | Command-capture tests assert forbidden commands for hot read paths; rotate current-hit captures only `EVAL`. | PASS |

## Spec and Quality Review

| Requirement | Evidence | Status |
| --- | --- | --- |
| CAS converges concurrent empty rotate to one winner. | `TestRepositoryRotateCASReturnsConcurrentWinner` uses `GoroutineStressTester` and verifies both callers return the same `kid` with one stored key. | PASS |
| Current-hit rotate does not call `create`. | `TestRepositoryRotateReturnsCurrentWithoutCallingCreate`. | PASS |
| Empty rotate persists candidate and updates current pointer. | `TestRepositoryRotateStoresCandidateWhenNoCurrentKeyExists`. | PASS |
| Forced rotate always stores a new current candidate. | `TestRepositoryForcedRotateAlwaysStoresCandidate`. | PASS |
| Capacity trim preserves newest retained keys. | `TestRepositoryCapacityTrimPreservesNewestKeys`. | PASS |
| `KeyTTL == 0` leaves Redis state without expiration. | `TestRepositoryKeyTTLZeroLeavesKeysWithoutRedisExpiration`. | PASS |
| Configured `KeyTTL` applies to all repository state keys and retains non-expired keys. | `TestRepositoryConfiguredKeyTTLRetainsNonExpiredKeys`. | PASS |
| Short `KeyTTL` is rejected before persistence. | `TestRepositoryRejectsKeyTTLShorterThanRetainedKeyValidityAndRetentionLeeway`. | PASS |
| Cancellation after create does not persist candidates. | `TestRepositoryRotateCanceledAfterCreateDoesNotPersistCandidate`; `TestRepositoryForcedRotateCanceledAfterCreateDoesNotPersistCandidate`. | PASS |
| Redis scripts keep DTO material private to package `jwt`. | Scripts receive encoded package-private DTO payloads; `jwt/redis` remains an alias facade. | PASS |

## Verdict

P0=0 P1=0

Task 6 verdict: PASS
