# Issue #35 Money Step 6-R Code Review

> 한국어 리뷰 경계: 이 문서는 리뷰 판정과 근거를 한국어 독자가 추적할 수 있도록 정리한다. 심각도 토큰, 판정 토큰, 파일 경로, 라인 번호, 이슈/PR 번호, 명령, 코드 식별자는 원문의 증거 앵커로 보존한다.

## 범위

- Worktree: `.worktrees/issue-35-money`
- Base: `origin/develop`
- Package: `money`
- Review method: 6 independent subagent lanes plus integrated leader review.

## Subagent Lanes

| Tier | Lane | Result |
|---|---|---|
| T1 | API/spec compliance | Initial P0=0 P1=0. P2/P3 items for exported `CanConvert`, missing exchange-rate overflow test, missing text/mismatch examples. All fixed before final gate. |
| T2 | Correctness/error semantics | Initial P0=0 P1=2 for same-currency rate `1` rejection and scalar overflow mapping. Both fixed before final gate. |
| T3 | Concurrency/race/stress | P0=0 P1=0 P2=0 P3=0. |
| T4 | Test coverage/examples | Initial P0=0 P1=0 with P2/P3 coverage gaps. Oversized unmarshal, exchange-rate overflow, arithmetic overflow, invalid `Sum` currency, locale examples, text/mismatch examples were added. |
| T5 | Dependency/security/performance | Initial P0=0 P1=0 with README size-limit P2. Fixed before final gate. |
| T6 | Docs/release/workflow | Initial P0=0 P1=0 with README size-limit P2. Fixed before final gate. |

## Fixes Applied After Review

- Removed non-spec exported `ExchangeRate.CanConvert`; conversion check is now private.
- Allowed same-currency `NewExchangeRate(USD, USD, "1")` using value comparison through `IsOne`.
- Mapped scalar decimal parse overflow to `ErrOverflow`.
- Added tests for same-currency rate `1`, scalar overflow, multiplication and quotient overflow, exchange-rate conversion overflow, oversized JSON unmarshal, invalid `Sum(Currency{})`, and expanded locale examples.
- Added runnable text round-trip and currency mismatch examples.
- Documented caller-owned request/body/streaming size limits in `money/README.md` and `money/README.ko.md`.

## Integrated Review Evidence

- `git diff --check`: pass
- `golangci-lint config verify`: pass
- `go test -count=1 ./money`: pass
- `go test -count=1 ./money -run 'TestExchangeRate|TestArithmetic|TestConstructors|TestRound|TestCompare|TestParse|TestMarshal'`: pass
- `go test -count=1 ./money -run 'TestExchangeRateValidation|TestExchangeRateConvertOverflow|TestArithmeticErrors|TestParseRejectsAmbiguousOrOversizedInput|ExampleMoney_MarshalText|ExampleMoney_Add_mismatch'`: pass
- `go test -race -count=1 ./money`: pass
- `mcp__code_review_graph.detect_changes_tool`: no test gaps returned for changed file set; new-file function mapping was not available, so manual diff and subagent lane evidence were used as primary review evidence.
- `mcp__code_review_graph.find_large_functions_tool` with `file_path_pattern=money/`, `min_lines=50`: 0 large nodes.

## 발견 사항

### P0

None.

### P1

None after fixes.

### P2

None remaining.

### P3

None remaining.

## 판정

P0=0 P1=0

PASS. Issue #35 `money` implementation is ready for PR creation.
