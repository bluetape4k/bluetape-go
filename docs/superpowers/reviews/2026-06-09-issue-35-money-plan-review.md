# Issue #35 Money Plan Review

> 한국어 리뷰 경계: 이 문서는 리뷰 판정과 근거를 한국어 독자가 추적할 수 있도록 정리한다. 심각도 토큰, 판정 토큰, 파일 경로, 라인 번호, 이슈/PR 번호, 명령, 코드 식별자는 원문의 증거 앵커로 보존한다.

## 범위

- Plan: `docs/superpowers/plans/2026-06-09-issue-35-money-decimal-plan.md`
- Spec: `docs/superpowers/specs/2026-06-09-issue-35-money-decimal-spec.md`
- Gate: Step 3-R, implementation plan review
- Baseline: `58bccab Add JWT helper utilities`

## 검토자 관점

| Lane | Result | P0 | P1 | P2 | P3 | Evidence |
|---|---|---:|---:|---:|---:|---|
| Implementer + architecture | FAIL -> PASS after rerun | 0 | 0 | 0 | 0 | Initial P1s on no-currency rejection, constructor semantics, and marshal/unmarshal destination behavior were fixed and rerun passed. |
| Test engineer + verifier | FAIL -> PASS after rerun | 0 | 0 | 0 | 0 | Initial P1s on `IsCurrency`/`MustParseCurrency` and `Neg`/`Abs`/`Equal` tests were fixed and rerun passed. |
| Security/SRE/performance | PASS | 0 | 0 | 2 | 0 | P2s on dependency revalidation commands and explicit stress options were fixed in the plan. |

## 통합 발견 사항

| Priority | Area | Disposition |
|---|---|---|
| P1 | No-currency boundary | Fixed. Plan rejects `XXX`, `999`, `Currency{}`, and no-currency money/exchange-rate construction with `ErrInvalidCurrency`. |
| P1 | Constructor semantics | Fixed. `NewFromInt64` is major units and `NewMinor` is currency minor units, with USD/KRW/JPY examples and tests. |
| P1 | Serialization contract | Fixed. Marshal rejects invalid receivers; zero-value unmarshal destinations are allowed; nil `*Money` unmarshal receivers return `ErrInvalidMoney`. |
| P1 | Missing currency helper tests | Fixed. Plan names `IsCurrency` and `MustParseCurrency` tests. |
| P1 | Missing unary/equality tests | Fixed. Plan names `Neg`, `Abs`, and `Equal` tests. |
| P2 | Dependency revalidation | Fixed. Plan records `go list -m -json` and `gh repo view` checks for selected dependencies. |
| P2 | Stress options | Fixed. Plan requires explicit workers, rounds, timeout, completion, and failure assertions. |
| P2 | Weak `rg` gates | Fixed. Plan splits stress/N/A checks and per-file docs checks. |

## 7-Tier Plan Verdict

| Tier | P0 | P1 | Notes |
|---|---:|---:|---|
| 1 Security | 0 | 0 | Parse/unmarshal and dependency validation are planned. |
| 2 Ops/SRE reliability | 0 | 0 | Provider-backed exchange rates remain in context-aware follow-up. |
| 3 Structural impact | 0 | 0 | Package boundary and no-currency behavior are implementable. |
| 4 Go API quality | 0 | 0 | Constructor, scalar arithmetic, and zero-value contracts are explicit. |
| 5 Tests/types/silent failure | 0 | 0 | All reviewed public methods and edge cases map to concrete tests. |
| 6 Performance/stability | 0 | 0 | Explicit stress options, race test, and full `make ci` are planned. |
| 7 Docs/release/evidence | 0 | 0 | README pairs, changelog, WIP, follow-up issues, PR metadata, and DoD body are planned. |

## 수렴 판정

P0=0 P1=0. Step 3-R PASS.
