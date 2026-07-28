# Issue 27 Workflow Runners Pre-Implementation Risk

> 한국어 리뷰 경계: 이 문서는 리뷰 판정과 근거를 한국어 독자가 추적할 수 있도록 정리한다. 심각도 토큰, 판정 토큰, 파일 경로, 라인 번호, 이슈/PR 번호, 명령, 코드 식별자는 원문의 증거 앵커로 보존한다.

이슈: #27
게이트: Step 3-P / T0
상태: PASS

## 검토한 증거

- `docs/superpowers/specs/2026-06-06-issue-27-workflow-runners-spec.md`
- `docs/superpowers/plans/2026-06-06-issue-27-workflow-runners-plan.md`
- `docs/superpowers/specs/2026-06-05-issue-135-0.4.0-state-workflow-spec.md`
- `docs/superpowers/plans/2026-06-05-issue-135-0.4.0-state-workflow-plan.md`
- `workreport/*`
- `testing/concurrency/*`

## 위험

| Risk | Severity | Mitigation fed into implementation |
|---|---|---|
| Parallel runner could leak goroutines when one child stops early. | P1 if missed | Use derived cancellable context, cancel on early-stop child reports, and always wait for started goroutines. |
| Result ordering could become nondeterministic under parallel execution. | P1 if missed | Preallocate one report slot per work item and write each child report by input index. |
| `StopOnFailure` could conflict with `workreport.Aggregate` by collecting too many children. | P1 if missed | Sequential stops before later work; parallel may still include already-started siblings, but cancels them and aggregates collected child reports. |
| Unknown failure policies could silently return success. | P1 if missed | Convert `workreport.Aggregate` errors into failed reports and test `errors.Is(report.Err, workreport.ErrUnknownFailurePolicy)`. |
| Nil work or nil predicate could panic. | P1 if missed | Return failed reports with package sentinel errors before invocation. |
| Predicate cancellation could be reported as a generic failure. | P2 | Treat `context.Canceled` and `context.DeadlineExceeded` as cancelled reports. |
| Mutable shared context map could creep into examples. | P2 | Keep examples closure-based and document no mutable `WorkContext` in README. |

## 판정

P0=0 P1=0. Step 4 implementation is unblocked after the spec/plan commit.
