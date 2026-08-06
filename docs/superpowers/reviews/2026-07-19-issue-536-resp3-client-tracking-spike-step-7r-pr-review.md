# Issue #536 RESP3 Client Tracking Spike Step 7-R Pre-PR Review

> 한국어 리뷰 경계: 이 문서는 리뷰 판정과 근거를 한국어 독자가 추적할 수 있도록 정리한다. 심각도 토큰, 판정 토큰, 파일 경로, 라인 번호, 이슈/PR 번호, 명령, 코드 식별자는 원문의 증거 앵커로 보존한다.

이슈: #536

날짜: 2026-07-19

기준: `origin/develop` at `f4acaab1676ca4a989051a28f60f37ab147d87f9`

검토한 후보 SHA: `50a6ded6af26cdb4845eaf5fe2af2d4aa722cb61`

브랜치: `test/issue-536-resp3-client-tracking-spike`

게이트: six independent perspectives plus main-session integration.

## 검토 모드

This is a pre-PR readiness review. Live GitHub inspection found no PR for the
candidate branch, and the current authority does not permit push or PR
creation. Remote CI, human reviews, review threads, and exact remote PR-head
verification are therefore `N/A` at this gate, not passing evidence.

## 실시간 메타데이터 검사

| 항목 | 실시간 결과 |
|---|---|
| Issue | `#536 test: Spike RESP3 client-tracking near-cache invalidation` |
| Issue state | OPEN |
| Milestone | `0.19.0` |
| Assignee | `debop` |
| Labels | `type: task`, `area: cache`, `area: testing`, `priority: p2` |
| Base | `origin/develop` at `f4acaab1676ca4a989051a28f60f37ab147d87f9` |
| Merge base | `f4acaab1676ca4a989051a28f60f37ab147d87f9` |
| Branch PR | None |
| Remote CI/reviews/threads | N/A until a PR exists |

## 수렴 이력

The first Step 7-R pass reviewed
`05062e401f5a7e4c084aafa1a67edb1fe426a4c2`. Performance found one P2 in the
Step 6-R record: it described all six repeated cases as Docker-backed, while
the selector actually contains five Docker-backed cases and the unit-only
unregister case.

Commit `50a6ded6af26cdb4845eaf5fe2af2d4aa722cb61` corrected the ledger without
changing code. Performance, stability, and security refreshed their reviews on
that SHA, and the remaining three perspectives reviewed the same repaired
candidate directly.

No lane timed out, and main-session fallback was not required.

## 최종 정확한 HEAD 결과

| 계층 | 관점 | 판정 | P0 | P1 | P2 | P3 |
|---|---|---:|---:|---:|---:|---:|
| 1 | Performance | PASS | 0 | 0 | 0 | 0 |
| 2 | Stability | PASS | 0 | 0 | 0 | 0 |
| 3 | Security | PASS | 0 | 0 | 0 | 0 |
| 4 | Operator/Ops | PASS | 0 | 0 | 0 | 0 |
| 5 | Developer/API | PASS | 0 | 0 | 0 | 0 |
| 6 | User/Caller | PASS | 0 | 0 | 0 | 0 |
| Main | Integration | PASS | 0 | 0 | 0 | 0 |

Every terminal lane reviewed
`50a6ded6af26cdb4845eaf5fe2af2d4aa722cb61` against
`f4acaab1676ca4a989051a28f60f37ab147d87f9`.

## 후보 범위

- One external-package RESP3 spike test file.
- One indexed research evidence ledger.
- Design, test specification, execution plan, Step 2-R, Step 3-R, and Step 6-R
  decision records.
- English and Korean research-index entries.
- No production Go file, public API, dependency, package README, root README,
  or changelog change.

The candidate proves explicit command-coupled invalidation delivery, physical
connection affinity, global invalidation, reconnect loss and recovery
requirements, unregister semantics, and bounded shutdown. It rejects an
autonomous coherent pooled RESP3 near-cache on the tested public go-redis
surface.

## 수용한 범위 경계

- Production remains on `redisnear.NewPubSub`.
- L1 stores `V` directly as a reference object; only Redis L2 performs
  serialization. The string-based spike does not claim mutable-reference
  aliasing proof.
- Context-free close watchdogs detect and fail a stuck close but cannot cancel
  it.
- RESP2, AUTH, TLS, Sentinel, Cluster, proxies, managed providers, and
  `REDIRECT` remain unproven and are not implied supported.
- Issue #560 owns performance and provider-comparison measurements.
- The implementation plan retains prospective checklist syntax; the completed
  local execution state is recorded by the Step 6-R DoD and this review.

These boundaries are explicit evidence limits, not unresolved production
defects, because no production RESP3 API is introduced.

## 최신 검증 증거

On candidate `50a6ded6af26cdb4845eaf5fe2af2d4aa722cb61`:

- `make ci` — PASS with exit code 0.
- `make lint` within CI — PASS, `0 issues`.
- Repository normal tests within CI — PASS.
- Repository race tests within CI — PASS.
- Testcontainers-backed packages within CI — PASS.
- Complete `^TestRESP3TrackingSpike` suite — PASS.
- Six selected cases with `-p 1 -count=3` — PASS: five Docker-backed cases
  plus the unit-only unregister case.
- Two-callback gate test with `-count=200` — PASS.
- `git diff --check origin/develop...50a6ded6af26cdb4845eaf5fe2af2d4aa722cb61`
  — PASS.
- Exact base, merge base, candidate SHA, and clean working tree — confirmed.

## 메인 통합 판정

PASS for local PR readiness.

- P0 = 0
- P1 = 0
- P2 = 0
- P3 = 0
- The issue goal is satisfied as a prove-or-reject spike: command-coupled RESP3
  delivery is proven, while autonomous coherent near-cache adoption is
  rejected with explicit lifecycle evidence.
- The evidence note is the durable reusable learning; lesson artifact is N/A
  to avoid duplication.
- The candidate is locally ready to push and open as a PR.
- It is not merge-ready: no PR, remote CI, human review, or review-thread state
  exists yet.
- Push and PR creation were not performed because the required target/base/head
  publication authority was not granted.

## DoD

| 항목 | 상태 |
|---|---|
| Live issue/milestone/base state refreshed | Done. |
| Branch PR absence confirmed | Done. |
| Six independent perspectives covered | Done. |
| Same exact repaired candidate SHA reviewed | Done: `50a6ded6af26cdb4845eaf5fe2af2d4aa722cb61`. |
| Main integration review completed | Done. |
| P0/P1 normalized | Done: `P0=0 P1=0`. |
| P2/P3 normalized | Done: `P2=0 P3=0`. |
| Full local CI evidence | Done: `make ci` PASS. |
| Production/API/dependency changes | None. |
| Remote CI/reviews/threads | N/A; no PR exists. |
| Push or PR side effect | Not performed; authority gate preserved. |
