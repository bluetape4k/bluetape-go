# Issue #33 JWT Helper Utilities Plan Review

> 한국어 리뷰 경계: 이 문서는 리뷰 판정과 근거를 한국어 독자가 추적할 수 있도록 정리한다. 심각도 토큰, 판정 토큰, 파일 경로, 라인 번호, 이슈/PR 번호, 명령, 코드 식별자는 원문의 증거 앵커로 보존한다.

Review type: Step 3-R plan review
Plan: `docs/superpowers/plans/2026-06-08-issue-33-jwt-helper-utilities-plan.md`
Spec: `docs/superpowers/specs/2026-06-08-issue-33-jwt-helper-utilities-spec.md`
이슈: #33
Milestone: 0.6.0
검토일: 2026-06-08

## 입력

- Step 2-R spec review passed with P0=0/P1=0.
- Follow-up issues are linked and scoped:
  - #173 distributed JWT KeyChain repositories.
  - #174 safe JWT compression and JOSE dependency scope.
  - #175 optional JWT provider cache adapters.
- Plan covers T0-T11 from dependency risk note through PR metadata, CI, and
  Step 7-R PR review.

## 서브에이전트 결과

| Reviewer | Role | Initial P0 | Initial P1 | Initial P2 | Initial P3 | Verdict |
| --- | --- | ---: | ---: | ---: | ---: | --- |
| Parfit | dependency-expert | 0 | 2 | 2 | 0 | Failed until inbound JOSE header parse rejection and fixed HMAC key strength were planned. |
| Averroes | architect | 0 | 0 | 1 | 1 | Passed, with split validation checks and exported-comment policy requested. |
| Euclid | test-engineer | 0 | 2 | 0 | 1 | Failed until deterministic reader-clock tests and lock-scope proof were planned. |
| Kepler | critic closure | 0 | 0 | 0 | 0 | Passed after P1/P2/P3 closure. |

## 차단 항목 종료

| Finding | Severity | Resolution |
| --- | --- | --- |
| Deterministic `IsExpired` and `RemainingTTL` tests were not mapped. | P1 | T4 now requires deterministic `IsExpired` and `RemainingTTL` assertions with fixed times. |
| Parse/signature verification lock scope was not mapped to proof. | P1 | T6 now requires a structural lock-scope note with code references; T10 includes lock-scope evidence in Step 6-R. |
| Inbound unsupported JOSE/compression headers were only rejected at compose time. | P1 | Spec and T4 now require parse rejection for signed tokens carrying `zip`, `crit`, `jku`, `jwk`, `x5u`, or `x5c`. |
| Fixed HMAC key strength was not specified or tested. | P1 | Spec and T2 now require HS256 >= 32 bytes, HS384 >= 48 bytes, HS512 >= 64 bytes, with empty/short secret tests using `ErrInvalidKey`. |

## 비차단 수정 적용

| Finding | Severity | Resolution |
| --- | --- | --- |
| `TryParse` was not mapped to tests. | P2 | T4 now requires `TryParse` success/failure coverage as a boolean wrapper over hardened `Parse`. |
| Fixed-provider missing-`kid` boundary was underspecified. | P2 | T5 now requires no-`kid` fixed-provider accept boundary and rotating-provider rejection tests. |
| Concurrency/N/A `rg` check used one alternation that could pass with only one side present. | P2 | T6 and validation commands now split `GoroutineStressTester`, lock-scope, and exact `AsyncJobTester N/A` checks. |
| Exported declaration comment policy was not carried into implementation tasks. | P3 | T1/T9/T10 now require the `docs/package-layout.md` exported-comment policy and verifier evidence. |
| `golangci-lint config verify` was missing. | P3 | T9 and validation commands now include `golangci-lint config verify`. |

## Gate

Final Step 3-R counts: P0=0, P1=0, P2=0, P3=0.

Gate verdict: PASS. The plan may advance to implementation.

## 검증

```bash
git diff --check
rg -n 'HS256 >= 32|unsupported|TryParse|exactly one fixed key|fixed HMAC key strength|inbound unsupported JOSE|golangci-lint config verify|comment policy|GoroutineStressTester|write lock|signature verification|AsyncJobTester N/A' docs/superpowers/specs/2026-06-08-issue-33-jwt-helper-utilities-spec.md docs/superpowers/plans/2026-06-08-issue-33-jwt-helper-utilities-plan.md
```
