# Issue #535 Step 3-R Plan Review

> 한국어 리뷰 경계: 이 문서는 리뷰 판정과 근거를 한국어 독자가 추적할 수 있도록 정리한다. 심각도 토큰, 판정 토큰, 파일 경로, 라인 번호, 이슈/PR 번호, 명령, 코드 식별자는 원문의 증거 앵커로 보존한다.

이슈: #535

날짜: 2026-07-18

Spec: `docs/superpowers/specs/2026-07-18-issue-535-redis-tiered-value-cache-design.md`

Plan: `docs/superpowers/plans/2026-07-18-issue-535-redis-tiered-value-cache-plan.md`

Reviewed substantive SHA: `e0d394b06f750529ac0e2c773b83feec5b7e61a6`

게이트: 7-Tier = six independent perspectives plus main-session integration.

## Final Exact-Head Results

| Tier | Perspective | Verdict | P0 | P1 | P2 |
|---|---|---:|---:|---:|---:|
| 1 | Performance | APPROVE | 0 | 0 | 0 |
| 2 | Stability | MAIN FALLBACK | 0 | 0 | 0 |
| 3 | Security | APPROVE | 0 | 0 | 0 |
| 4 | Operator/Ops | APPROVE | 0 | 0 | 0 |
| 5 | Developer/API | APPROVE | 0 | 0 | 0 |
| 6 | User/Caller | APPROVE WITH COMMENT | 0 | 0 | 1 |
| Main | Integration | APPROVE | 0 | 0 | 1 |

All completed lanes reviewed the same substantive SHA. The stability lane did
not provide a result within the enforced wait limit despite three bounded
attempts: `lane timed out; main integration fallback performed`.

## Blocking Findings Closed During Review

| Area | Repair now present in the reviewed spec/plan |
|---|---|
| Different-key concurrency | Channel-latched L1, loader, and serializer tests reject package-global serialization. |
| Serializer trust and concurrency | The caller-owned serializer is immutable and concurrency-safe; hostile payload, redaction, deterministic overlap, and race evidence are explicit. |
| Public invalidation budget | Token wait uses `InvalidationWaitTimeout`; maintenance delete uses the smaller child `LocalCleanupTimeout` budget. |
| Raw-key leakage | Redis commands receive `Key.Value`; error helpers receive only `Key.RedactedID`. |
| TDD dependency order | `ValueCache` completes its interface in Tasks 2-3; `TieredCache` completes and asserts `LoadingCache` only in Task 8. |
| Zero-value coverage | `SetDefault`, `Clear`, and every tiered public method are assigned to focused zero-value tests after the method exists. |
| Operator evidence | Go 1.26.3, mixed-version behavior, namespace-reuse restrictions, fleet reset, and executable locale parity are explicit. |
| Documentation trust boundary | Both locales cover untrusted bytes, authenticated envelopes, decoder limits, namespace/ACL scope, SCAN page limits, and serializer concurrency. |

## Stability Main-Session Fallback

The main session reviewed coordinator identity/retirement, token waiters,
flight publication/cancellation arbitration, participant release, local-state
leases, one-shot tickets, generations, repair epochs, same-key linearization,
caller versus mandatory cleanup contexts, zero-value safety, and task-level
RED/GREEN command coverage.

The plan requires deterministic channel latches for terminal races, repeated
tests, package race tests, coordinator registry-zero assertions, no background
goroutine that outlives a method, and the repository `make race` gate. No
remaining P0/P1 stability issue was found.

## Accepted P2

`ClearProgress.ScannedKeys` can be read as a total Redis-keyspace progress
counter even though it records only namespace-matching keys returned so far.
It is not a completion percentage or resumable cursor. The public field remains
as approved; Step 6-R must confirm that its GoDoc and both package READMEs state
this limitation without promising total progress.

## 검증 증거

- `git diff --check origin/develop...e0d394b06f750529ac0e2c773b83feec5b7e61a6` — PASS.
- Plan/spec placeholder scan — PASS; only the historical checklist phrase
  `No placeholders` remains.
- Markdown fence parity — plan 130 fences, spec 16 fences; both even.
- Final performance, security, operator/Ops, and developer/API lanes:
  `P0=0/P1=0/P2=0`.
- Final user/caller lane: `P0=0/P1=0/P2=1`, accepted above.
- No implementation test was run at Step 3-R because implementation has not
  started.

## 메인 통합 판정

APPROVE.

- P0 = 0
- P1 = 0
- Accepted P2 = 1
- Stop condition reached: approved spec and executable plan are committed;
  implementation remains untouched and PR authority has not been exercised.

## DoD

| 항목 | 상태 |
|---|---|
| Six perspectives covered | Done; stability used the documented main fallback after timeout. |
| Same substantive SHA reviewed | Done; `e0d394b06f750529ac0e2c773b83feec5b7e61a6`. |
| Main integration review completed | Done. |
| P0/P1 normalized | Done. |
| Accepted P2 recorded | Done. |
| Plan ready for implementation handoff | Done. |
