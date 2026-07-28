# Issue #205 Step 3-R Plan Review

> 한국어 리뷰 경계: 이 문서는 리뷰 판정과 근거를 한국어 독자가 추적할 수 있도록 정리한다. 심각도 토큰, 판정 토큰, 파일 경로, 라인 번호, 이슈/PR 번호, 명령, 코드 식별자는 원문의 증거 앵커로 보존한다.

## 범위

- Plan: `docs/superpowers/plans/2026-06-21-issue-205-foundation-hardening-plan.md`
- Spec: `docs/superpowers/specs/2026-06-21-issue-205-foundation-hardening-design.md`
- Issue: #205 `audit: Harden existing core collections and codec contracts`
- Milestone: `0.6.3`

## Native Lane Status

Six Step 3-R perspectives were started as native review lanes. Several lanes
completed with findings; the operator/Ops lane and later close/wait cleanup
stalled beyond the bounded workflow SLA. Per the workflow fallback rule, this
artifact records the completed lane findings and the current-session local
integration fallback instead of waiting further.

| Perspective | Result |
|---|---|
| Performance | P0=0 P1=0; bounded `make ci`, race scope, and UTF-8 validation cost notes reviewed. |
| Stability | P0=0 P1=1 initially; empty non-nil and nil callback precedence coverage required. |
| Security | P0=0 P1=1 initially; no-error `Encode*String` behavior needed explicit non-validating contract and malformed-input sentinel separation. |
| Operator/Ops | Lane timed out; main integration fallback performed. No release, workflow, Docker, or runtime operation scope added by the plan. |
| Developer/API | P0=0 P1=1 initially; shared sentinel/API decision and Go doc requirements needed to be explicit. |
| User/Caller | P0=0 P1=2 initially; README/example migration path and URL62/Base64URL binary alternatives needed to be explicit. |

## Current-Session Integration Review

| Priority | Area | Finding | Resolution |
|---|---|---|---|
| P1 | Stability | Plan omitted empty non-nil serializer behavior and nil callback precedence coverage. | Resolved. Task 3 includes empty non-nil serializer tests and nil input negative sentinel checks. Task 4 includes nil/empty and nil callback precedence tests for slice/map helpers. |
| P1 | Security | Plan could make malformed codec errors indistinguishable from invalid UTF-8 text errors. | Resolved. Task 2 includes malformed Base64URL, invalid hex, and malformed string decoder tests that assert errors do not wrap `core.ErrInvalidUTF8`. |
| P1 | Security/API | Plan did not clearly state no-error `Encode*String` helpers cannot validate invalid UTF-8. | Resolved. API Decision and Task 5 require docs for no-error string encoders as non-validating string-to-byte convenience wrappers. |
| P1 | Developer/API | Plan did not make the shared sentinel ownership and dependency direction explicit enough. | Resolved. API Decision defines `core.ErrInvalidUTF8`, intentional `codec`/`serialization -> core` dependency, and import-cycle verification via `go list -deps`. |
| P1 | User/Caller | Plan lacked concrete migration examples for callers. | Resolved. Task 5 adds examples using `errors.Is(err, core.ErrInvalidUTF8)` and byte-helper/`BytesSerializer` fallback. |
| P1 | User/Caller | Plan did not explicitly include URL62/Base64URL binary alternatives. | Resolved. Task 2 binary decoder tests include `DecodeURL62` and `DecodeBase64URL`; Task 5 README checks require both names. |

## 체크리스트

| Check | Status | Evidence |
|---|---|---|
| Six perspectives completed or fallback recorded | PASS | Completed findings plus operator timeout fallback recorded above. |
| Spec acceptance criteria map to plan tasks | PASS | Plan Tasks 1-6 cover text contracts, binary contracts, collections/core regression tests, README/examples, and verification. |
| Task ordering implementable | PASS | RED tests precede production changes per package; docs follow proven behavior; final verification follows implementation. |
| Docs and README impact assigned | PASS | Task 5 covers Go doc, examples, and English/Korean README updates. |
| Dependency/source checks assigned | PASS | API Decision and Task 6 require `go list -deps ./codec ./serialization` to confirm the intentional `core` dependency. |
| P0/P1 convergence | PASS | All initial P1 items have required plan edits; latest integrated table is P0=0 P1=0. |

## 판정

Step 3-R PASS.

P0=0 P1=0
