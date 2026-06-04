# Issue 23 Redis Near Cache Plan Review

Issue: #23
Gate: Step 3-R
Date: 2026-06-04
Plan: `docs/superpowers/plans/2026-06-04-issue-23-redis-near-cache-plan.md`
Spec: `docs/superpowers/specs/2026-06-04-issue-23-redis-near-cache-spec.md`

## Perspective Findings

| Perspective | P0 | P1 | P2 | P3 | Notes |
|---|---:|---:|---:|---:|---|
| Implementer | 0 | 0 | 0 | 0 | Tasks are ordered scaffold -> message -> lifecycle -> behavior -> tests -> docs. |
| Test Engineer | 0 | 0 | 0 | 0 | Each public behavior has unit, Testcontainers, stress, or async coverage. |
| Architect | 0 | 0 | 0 | 0 | New package keeps Redis strategy isolated and avoids new local-cache dependency. |
| Delivery/Docs | 0 | 0 | 0 | 0 | README pair, CHANGELOG, lessons, and PR evidence are assigned. |

## Local 7-Tier Review

| Tier | P0 | P1 | P2 | P3 | Evidence |
|---|---:|---:|---:|---:|---|
| 1 Security | 0 | 0 | 0 | 0 | Plan includes malformed payload handling and no executable deserialization. |
| 2 Ops/SRE reliability | 0 | 0 | 0 | 0 | Plan includes ack-before-return, bounded retry, close, and receive-error clearing. |
| 3 Structural impact | 0 | 0 | 0 | 0 | `cache/redisnear` is a narrow public package and reuses `cache.Memory`. |
| 4 API quality | 0 | 0 | 0 | 0 | Constructor-per-strategy preserves future RESP3 boundary. |
| 5 Tests/types/silent failure | 0 | 0 | 0 | 0 | Test matrix covers success, failure, lifecycle, Testcontainers, stress, and async. |
| 6 Performance/stability | 0 | 0 | 0 | 0 | Stress tests are mandatory; benchmarks are explicitly linked to #107. |
| 7 Docs/release/evidence | 0 | 0 | 0 | 0 | README/CHANGELOG/lesson/PR evidence tasks are included. |

## Required Plan Checks

| Check | Status | Evidence |
|---|---|---|
| Every spec requirement maps to a task | PASS | T1-T8 and Test Matrix cover spec sections. |
| Task ordering is implementable | PASS | Redis lifecycle tasks precede integration tests. |
| Concrete verification commands named | PASS | Package, race, full test, and diff commands listed. |
| Testcontainers serial execution recorded | PASS | Validation section explicitly says serial. |
| Resource lifecycle explicit | PASS | `NewPubSub`, subscriber loop, `Close`, and `ErrClosed` are named tasks/checks. |
| Stress/benchmark decision explicit | PASS | Stress in #23; benchmarks in #107. |

## Gate Verdict

P0 = 0
P1 = 0

Step 3-R is closed. Implementation may proceed after committing the spec, plan,
and review artifacts to the feature branch.
