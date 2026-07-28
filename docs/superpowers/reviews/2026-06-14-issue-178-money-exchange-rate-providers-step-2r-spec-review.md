# Issue #178 Money Exchange Rate Providers Step 2-R Spec Review

> 한국어 리뷰 경계: 이 문서는 리뷰 판정과 근거를 한국어 독자가 추적할 수 있도록 정리한다. 심각도 토큰, 판정 토큰, 파일 경로, 라인 번호, 이슈/PR 번호, 명령, 코드 식별자는 원문의 증거 앵커로 보존한다.

## 범위

- Spec: `docs/superpowers/specs/2026-06-14-issue-178-money-exchange-rate-providers-design.md`
- Research: `docs/superpowers/research/2026-06-14-issue-178-money-exchange-rate-providers-research.md`
- Issue: #178
- Follow-ups created: #231 IMF provider, #232 Bloomberg provider
- Baseline: `origin/develop` at `9b5464a`
- Worktree: `.worktrees/issue-178-money-exchange-rate-providers`
- Required reference: `/Users/debop/.codex/skills/bluetape4k-full-feature/references/step-2r-spec-review.md`

## 실행 메모

7-Tier gate shape is six independent lanes plus main integration review:
performance, stability, security, operator/Ops, developer/API, user/caller,
then current-session integration.

Native subagents were intentionally not used in this session. The user
instructed the main session to switch roles locally because native subagent
lifecycle repeatedly stalled in this thread. This artifact records
`main-role fallback performed per user instruction`.

## 관점별 결과

| Lane | P0 | P1 | P2 | P3 | Verdict | Evidence |
|---|---:|---:|---:|---:|---|---|
| Performance | 0 | 0 | 0 | 0 | PASS | Spec requires snapshot-level cache, no background goroutine, targeted tests, race gate, and no float math for cross rates. |
| Stability | 0 | 0 | 0 | 0 | PASS after edit | Initial P1: stale fallback only exposed `Stale=true`, which could hide refresh failure. Fixed by adding `ExchangeRateQuote.RefreshError` and required tests. |
| Security | 0 | 0 | 0 | 1 | PASS | No credentials/secrets in #178; Bloomberg credential/entitlement provider moved to #232. P3: implementation should keep endpoint injection documented as caller-owned configuration, not user input. |
| Operator/Ops | 0 | 0 | 0 | 0 | PASS | Spec exposes source, observed/fetched/expires/stale metadata and documents ECB informational/non-accounting boundary. |
| Developer/API | 0 | 0 | 0 | 0 | PASS after edit | Initial P1: invalid options and nil provider behavior were under-specified. Fixed by requiring nil/typed-nil provider rejection and option validation. |
| User/Caller | 0 | 0 | 0 | 0 | PASS | Spec separates value-only `Convert` from context-aware `ConvertWithProvider`, returns quote metadata, keeps README pair in scope, and links #231/#232. |

## 통합 발견 사항

| ID | Severity | Finding | Resolution |
|---|---|---|---|
| S2R-1 | P1 | Stale fallback could hide the refresh/fetch failure if the only visible signal was `ExchangeRateQuote.Stale=true`. | Added `RefreshError error` to `ExchangeRateQuote`; stale fallback tests must assert `Stale=true` and non-nil wrapped `RefreshError`. |
| S2R-2 | P1 | `ECBProviderOptions` validation and nil/typed-nil provider behavior were not explicit enough for a public API. | Added constructor validation requirements for negative durations/retry count, empty endpoint, invalid endpoint scheme, and nil/typed-nil provider rejection in `ConvertWithProvider`. |
| S2R-3 | P3 | Custom endpoint is caller-owned configuration; future implementation should not accept untrusted end-user endpoint values without caller validation. | Recorded as implementation note; no spec blocker because this library does not process external user input directly. |

## Re-Review

After the P1 edits:

- Stability lane rechecked stale fallback and cancellation semantics.
- Developer/API lane rechecked option validation, nil provider behavior, and Go-shaped public API.
- Main integration rechecked #178 scope and follow-up split.

## 수렴 판정

P0=0 P1=0.

Step 2-R PASS.

## Checklist Completion

| 항목 | 상태 | Notes |
|---|---|---|
| Six perspective lanes complete or fallback recorded | Done | Main-role fallback performed per user instruction. |
| Tier 1 performance spec review complete | Done | P0=0 P1=0. |
| Tier 2 stability spec review complete | Done | P1 fixed and rechecked. |
| Tier 3 security spec review complete | Done | P0=0 P1=0. |
| Tier 4 operator/Ops spec review complete | Done | P0=0 P1=0. |
| Tier 5 developer/API spec review complete | Done | P1 fixed and rechecked. |
| Tier 6 user/caller spec review complete | Done | P0=0 P1=0. |
| Main-session integrated spec review complete | Done | P0=0 P1=0. |
| P2/P3 disposition recorded | Done | One P3 recorded for implementation attention. |
