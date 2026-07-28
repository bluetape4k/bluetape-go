# Issue #231 IMF Provider Step 6-R Review

> 한국어 감사/리뷰 경계: 이 문서는 리뷰 결론과 남은 위험을 한국어 독자가 추적할 수 있도록 정리한다. 심각도 표기, 판정 표기, 파일 경로, 라인 번호, 이슈/PR 링크, 명령, 코드 식별자, 인용 증거는 원문의 증거 앵커로 보존한다.

범위: `money` IMF exchange-rate provider implementation, tests, README pair,
research note, WIP, and changelog updates.

Baseline: `develop` at `b83d8b3`.

## 게이트 결과

P0=0 P1=0

Final verdict: PASS.

## 관점별 결과

| Lane | P0 | P1 | P2 | P3 | Verdict | Evidence |
|---|---:|---:|---:|---:|---|---|
| Performance/runtime | 0 | 0 | 1 | 0 | PASS | Added IMF goroutine stress tests for concurrent fresh/stale paths and post-refresh stale-age regression. Duplicate per-key refresh remains a non-blocking P2 because request coalescing is not a current provider contract. |
| Stability/correctness | 0 | 0 | 0 | 0 | PASS | Fixed context stale fallback, SDMX series attribute validation, dedicated EUR direct/reverse fixtures, and missing-observation cases. |
| Security | 0 | 0 | 0 | 0 | PASS | Added bounded success body reads, bounded HTTP error diagnostics, IMF country-code validation, constrained path components, and retry classification. |
| Operator/Ops | 0 | 0 | 0 | 0 | PASS | Retry semantics, stale fallback behavior, diagnostics, timeout/cancellation, and failure mapping are tested and documented. |
| Developer/API | 0 | 0 | 0 | 0 | PASS | Public API remains a narrow `ExchangeRateProvider`; `Convert` value path is unchanged. Options document family/frequency/window/country-code semantics. |
| User/caller docs | 0 | 0 | 0 | 0 | PASS | README pair documents IMF reference-data caveat, USD/EUR scope, default country-code map, fixed period window options, retry behavior, and XDR deferral. |

## 수정한 발견 사항

- P1: Unbounded IMF HTTP response handling could exhaust memory. Fixed with a
  4 MiB success-body cap, bounded sanitized error diagnostics, and oversized
  response tests.
- P1: Caller cancellation/deadline could be hidden behind stale fallback. Fixed
  by returning context errors before stale fallback and adding stale-cache
  cancellation regression coverage.
- P1: IMF concurrent cache/stale-refresh paths lacked stress coverage. Fixed
  with `GoroutineStressTester` and `AsyncJobTester` coverage under `go test
  -race`.
- P1: SDMX response series attributes were not validated. Fixed by matching
  `COUNTRY`, `INDICATOR`, `TYPE_OF_TRANSFORMATION`, and `FREQUENCY` before
  consuming observations.
- P1: EUR tests relabeled USD fixtures. Fixed with dedicated `XDC_EUR` and
  `EUR_XDC` fixtures based on live IMF sample values.
- P2/P3 docs gaps: default domestic-currency map, period-window override, retry
  behavior, and source URLs are now documented.

## 검증

| Command / Review | Status | Evidence |
|---|---|---|
| `git diff --check` | PASS | Whitespace check clean. |
| `go test -count=1 ./money` | PASS | Package tests passed after P1 repairs. |
| `go test -race -count=1 ./money` | PASS | Race gate passed with IMF stress tests. |
| `make ci` | PASS | Full repo CI command completed successfully. |
| Security re-review | PASS | P0=0 P1=0 after bounded body, diagnostics, validation, and retry repairs. |
| Performance re-review | PASS | P0=0 P1=0; duplicate refresh recorded as non-blocking P2/non-goal. |
| Correctness re-review | PASS | P0=0 P1=0 after dedicated EUR fixtures and series validation. |

## 잔여 위험

IMF refresh coalescing is not implemented. Concurrent expired-cache callers may
each attempt a refresh, but stale fallback, locks, and race coverage are now
tested. Add per-key refresh coalescing only if outbound request cardinality
becomes a documented provider contract.
