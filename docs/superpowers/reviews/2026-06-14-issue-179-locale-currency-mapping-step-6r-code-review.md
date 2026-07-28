# Issue #179 Step 6-R Code Review

> 한국어 리뷰 경계: 이 문서는 리뷰 판정과 근거를 한국어 독자가 추적할 수 있도록 정리한다. 심각도 토큰, 판정 토큰, 파일 경로, 라인 번호, 이슈/PR 번호, 명령, 코드 식별자는 원문의 증거 앵커로 보존한다.

## 검토 모드

7-Tier gate executed as six independent review lanes plus main integration review.

Native subagents were not used for this gate because this session showed unstable child-agent waits and the operator instruction was to continue with main-session role switching. Main integration fallback performed.

## 범위

- `money/currency.go`
- `money/currency_test.go`
- `money/currency_concurrency_test.go`
- `money/README.md`
- `money/README.ko.md`
- `README.md`
- `README.ko.md`
- `CHANGELOG.md`
- `go.mod`
- `docs/superpowers/specs/2026-06-14-issue-179-locale-currency-mapping-design.md`
- `docs/superpowers/plans/2026-06-14-issue-179-locale-currency-mapping-plan.md`

No `jwt` files are modified in this diff.

## 증거

- `node scripts/generate-money-locale-currency-diagram.mjs`: PASS
  - `nodes=9 routes=8 segments=10`
  - `badEndpointAngle=0 badBends=0 interiorCrossings=0 nodeOverlaps=0 laneClearance=0 marginImbalance=0`
  - `margins=L48/R48/T48/B48 titleGap=58`
- `xmllint --noout docs/images/readme-diagrams/money-locale-currency-resolution-flow.svg`: PASS
- `go test -count=1 ./money ./testing/concurrency`: PASS
- `go test -race -count=1 ./money ./testing/concurrency`: PASS
- `make fmt-check`: PASS
- `make vet`: PASS
- `make lint`: PASS, `0 issues`
- `git diff --check`: PASS
- `go test -count=1 -timeout=2m ./...`: FAIL in unchanged `jwt` cached-provider tests under local `go1.26.4 darwin/arm64`
  - `TestCachedProviderAsyncCancellationDoesNotCache`
  - `TestCachedProviderSetFailureIsVisible`
  - `TestCachedProviderLiveWaiterRetriesAfterCanceledOwner`

## Lane 1: Performance

판정: PASS.

`CurrencyByLocale` now performs a small BCP47 normalization, explicit region extraction, and one CLDR currency query per call. There is no new shared cache, lock, goroutine, I/O, or network dependency. The lookup path is bounded by locale tag segments and CLDR tender iterator size.

The repo-local `GoroutineStressTester` covers concurrent success and rejection paths, and `go test -race` found no race.

## Lane 2: Stability And Concurrency

판정: PASS with visible external blocker.

The implementation keeps language-only tags invalid by requiring an explicit region before currency resolution. It tolerates `language.ValueError` only after a valid explicit region is found, preserving the existing `at-AT` compatibility case while still rejecting malformed tags.

The full repository test run is currently blocked by unchanged `jwt` cached-provider tests on local Go 1.26.4. This is not introduced by the #179 diff because no `jwt` files are changed. PR CI remains the final full-suite gate.

## Lane 3: Security

판정: PASS.

The change parses local strings only and does not add network calls, file access, credential handling, serialization, shell execution, or user-controlled resource expansion beyond bounded tag parsing. Invalid, missing, no-tender, and ambiguous regions are rejected through the existing `ErrInvalidCurrency` sentinel.

## Lane 4: Operator And Operations

판정: PASS.

`golang.org/x/text` is promoted to a direct dependency because the money package now imports `language` and `currency` directly. README files and CHANGELOG document the CLDR-backed behavior and caveat that locale mapping is a convenience, not legal/accounting authority.

Diagram artifacts are regenerated and checked with geometry, SVG, and PNG gates.

## Lane 5: Developer And API

판정: PASS.

The public API remains `CurrencyByLocale(tag string) (Currency, error)`. No new exported function or behavior flag is added. Errors wrap `ErrInvalidCurrency`, preserving `errors.Is` callers.

The implementation uses `currency.Query(currency.Region(r))` rather than `currency.FromTag`, avoiding likely-region inference that would silently change language-only behavior.

## Lane 6: User And Caller

판정: PASS.

Common locales such as `ko-KR`, `en_US`, `de-DE`, `en-GB`, `fr-CA`, `en-AU`, `pt-BR`, `hi-IN`, `es-MX`, and `zh-Hant-TW` are covered. Ambiguous or unsupported cases such as `ko`, `und`, `en-001`, `en-QM`, `en-AQ`, `es-PA`, and `en-u-cu-usd` are rejected consistently.

Docs explain that callers must choose a currency themselves when a region has multiple current tender units.

## 메인 통합 검토

P0 findings: 0.

P1 findings: 0.

P2 findings:

- Local full-suite validation is blocked by existing `jwt` cached-provider tests under Go 1.26.4. The blocker is outside the #179 diff and must remain visible in the PR DoD until CI proves or rejects the full-suite state.

P3 findings: 0.

Integrated verdict: PASS for PR creation. Do not merge until GitHub CI and maintainer approval complete.
