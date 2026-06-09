# Issue #35 Money and Decimal Helpers Plan

## Scope

- Issue: #35, `Port money and decimal helpers`
- Spec: `docs/superpowers/specs/2026-06-09-issue-35-money-decimal-spec.md`
- Review: `docs/superpowers/reviews/2026-06-09-issue-35-money-spec-review.md`
- Package: `money`
- Workflow: Type A full-feature, Go implementation with `$bluetape-go-patterns`

## Implementation Order

### T1 - Dependency and package scaffold

- complexity: medium
- files: `go.mod`, `go.sum`, `money/doc.go`, `money/README.md`, `money/README.ko.md`
- apply: `$bluetape-go-patterns`
- tasks:
  - Run `go get github.com/govalues/money@v0.2.4`.
  - Run `go mod tidy` after code exists.
  - Create top-level `money` package with package docs.
  - Keep exported Go comments in Korean and start each exported comment with the exported identifier.
  - Record that `govalues/decimal` is used internally/transitively unless scalar parsing requires a direct import.
- verification:
  - `go list -m github.com/govalues/money github.com/govalues/decimal`
  - `go list -m -json github.com/govalues/money github.com/govalues/decimal`
  - `gh repo view govalues/money --json nameWithOwner,isArchived,pushedAt,updatedAt,latestRelease,licenseInfo`
  - `gh repo view govalues/decimal --json nameWithOwner,isArchived,pushedAt,updatedAt,latestRelease,licenseInfo`
  - `git diff -- go.mod go.sum`

### T2 - Currency API

- complexity: high
- files: `money/currency.go`, `money/errors.go`, `money/currency_test.go`
- apply: `$bluetape-go-patterns`
- tasks:
  - Implement `Currency` wrapper and common values `KRW`, `USD`, `EUR`, `CNY`, `JPY`.
  - Implement `ParseCurrency`, `MustParseCurrency`, `IsCurrency`, `CurrencyByLocale`.
  - Implement `Code`, `Num`, `Scale`, `String`, `IsZero`.
  - Reject upstream no-currency values `XXX`, `999`, and wrapper `Currency{}` with `ErrInvalidCurrency`.
  - Normalize locale tags case-insensitively and accept `_` separators.
  - Support required locale examples from the spec and reject missing/unknown/unsupported tags with `ErrInvalidCurrency`.
  - Map upstream invalid currency errors to `ErrInvalidCurrency` with `errors.Is` compatibility.
  - Test `IsCurrency` true/false cases and `MustParseCurrency` success plus panic/failure behavior.
- verification:
  - `go test -count=1 ./money -run 'Test(ParseCurrency|MustParseCurrency|IsCurrency|CurrencyByLocale|CurrencyMethods|NoCurrency)'`

### T3 - Money constructors and scalar/value API

- complexity: high
- files: `money/money.go`, `money/errors.go`, `money/money_test.go`
- apply: `$bluetape-go-patterns`
- tasks:
  - Implement `Money` wrapper, `New`, `NewFromInt64`, `NewFromFloat64`, `Zero`, `NewMinor`, `Parse`.
  - Implement `Currency`, `String`, `Amount`, `MinorUnits`, `Float64`, `IsZero`.
  - Reject `Money{}` where a valid money value is required with `ErrInvalidMoney`.
  - Define and test `NewFromInt64` as major units: `NewFromInt64(12, USD)` is `USD 12.00`; `NewFromInt64(12, KRW)` is `KRW 12`.
  - Define and test `NewMinor` as currency minor units: `NewMinor(12, USD)` is `USD 0.12`; `NewMinor(12, JPY)` is `JPY 12`.
  - Reject any money construction using `XXX`, `999`, or `Currency{}` with `ErrInvalidCurrency`.
  - Reject NaN, +/-Inf, non-representable float, malformed amount, overflow, and invalid currency with sentinel-compatible errors.
  - Keep float construction documented as ergonomic, not preferred for deterministic financial input.
  - Add table cases for negative values, minor-unit extraction/overflow, `Float64` conversion error, and no-currency rejection.
- verification:
  - `go test -count=1 ./money -run 'Test(New|Zero|Minor|Float|Parse|MoneyMethods|NoCurrency)'`

### T4 - Arithmetic, rounding, comparison, and aggregation

- complexity: high
- files: `money/money.go`, `money/money_test.go`, `money/aggregation_test.go`
- apply: `$bluetape-go-patterns`
- tasks:
  - Implement `Round`, `RoundTo`, `Add`, `Sub`, `Neg`, `Abs`, `Cmp`, `Equal`.
  - Implement `Mul(factor string)` and `Quo(divisor string)` using parsed scalar decimal internally; do not expose `govalues/decimal.Decimal` in #35 public API.
  - Map malformed scalar to `ErrInvalidAmount`, divide-by-zero to `ErrDivideByZero`, overflow to `ErrOverflow`.
  - Implement `Sum(currency Currency, values ...Money)`.
  - `Sum(currency)` returns `Zero(currency)`.
  - `Sum` rejects invalid currency, zero-value money members, and mixed currencies with typed errors.
  - Add concrete table cases for `Neg`, `Abs`, `Equal`, zero-value/invalid money behavior, half-even currency-scale `Round`, and custom `RoundTo` scales.
- verification:
  - `go test -count=1 ./money -run 'Test(Add|Sub|Mul|Quo|Round|RoundTo|Neg|Abs|Cmp|Equal|Sum)'`

### T5 - Serialization, parsing, and bounded input behavior

- complexity: high
- files: `money/encoding.go`, `money/encoding_test.go`
- apply: `$bluetape-go-patterns`
- tasks:
  - Implement `encoding.TextMarshaler`, `encoding.TextUnmarshaler`, `json.Marshaler`, `json.Unmarshaler`.
  - Use JSON shape `{"amount":"12.34","currency":"USD"}`.
  - Use documented text shape `USD 12.34`.
  - Reject unknown currency, malformed amount, ambiguous empty input, invalid JSON shape, and invalid zero-value marshal receivers with typed errors.
  - Allow `var m Money; json.Unmarshal(...)` and `m.UnmarshalText(...)` to populate valid input into a zero-value destination.
  - Reject nil `*Money` unmarshal receivers with `ErrInvalidMoney` instead of panic.
  - Add bounded oversized parse/unmarshal tests and document that HTTP/body size limits are caller-owned.
- verification:
  - `go test -count=1 ./money -run 'Test(Marshal|Unmarshal|ParseText|ZeroValueDestination|NilUnmarshal|Oversized)'`

### T6 - Exchange-rate value API

- complexity: high
- files: `money/exchange_rate.go`, `money/exchange_rate_test.go`
- apply: `$bluetape-go-patterns`
- tasks:
  - Implement `ExchangeRate` wrapper.
  - Implement `NewExchangeRate`, `Convert`, `Valid`, `Base`, `Quote`, `Rate`, `IsZero`.
  - Treat `ExchangeRate{}`, zero, negative, malformed rates, and same-currency non-1 rates as `ErrInvalidExchangeRate`.
  - Reject exchange-rate construction using `XXX`, `999`, or `Currency{}` with `ErrInvalidCurrency`.
  - Validate wrapper rate before upstream conversion so invalid/zero rate is not collapsed into currency mismatch.
  - Map valid rate with amount currency outside base/quote to `ErrCurrencyMismatch`.
  - Test direct conversion, reverse conversion, invalid rate, zero-value rate, amount outside base/quote, same-currency rate, and overflow/error mapping.
- verification:
  - `go test -count=1 ./money -run 'Test(ExchangeRate|Convert)'`

### T7 - Examples, stress, and race coverage

- complexity: high
- files: `money/money_example_test.go`, `money/money_concurrency_test.go`, `docs/superpowers/reviews/2026-06-09-issue-35-money-concurrency-notes.md`
- apply: `$bluetape-go-patterns`
- tasks:
  - Add runnable examples for construction, arithmetic/mismatch handling, JSON/text round trip, aggregation, and caller-supplied exchange-rate conversion.
  - Use `testing/concurrency.GoroutineStressTester` to repeatedly parse, construct, serialize, add/subtract same-currency values, and reject cross-currency operations across goroutines.
  - Use explicit stress options, not helper defaults: at least `Workers: max(32, runtime.GOMAXPROCS(0)*4)`, `RoundsPerTask: 512`, and `Timeout: 10 * time.Second`.
  - Assert `report.Completed`, `report.Failures == 0`, and expected operation counts.
  - Run `go test -race -count=1 ./money`.
  - Because #35 core has no context-aware async/provider/IO boundary, record exact note: `AsyncJobTester N/A: #35 money core has no context-aware async, provider, IO, or cancellation boundary.`
- verification:
  - `go test -count=1 ./money -run 'Example|Stress|Concurrent'`
  - `go test -race -count=1 ./money`
  - `rg -n 'GoroutineStressTester' money`
  - `rg -n 'AsyncJobTester N/A: #35 money core has no context-aware async, provider, IO, or cancellation boundary' docs/superpowers/reviews/2026-06-09-issue-35-money-concurrency-notes.md`

### T8 - Follow-up issues

- complexity: medium
- files: GitHub issues, PR body references
- apply: `$bluetape-go-patterns`
- tasks:
  - Create provider-backed exchange-rate issue with milestone `0.6.1`, assignee `debop`, labels `priority: p1`, `area: utilities`, `type: research`, linked to #35.
  - Create full locale-to-currency mapping issue with milestone `0.6.1` or later, assignee `debop`, labels `priority: p2`, `area: utilities`, `type: research`, linked to #35.
  - Create optional long-backed `FastMoney` issue with later milestone, assignee `debop`, labels `priority: p2`, `area: utilities`, `type: research`, linked to #35.
  - Use follow-up links in README and PR body.
- verification:
  - `gh issue view <issue> --json assignees,labels,milestone,body`

### T9 - Documentation and release artifacts

- complexity: medium
- files: `money/README.md`, `money/README.ko.md`, `README.md`, `README.ko.md`, `CHANGELOG.md`, `WIP.md`
- apply: `$bluetape-go-patterns`
- tasks:
  - Document precision model, dependency adoption, half-even/currency-scale rounding, serialization shape, minor-unit behavior, currency mismatch, provider deferral, not-full-accounting-system caveat, and stress/race validation.
  - Keep English and Korean README pairs in sync.
  - Update root package list and 0.6.0 status without claiming #34 measure is merged unless current branch contains it.
  - Update `CHANGELOG.md` in Keep a Changelog style.
  - Update `WIP.md` for #35 and 0.6.0 state.
- verification:
  - `test -f money/README.md && test -f money/README.ko.md`
  - `rg -n 'not a full accounting|precision|rounding|serialization|GoroutineStressTester' money/README.md`
  - `rg -n '전체 회계 시스템|정밀도|반올림|직렬화|GoroutineStressTester' money/README.ko.md`
  - `rg -n '\\| .*money.*\\|' README.md`
  - `rg -n '\\| .*money.*\\|' README.ko.md`
  - `rg -n 'money|Money|Currency|ExchangeRate' CHANGELOG.md`
  - `rg -n '#35|money|Money' WIP.md`

### T10 - Full validation

- complexity: high
- files: repository checks and review artifacts
- apply: `$bluetape-go-patterns`
- tasks:
  - Run formatting: `gofmt` on touched Go files.
  - Run `go mod tidy`.
  - Run `git diff --check`.
  - Revalidate selected dependency versions/license/archived state with `go list -m -json` and `gh repo view`, then record evidence in the Step 4-T testlog.
  - Run `golangci-lint config verify` when available/configured.
  - Run `go test -count=1 ./money`.
  - Run `go test -race -count=1 ./money`.
  - Run `make ci`.
  - If any validation is unavailable, record exact command, failure, and next-best evidence.
- verification:
  - command outputs in Step 4-T testlog.

### T11 - Step 6-R review, PR, and metadata

- complexity: high
- files: `docs/superpowers/reviews/*`, `docs/review/*`, PR body
- apply: `$bluetape-go-patterns`
- tasks:
  - Create Step 4-T testlog.
  - Run Step 6-R 7-Tier code review with subagents and close only when P0=0/P1=0.
  - Store concise review artifact under tracked docs before PR creation.
  - Commit spec and plan before implementation commit if not already committed.
  - Commit implementation with Lore protocol.
  - Push branch and create PR.
  - Set PR assignee, labels, and milestone to match #35.
  - Verify PR body ends with `## DoD Status`.
  - Do not merge; merge remains user-requested.
- verification:
  - `gh pr view <pr> --json assignees,labels,milestone,body`
  - `gh pr checks <pr>`

## Acceptance Mapping

| Spec requirement | Plan tasks |
|---|---|
| Candidate dependency comparison and adoption decision | T1 plus existing spec/research preservation |
| Currency lookup, validation, constants, limited locale lookup | T2 |
| Money creation, zero-value handling, minor units, float caveats | T3 |
| Arithmetic, rounding, comparison, aggregation | T4 |
| Parsing, formatting, JSON/text serialization, bounded input behavior | T5 |
| Caller-supplied exchange-rate conversion | T6 |
| Provider-backed exchange-rate deferral links | T8, T9 |
| `FastMoney` decision and follow-up | T8, T9 |
| Stress/race requirement | T7, T10 |
| README and release docs | T9 |
| PR metadata and review gates | T11 |

## Risk Controls

| Risk | Control |
|---|---|
| Silent currency coercion | T4 and T6 require `ErrCurrencyMismatch` tests. |
| Upstream invalid rate collapse | T6 validates wrapper rate before upstream conversion. |
| Float exactness confusion | T3/T9 document and test NaN/Inf/non-representable values. |
| Unbounded parse/unmarshal behavior | T5 tests bounded oversized inputs and documents caller-owned request-size limits. |
| Async helper requirement misapplied | T7 records exact `AsyncJobTester N/A` note; provider follow-up must use `AsyncJobTester`. |
| Documentation drift | T9 uses `rg` against all touched README/release files. |

## Commit Boundary

- Commit 1: spec, Step 2-R review, and plan after Step 3-R passes.
- Commit 2: implementation, tests, docs, follow-up issue links, and review artifacts.

## Step 3 Checklist Completion Report

| Item | Status | Notes |
|---|---|---|
| Plan path confirmed inside feature worktree | Done | `docs/superpowers/plans/2026-06-09-issue-35-money-decimal-plan.md` |
| All tasks have complexity labels | Done | T1-T11 |
| `$bluetape-go-patterns` applied to Go code-bearing tasks | Done | Every task includes apply line. |
| Plan code/test snippets conform to Go patterns | Done | No implementation snippets beyond API names; tests are command/task based. |
| Thread/cancellation helpers assigned | Done | `GoroutineStressTester`; `AsyncJobTester` exact N/A note unless context-aware API is added. |
| Tests and verification tasks included | Done | T2-T7, T10 |
| Multilingual README and contributor artifacts included | Done | T9, T11 |
| Risky ordering/dependency assumptions explicit | Done | T1 dependency, T8 follow-up before docs, T6 wrapper validation before upstream conversion. |
| Spec + plan committed before implementation | Pending | Commit after Step 3-R PASS. |
