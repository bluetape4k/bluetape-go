# Issue 569 Redis Foundation Spec Review

Date: 2026-07-10 KST
Gate: Step 2-R
Spec: `docs/superpowers/specs/2026-07-10-issue-569-redis-foundation-spec.md`
Branch: `feat/issue-569-redis-foundation`

## Reviewed Scope

- New public `redis` package design with package name `btredis`.
- Key builder, hash-tag, owner-token, lease, script helper, TTL, and error
  contracts.
- Test, documentation, release evidence, and migration boundaries.

## Evidence

- `gh issue view 569`: issue is open, assigned to `debop`, milestone `0.19.0`.
- Worktree `.worktrees/issue-569-redis-foundation` from `develop`.
- Baseline `make ci`: PASS on the worktree before spec edits.
- Current source evidence:
  - `lock/redis/mutex.go`
  - `leader/redis/elector.go`
  - `ratelimit/redis/limiter.go`
  - `probabilistic/redis/keys.go`
  - `testcontainers/redis/redis.go`
- Lesson evidence:
  - `docs/lessons/2026-06-04-issue-24-redis-distributed-lock.md`
  - `docs/lessons/2026-07-08-issue-412-redis-testcontainers.md`
  - `docs/lessons/2026-07-09-issue-437-jwt-redis-contention.md`
- External evidence:
  - Redis `SET`, `EVAL`, and Cluster hash-tag documentation.
  - Local `go doc github.com/redis/go-redis/v9.Script.Run`.
- Hygiene:
  - placeholder scan: PASS, no matches.
  - `git diff --check`: PASS.

## Initial Review Findings

| Tier | Perspective | Initial P0 | Initial P1 | Main finding |
|---|---|---:|---:|---|
| 1 | Performance | 0 | 1 | Missing Redis-backed interleaved-owner stress under race. |
| 2 | Stability | 0 | 1 | Post-dispatch cancellation side effect was unspecified. |
| 3 | Security | 0 | 1 | Owner-token entropy, length, and canonical encoding were underspecified. |
| 4 | Operator/Ops | 0 | 1 | Nil context normalized to background for external Redis IO. |
| 5 | Developer/API | 0 | 2 | Mutable `Lease` and mixed structural/logical key variadic API. |
| 6 | User/Caller | 0 | 0 | README examples and token logging guidance needed stronger acceptance. |

## Spec Revisions Applied

- Required bounded Redis-backed interleaved-owner stress and race validation for
  lease script helpers.
- Documented post-dispatch cancellation as an indeterminate commit-state case.
- Rejected nil contexts for public script helpers and required caller-owned
  timeout/deadline examples.
- Made `OwnerToken` opaque, 256-bit, lowercase-hex canonical, and redacted by
  default `String()`.
- Added sensitive `RedisValue()` guidance and tests that prevent token/key
  leakage.
- Made `Lease` immutable with accessor methods and required `lease.Validate()`
  before Redis IO.
- Split key construction into structural segments, `StructuralKey`, and one
  verbatim `LogicalKey` terminal.
- Added key-format parity requirements before #570 migrations.
- Added package-level `redis.NewScript` reuse requirement.
- Added `NewOpError` / `NewOpErrorWithRedactedKey` construction contracts.
- Added invalid TTL no-Redis-dispatch proof with fake `redis.Scripter`.
- Strengthened README parity, operator runbook, release metadata, and changelog
  acceptance criteria.

## Rerun Results

| Tier | Perspective | Rerun P0 | Rerun P1 | Notes |
|---|---|---:|---:|---|
| 1 | Performance | 0 | 0 | Prior stress, benchmark-precondition, script allocation, and key policy gaps closed. |
| 2 | Stability | 0 | 0 | Prior cancellation, invalid lease, and cleanup gaps closed. |
| 3 | Security | 0 | 0 | Prior token entropy/leakage and redaction gaps closed. |
| 4 | Operator/Ops | 0 | 0 | Prior nil context, key parity, hash-tag, runbook, and metadata gaps closed. |
| 5 | Developer/API | 0 | 0 | P2 suggestions on OpError construction, TTL ordering, and structural terminal were applied. |
| 6 | User/Caller | 0 | 0 | No P0/P1; P2/P3 guidance incorporated through docs/example acceptance. |

## Integrated Verdict

P0=0 P1=0

Step 2-R is closed. Step 3 implementation planning is unblocked.

## Follow-Up Checks For Later Gates

- Step 3 plan must include TDD tasks for token canonicality, redacted string
  behavior, key builder structural/logical separation, fake `redis.Scripter`
  no-dispatch tests, and Redis Testcontainers drift/stress coverage.
- Step 6-R code review must verify no existing Redis package imports the new
  foundation package in #569.
- PR body must end with `## DoD Status`.
