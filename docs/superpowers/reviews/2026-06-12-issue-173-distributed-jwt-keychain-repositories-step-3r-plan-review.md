# Issue #173 Step 3-R Plan Review

Issue: #173
Milestone: 0.6.1
Date: 2026-06-12

Plan: `docs/superpowers/plans/2026-06-12-issue-173-distributed-jwt-keychain-repositories-plan.md`
Spec: `docs/superpowers/specs/2026-06-12-issue-173-distributed-jwt-keychain-repositories-design.md`
Spec review: `docs/superpowers/reviews/2026-06-12-issue-173-distributed-jwt-keychain-repositories-step-2r-spec-review.md`

## Review Contract

Step 3-R used the required 7-Tier gate shape:

- Six independent native subagent lanes: performance, stability, security, operator/Ops, developer/API, and user/caller.
- One main-session integration review: deduplicate findings, normalize severity, verify plan edits, own documentation/release/evidence integrity, and close the gate only after `P0=0 P1=0`.
- No seventh integration subagent was spawned.

## Inputs

- Current plan artifact under `docs/superpowers/plans`.
- Step 2-R spec and spec review artifacts.
- `$bluetape4k-full-feature` Step 3-R references:
  - `references/step-3r-plan-review-perspectives.md`
  - `references/step-3r-plan-review.md`
- `$bluetape-go-patterns` plan-review checks for Go API, context, cancellation, race/stress, Testcontainers, and public documentation boundaries.

## Lane Results

| Tier | Perspective | Initial P0 | Initial P1 | Final P0 | Final P1 | Final verdict |
| --- | --- | ---: | ---: | ---: | ---: | --- |
| Tier 1 Performance | Redis hot path, command count, benchmarks, chart gate | 0 | 1 | 0 | 0 | PASS |
| Tier 2 Stability | Cancellation, deadline, TTL retention, Testcontainers stability | 0 | 2 | 0 | 0 | PASS |
| Tier 3 Security | Raw key exposure, algorithm boundary, namespace validation | 0 | 2 | 0 | 0 | PASS |
| Tier 4 Operator/Ops | Runbook, rollout, PR metadata, observability evidence | 0 | 0 | 0 | 0 | PASS |
| Tier 5 Developer/API | API shape, nil guards, task ordering, facade compile order | 0 | 2 | 0 | 0 | PASS |
| Tier 6 User/Caller | Misuse resistance, README/examples, unsupported capabilities | 0 | 1 | 0 | 0 | PASS |

## Consolidated Findings and Fixes

| Priority | Area | Finding | Plan change | Final status |
| --- | --- | --- | --- | --- |
| P1 | Performance | Redis command-budget proof was described but not enforced by tests. | Added command-capture tests, no-scan/list/all-key command checks, `BenchmarkRedisRepositoryRotateExpired`, benchmark budget, and chart publication gate. | Fixed and rerun. |
| P1 | Stability | Post-create cancellation needed proof that Redis candidates are not persisted after cancellation. | Added no-persist cancellation tests for `Rotate`, `ForcedRotate`, and constructor/bootstrap paths. | Fixed and rerun. |
| P1 | Stability | Testcontainers-backed Redis validation could run unsafely in parallel. | Added serial `go test -p 1` gates for Redis package tests, race tests, benchmarks, and final validation. | Fixed and rerun. |
| P1 | Security | Public raw-key repository helpers would expose signing material through stable API. | Moved Redis core storage and DTO reconstruction into package `jwt`; kept `jwt/redis` as a facade over `jwt.NewRedisRepository`; prohibited exported raw-key seed/import helpers. | Fixed and rerun. |
| P1 | Security | Algorithm-family mismatch checks needed implementation-level test coverage. | Added algorithm/family mismatch tests for stored Redis state and distributed constructor validation. | Fixed and rerun. |
| P1 | Developer/API | Constructor path needed explicit nil and typed-nil repository rejection before bootstrap. | Added `requireDistributedRepository(repo)` with typed-nil reflection guard and constructor tests. | Fixed and rerun. |
| P1 | Developer/API | Context cancellation around key creation needed an implementable helper. | Added `createWithContext(ctx, create)` wrapper and required pre/post-create cancellation checks. | Fixed and rerun. |
| P1 | Developer/API | `jwt/redis` facade was ordered before the package-`jwt` repository symbols existed. | Moved facade implementation and facade tests to the task that creates `jwt.RedisRepository` and `jwt.NewRedisRepository`. | Fixed and rerun. |
| P1 | User/Caller | Public raw-key helper path created caller-visible misuse risk. | Same package-`jwt` Redis core and facade-only `jwt/redis` design removed the public raw-key path. | Fixed and rerun. |
| P2 | TTL retention | Call-level parse leeway could not prove Redis TTL safety. | Added repository-level `RedisRepositoryOptions.RetentionLeeway` and TTL validation against retained key validity plus leeway. | Fixed in plan. |
| P2 | Operator/Ops | README and PR tasks needed explicit Redis operations guidance and metadata. | Added TLS/ACL/persistence/noeviction diagnostics, safe `redis-cli` inspection guidance, rollback notes, `--assignee debop`, and label edits. | Fixed in plan. |
| P2 | User/Caller | Docs needed explicit unsupported capabilities and realistic examples. | Added README verification for MongoDB #198 deferral, unsupported migration/raw-key import boundaries, and HMAC/RSA examples. | Fixed in plan. |

## Rerun Evidence

| Perspective | Latest blocker status | Evidence |
| --- | --- | --- |
| Performance | `P0=0 P1=0` | Affected rerun confirmed command-capture tests, expired-rotate benchmark, benchmark budget, and chart gate. |
| Stability | `P0=0 P1=0` | Affected rerun confirmed no-persist cancellation coverage, serial `-p 1` Testcontainers gates, and `RetentionLeeway` TTL contract. |
| Security | `P0=0 P1=0` | Affected rerun confirmed no public raw-key API, algorithm/family mismatch coverage, and namespace validation. |
| Operator/Ops | `P0=0 P1=0` | Initial lane had no blockers; P2 runbook and PR metadata edits were integrated. |
| Developer/API | `P0=0 P1=0` | Affected rerun confirmed typed-nil repository guard, `createWithContext`, facade ordering, and `RetentionLeeway` implementability. |
| User/Caller | `P0=0 P1=0` | Affected rerun confirmed no public raw-key helper, README unsupported-capability coverage, and RSA example coverage. |

## Main Integration Review

| Check | Result | Evidence |
| --- | --- | --- |
| Every spec acceptance maps to concrete tasks | PASS | Plan acceptance mapping covers distributed provider, Redis repository behavior, cross-instance parse, cancellation, docs, stress tests, benchmarks, and review gates. |
| Task ordering is implementable | PASS | Package-`jwt` Redis core precedes `jwt/redis` facade; tests precede implementation where TDD is required. |
| No task depends on later artifacts | PASS | Facade moved after core repository symbols; benchmark chart branch follows benchmark output generation. |
| Go API/context contract is concrete | PASS | `DistributedKeyChainRepository`, `requireContext`, `requireDistributedRepository`, and `createWithContext` are planned with nil, typed-nil, cancellation, and deadline tests. |
| Redis security boundary is preserved | PASS | Key material reconstruction stays inside package `jwt`; `jwt/redis` exposes aliases/facade only; docs treat Redis as signing authority. |
| Performance and stability evidence is concrete | PASS | Command-capture tests, benchmark budget, serial Redis Testcontainers commands, race gate, stress helper coverage, and chart gate are named. |
| Documentation and release readiness are covered | PASS | README pair, facade docs, examples, runbook, PR body, Step 6-R, and Step 7-R tasks are planned. |
| Open user questions | PASS | None. MongoDB remains backlog #198; Redis is in #173 scope. |

Material design note: the plan keeps Redis DTO encode/decode and key material reconstruction inside package `jwt`, while exposing package `jwt/redis` as a user-facing facade. This preserves the user-facing Redis import path without adding public raw-key import or seed APIs.

## Verification Commands

```bash
rg -n 'TBD|TODO|implement later|fill in details|Similar to|appropriate|add validation|handle edge cases|Write tests for the above|NewHMACKeyChainForRepository|NewRSAKeyChainForRepository|TestOptionsNormalizeKeyTTLRule|benchmem ./jwt/redis' docs/superpowers/plans/2026-06-12-issue-173-distributed-jwt-keychain-repositories-plan.md
git diff --check
```

Expected result: first command returns no matches; `git diff --check` passes.

## Gate Verdict

P0=0 P1=0

Gate verdict: PASS

Step 4 implementation is unblocked after this Step 3-R plan-review artifact and the plan are committed.

## Step 3-R Checklist Completion Report

| Item | Status | Notes |
| --- | --- | --- |
| Required Step 3-R references loaded | Done | `step-3r-plan-review-perspectives.md` and `step-3r-plan-review.md` were read before closure. |
| Six independent native subagent lanes used | Done | Performance, stability, security, operator/Ops, developer/API, and user/caller lanes ran independently. |
| Main integration review performed by the main session | Done | This artifact integrates lane results; no seventh integration subagent was used. |
| P0/P1 findings fixed and affected lanes rerun | Done | All affected lanes reran to `P0=0 P1=0`. |
| Plan updated for P2/P3 decisions that should not be deferred | Done | TTL retention, runbook, PR metadata, docs/examples, and benchmark chart gate are in the plan. |
| Placeholder and stale-design scan performed | Done | Stale raw-key helper and placeholder scan returned no matches. |
| `git diff --check` performed | Done | Whitespace check passed before review artifact creation and must pass again before commit. |
| Step 3-R verdict recorded | Done | `P0=0 P1=0`; gate verdict is PASS. |
