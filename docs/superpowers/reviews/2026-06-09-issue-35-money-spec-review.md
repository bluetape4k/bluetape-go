# Issue #35 Money Spec Review

> 한국어 리뷰 경계: 이 문서는 리뷰 판정과 근거를 한국어 독자가 추적할 수 있도록 정리한다. 심각도 토큰, 판정 토큰, 파일 경로, 라인 번호, 이슈/PR 번호, 명령, 코드 식별자는 원문의 증거 앵커로 보존한다.

## 범위

- Spec: `docs/superpowers/specs/2026-06-09-issue-35-money-decimal-spec.md`
- Gate: Step 2-R, 7-Tier spec/design review
- Baseline: `58bccab Add JWT helper utilities`
- Worktree: `.worktrees/issue-35-money`

## 검토자 관점

| Lane | Result | P0 | P1 | P2 | P3 | Evidence |
|---|---|---:|---:|---:|---:|---|
| Developer/API + structural | FAIL -> PASS after rerun | 0 | 0 | 0 | 0 | Initial P1s on scalar arithmetic and `ExchangeRate{}` zero-value mapping were fixed; rerun cited spec lines for `Mul`/`Quo`, `ExchangeRate{}` validation, locale, `Sum`, and float special values. |
| Security + dependency | PASS | 0 | 0 | 2 | 1 | P2s on maintenance evidence and parse/unmarshal size boundary were fixed in the spec. |
| Ops/SRE + performance/stability | PASS | 0 | 0 | 1 | 1 | P2 on validation commands and P3 on follow-up issue ordering were fixed in the task list. |
| Testability/types/silent failure | FAIL -> PASS after rerun | 0 | 0 | 0 | 0 | Initial P1s on exchange-rate tests and `Sum` tests were fixed; rerun verified direct/reverse/invalid/mismatch/overflow and `Sum` empty/success/mixed/zero-value cases. |
| Library-user/docs/release | PASS | 0 | 0 | 1 | 1 | P2 on runnable examples and P3 on follow-up issue metadata were fixed in the spec. |

## Integrated 7-Tier Findings

| Tier | P0 | P1 | P2/P3 disposition |
|---|---:|---:|---|
| 1 Security | 0 | 0 | Parse/unmarshal boundary documented; request/body limits remain caller-owned. |
| 2 Ops/SRE reliability | 0 | 0 | Provider-backed exchange rates remain deferred to context-aware follow-up. |
| 3 Structural impact | 0 | 0 | Wrapper API retained; scalar arithmetic and zero-value behavior clarified. |
| 4 Go API quality | 0 | 0 | `Mul`/`Quo` string scalar API avoids direct public decimal dependency for #35. |
| 5 Tests/types/silent failure | 0 | 0 | Exchange-rate, `Sum`, locale, float special value, serialization, bounded parse/unmarshal, stress, and race tests are required. |
| 6 Performance/stability | 0 | 0 | `GoroutineStressTester`, `go test -race`, and validation command set are required; `AsyncJobTester` is N/A unless context-aware API is added. |
| 7 Docs/release/evidence | 0 | 0 | README pairs, package docs, examples, CHANGELOG, WIP, follow-up issue metadata, and dependency research preservation are required. |

## Required Spec Edits Applied

- Added dependency maintenance evidence for all compared candidates.
- Added `ExchangeRate{}` zero-value and wrapper-level validation semantics.
- Clarified `Mul(factor string)` and `Quo(divisor string)` scalar API.
- Added `Zero(currency)`, `Sum` empty-input behavior, locale normalization scope, float NaN/Inf rejection, bounded parse/unmarshal tests, and request-size caller boundary.
- Added exchange-rate direct/reverse/invalid/mismatch/overflow tests.
- Added runnable `money_example_test.go` requirement.
- Moved follow-up issue creation before README linking and specified follow-up metadata.
- Added validation commands: `go mod tidy`, `git diff --check`, `golangci-lint config verify`, targeted test, race test, and `make ci`.

## 수렴 판정

P0=0 P1=0. Step 2-R PASS.
