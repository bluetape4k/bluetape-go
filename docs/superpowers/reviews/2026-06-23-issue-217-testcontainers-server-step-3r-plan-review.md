# Issue #217 Step 3-R Plan Review

> 한국어 리뷰 경계: 이 문서는 리뷰 판정과 근거를 한국어 독자가 추적할 수 있도록 정리한다. 심각도 토큰, 판정 토큰, 파일 경로, 라인 번호, 이슈/PR 번호, 명령, 코드 식별자는 원문의 증거 앵커로 보존한다.

이슈: [#217](https://github.com/bluetape4k/bluetape-go/issues/217)
Spec: `docs/superpowers/specs/2026-06-23-issue-217-testcontainers-server-design.md`
Plan: `docs/superpowers/plans/2026-06-23-issue-217-testcontainers-server-plan.md`
날짜: 2026-06-23

## 범위

Reviewed implementation ordering, test coverage, wrapper lifecycle, docs
coverage, and final validation gates for the #217 server abstraction.

Subagent lanes were treated as local independent perspectives for this review
iteration to keep the issue moving while preserving the six-lane review shape.

## 6개 관점 발견 사항

| Tier | Perspective | Verdict | Findings |
|---|---|---:|---|
| 1 | Performance | Pass | Added adapter methods are thin Testcontainers delegations. Plan does not add polling loops or hot-path allocations beyond map cloning in test setup code. |
| 2 | Stability | Pass after edit | Plan now handles the container-started/server-construction-failed path by terminating the container before failing the test. Serial Docker validation remains explicit. |
| 3 | Security | Pass | Env export is opt-in, validates before mutation, and uses test-scoped `testing.TB.Setenv`. No secrets are logged by the planned code. |
| 4 | Operator/Ops | Pass | Dynamic mapped-port behavior and fixed-port collision risks are assigned to both English and Korean READMEs. |
| 5 | Developer/API | Pass | Task order is implementable: server package tests first, wrapper adaptation second, docs third, validation/review fourth. |
| 6 | User/Caller | Pass | Existing `Start` callers stay compatible; new `StartServer` is documented for generic server use. |

## 종합 발견 사항

| Priority | Area | Finding | Required plan edit |
|---|---|---|---|
| P1 | Stability | Wrapper plan could leak a started container if `server.New` failed before cleanup registration. | Resolved: Task 2 now requires immediate `testcleanup.Terminate` on construction failure before `tb.Fatalf`. |
| P2 | Tests | Existing `Start` behavior is indirectly covered once smoke tests switch to `StartServer`. | Keep `Start` as a thin call through the same detail extraction helper; avoid duplicate Docker starts. |
| P3 | Docs | README locale drift remains possible across ten files. | Task 3 requires mirrored English/Korean sections and `git diff --check`. |

## 통합 판정

P0: 0
P1: 0
P2: 1
P3: 1

The plan is implementation-ready. The P2/P3 items are tracked by implementation
and documentation tasks and do not block Step 4.
