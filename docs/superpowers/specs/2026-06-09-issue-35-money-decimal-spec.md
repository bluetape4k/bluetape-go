# Issue #35 Money and Decimal Helpers Spec

## 1. 문제 정의

Issue #35는 JVM `bluetape4k-money`의 핵심 사용 경험을 `bluetape-go`에 맞게 이식하거나, 안전하지 않은 범위는 명시적으로 후속 이슈로 분리하는 작업이다.

목표는 회계 시스템 전체가 아니라, 라이브러리 사용자에게 다음을 제공하는 작고 일관된 money package다.

- ISO 4217 currency lookup, validation, common constants.
- Decimal-backed monetary amount with explicit currency and rounding behavior.
- Creation from string, integer, float, decimal string, currency code, and minor units.
- Arithmetic, rounding, comparison, aggregation, parsing, formatting, JSON/text serialization.
- Caller-supplied exchange rate conversion primitives.
- Stress/race evidence for goroutine-safe value usage.

## 2. 현재 근거

### GitHub issue

- `gh issue view 35 --json ...` 확인 결과, issue #35는 `priority: p1`, `area: utilities`, `type: research`, milestone `0.6.0`, assignee `debop`이다.
- Acceptance criteria는 dependency 비교, Money/FastMoney/currency/rounding/aggregation/parsing/exchange-rate 결정, 구현 시 테스트, README 정밀도/rounding/serialization/비회계시스템 설명을 요구한다.
- Stress requirement는 새 기능에 `GoroutineStressTester`, `AsyncJobTester` 사용을 요구한다.

### JVM `bluetape4k-money`

검토 소스:

- `/Users/debop/work/bluetape4k/bluetape4k-projects/utils/money/README.md`
- `/Users/debop/work/bluetape4k/bluetape4k-projects/utils/money/src/main/kotlin/io/bluetape4k/money/CurrencySupport.kt`
- `/Users/debop/work/bluetape4k/bluetape4k-projects/utils/money/src/main/kotlin/io/bluetape4k/money/MoneySupport.kt`
- `/Users/debop/work/bluetape4k/bluetape4k-projects/utils/money/src/main/kotlin/io/bluetape4k/money/FastMoneySupport.kt`
- `/Users/debop/work/bluetape4k/bluetape4k-projects/utils/money/src/main/kotlin/io/bluetape4k/money/MoneyAmountSupport.kt`
- `/Users/debop/work/bluetape4k/bluetape4k-projects/utils/money/src/main/kotlin/io/bluetape4k/money/CurrencyConverter.kt`
- `/Users/debop/work/bluetape4k/bluetape4k-projects/utils/money/src/main/kotlin/io/bluetape4k/money/CurrencyConversionSupport.kt`

확인된 JVM 기능:

- JSR-354/Moneta 기반 `CurrencyUnit`, `Money`, `FastMoney`, `MonetaryAmount` helper.
- `KRW`, `USD`, `EUR`, `CNY`, `JPY` common constants.
- currency code와 `Locale` 기반 lookup/cache.
- `Money`는 `BigDecimal` 기반, `FastMoney`는 `Long` 기반 minor-unit 지향 모델.
- `+`, `-`, `*`, `/`, unary `-`, value extraction, rounding, aggregation.
- ECB/IMF provider를 통한 exchange-rate conversion.

Go package는 Kotlin extension/operator surface를 기계적으로 복제하지 않는다. `bluetape-go-patterns`에 따라 작고 명시적인 Go API를 설계한다.

### Current `bluetape-go`

- `go.mod`에는 아직 money/decimal dependency가 없다.
- 0.6.0 research 문서 `docs/research/2026-06-01-milestone-0.6.0-utilities-research.md`는 `money`를 "research-first"로 분류했고, decimal dependency와 currency semantics를 설계 위험으로 기록했다.
- `testing/concurrency`에는 `GoroutineStressTester`와 `AsyncJobTester`가 이미 있다.
- 기존 public package들은 top-level directory, `doc.go`, package `README.md`, examples, focused tests, root README/README.ko/CHANGELOG/WIP 갱신 패턴을 사용한다.

### Dependency research preservation

외부 dependency 근거는 wiki 연구 노트로 보존했다.

- `/Users/debop/work/bluetape4k/bluetape4k-wiki/research/2026-06-09-go-money-decimal-dependency-candidates.md`
- `gno update`: `bluetape4k-wiki` 1 added.
- `gno embed --collection bluetape4k-wiki`: 10/10 chunks embedded. Metal compile warning은 있었지만 embedding completed.
- `gno search "govalues money bluetape-go issue 35" -c bluetape4k-wiki`: 해당 연구 노트 1건 검색됨.

## 3. Dependency 비교

2026-06-09 기준 GitHub CLI, GitHub README/license API, `go list -m -versions`, 임시 module `go doc`로 확인했다.

| Candidate | Latest | License | Maintenance evidence | Strength | Weakness | Decision |
|---|---:|---|---|---|---|---|
| `github.com/govalues/money` | `v0.2.4` | MIT | Not archived; pushed 2025-01-25; updated 2026-06-04; latest release `v0.2.4`. | Immutable `Amount`, `Currency`, `ExchangeRate`; ISO 4217 metadata; panic-free error-returning arithmetic; minor units; parsing/formatting/serialization; exchange-rate primitives. | Smaller ecosystem than `shopspring/decimal`; upstream API becomes important public dependency. | Adopt as core dependency. |
| `github.com/govalues/decimal` | `v0.1.36` | MIT | Not archived; pushed 2025-01-18; updated 2026-06-08; latest release `v0.1.36`. | Correctly rounded decimal, immutable, panic-free, JSON/XML/SQL/BSON support, no heap allocation goal. | 19 digit precision family, not arbitrary precision. | Accept transitively through `govalues/money`; avoid direct public dependency unless needed. |
| `github.com/shopspring/decimal` | `v1.4.0` | Other/NOASSERTION via GitHub license API | Not archived; pushed 2026-03-15; updated 2026-06-09; latest release `v1.4.0`. | Very popular arbitrary-precision decimal; SQL/JSON/XML support; mature. | Decimal-only; mutable package globals such as division/json knobs; no currency mismatch model. | Reject for #35 core. Keep as reference. |
| `github.com/cockroachdb/apd/v3` | `v3.2.3` | Apache-2.0 | Not archived; pushed 2026-03-23; updated 2026-06-08; latest release `v3.2.3`. | Powerful arbitrary precision, context flags/traps, SQL-friendly. | Heavier API and stateful context model than a small utility package needs. | Reject for #35 core. |
| `github.com/Rhymond/go-money` | `v1.0.15` | MIT | Not archived; pushed 2026-04-29; updated 2026-06-08; latest release `v1.0.15`. | Popular Fowler-style integer minor-unit money package. | Narrower decimal/exchange-rate surface; minor-unit model only; weaker fit for JVM `Money` parity. | Reject for #35 core; use as minor-unit comparison reference. |

## 4. 설계 옵션

### Option A: First-party implementation only

Implement currency metadata, decimal arithmetic, parsing, formatting, serialization, rounding, and exchange rate conversion directly.

장점:

- Public dependency를 줄일 수 있다.
- API 전체를 repo가 소유한다.

단점:

- Decimal correctness와 ISO 4217 metadata를 직접 유지해야 한다.
- Overflow, rounding, serialization, parsing edge cases가 커져 0.6.0 범위와 맞지 않는다.
- 기존 Go 생태계의 검증된 money/decimal 구현을 재사용하지 못한다.

결정: Reject. #35의 핵심 위험은 money semantics이며, 직접 구현은 correctness risk가 더 크다.

### Option B: Thin re-export of `govalues/money`

Package `money`가 upstream type/function을 그대로 type alias 또는 wrapper 없이 노출한다.

장점:

- 구현량이 작다.
- Upstream 기능을 거의 그대로 사용할 수 있다.

단점:

- `bluetape-go`가 보장할 sentinel/typed error, docs, locale helper, README semantics가 약해진다.
- "thin wrapper보다 explicit semantics"라는 issue 방향과 맞지 않는다.
- 나중에 exchange-rate provider나 locale mapping을 추가할 때 public surface가 흩어진다.

결정: Reject. Upstream은 core engine으로 쓰되 public API는 작게 소유한다.

### Option C: `govalues/money` 기반 bluetape-go API layer

`money` package는 `govalues/money`를 내부 engine으로 채택하고, repo가 소유하는 좁은 API와 error contract를 제공한다.

장점:

- Decimal/currency correctness는 검증된 dependency를 활용한다.
- `bluetape-go`가 public contract, errors, docs, examples, serialization guidance를 통제한다.
- JVM `bluetape4k-money`의 주요 capability를 Go idiom으로 제공할 수 있다.

단점:

- Upstream dependency가 public behavior에 영향을 준다.
- Wrapper boundary가 너무 두꺼우면 단순 위임 코드가 늘 수 있다.
- 별도 `FastMoney` type과 provider-backed exchange-rate는 완전 parity가 아니다.

결정: Adopt. #35의 기본 구현안이다.

## 5. API 범위

Package path: `github.com/bluetape4k/bluetape-go/money`.

### Public types

```go
type Currency struct { /* wraps govalues/money.Currency */ }
type Money struct { /* wraps govalues/money.Amount */ }
type ExchangeRate struct { /* wraps govalues/money.ExchangeRate */ }
```

Type alias 대신 wrapper를 사용한다. 이유:

- `bluetape-go`가 stable zero-value, error mapping, docs, examples를 소유한다.
- Upstream API 전체를 그대로 public contract로 고정하지 않는다.
- 향후 locale/provider/cache 기능을 추가해도 surface를 유지할 수 있다.

Zero-value contract:

- `Currency{}`, `Money{}`, `ExchangeRate{}`는 invalid/unspecified value로 취급한다.
- Zero amount는 `Zero(currency)` 또는 `New("0", currency)`로 생성한다.
- Invalid zero-value 사용은 `ErrInvalidCurrency`, `ErrInvalidMoney`, 또는 `ErrInvalidExchangeRate`를 반환한다.
- `Convert`는 upstream conversion 호출 전에 wrapper-level `ExchangeRate.Valid()` 검증을 수행한다. Invalid/zero exchange rate는 `ErrInvalidExchangeRate`로, valid rate와 amount의 base/quote mismatch는 `ErrCurrencyMismatch`로 매핑한다.
- Upstream `XXX`/`999` no-currency는 `govalues/money`에서 parse 가능하지만, #35 public wrapper에서는 invalid currency로 취급한다. `ParseCurrency("XXX")`, `ParseCurrency("999")`, `Currency{}`, and any money/exchange-rate construction using no-currency return `ErrInvalidCurrency`.

### Currency API

필수:

- `ParseCurrency(code string) (Currency, error)`
- `MustParseCurrency(code string) Currency`
- `IsCurrency(code string) bool`
- `CurrencyByLocale(tag string) (Currency, error)`
- constants/vars: `KRW`, `USD`, `EUR`, `CNY`, `JPY`
- methods: `Code() string`, `Num() string`, `Scale() int`, `String() string`, `IsZero() bool`
- `IsZero()` is true for wrapper zero-value and no-currency; no-currency is never a valid business currency in #35.

Locale lookup scope:

- `CurrencyByLocale`는 BCP47-like tag의 region subtag를 보고 currency를 반환한다.
- 0.6.0 core는 case-insensitive tag와 `_` separator normalization을 지원한다.
- Required accepted examples: `ko-KR`, `ko_KR` -> `KRW`; `en-US`, `en_US` -> `USD`; `ja-JP` -> `JPY`; `zh-CN` -> `CNY`; `de-DE`, `fr-FR`, `it-IT`, `es-ES`, `nl-NL`, `pt-PT`, `fi-FI`, `ie-IE`, `at-AT`, `be-BE` -> `EUR`.
- Missing region, unknown region, historical currency ambiguity, and unsupported tag format return `ErrInvalidCurrency`.
- full CLDR locale-to-currency database는 #35 범위 밖이다. 필요 시 follow-up으로 분리한다.

### Money creation

필수:

- `New(amount string, currency Currency) (Money, error)`
- `NewFromInt64(units int64, currency Currency) (Money, error)`
- `NewFromFloat64(amount float64, currency Currency) (Money, error)`
- `Zero(currency Currency) (Money, error)`
- `NewMinor(units int64, currency Currency) (Money, error)`
- `Parse(s string) (Money, error)`
- convenience: `KRWAmount`, `USDAmount`, `EURAmount`, `CNYAmount`, `JPYAmount` if they remain narrow and tested.

Float creation caveat:

- `NewFromInt64` uses major units: `NewFromInt64(12, USD)` is `USD 12.00`; `NewFromInt64(12, KRW)` is `KRW 12`.
- `NewMinor` uses currency minor units: `NewMinor(12, USD)` is `USD 0.12`; `NewMinor(12, JPY)` is `JPY 12`.
- `NewFromFloat64` exists for ergonomics but README must prefer string/minor-unit constructors for deterministic financial input.
- `NewFromFloat64` rejects NaN, +/-Inf, and non-representable values with `ErrInvalidAmount` or `ErrOverflow`.

### Money behavior

필수:

- `Currency() Currency`
- `String() string`
- `Amount() string`
- `MinorUnits() (int64, error)`
- `Float64() (float64, error)` with README caveat.
- `Round() (Money, error)` using currency scale and upstream half-even semantics.
- `RoundTo(scale int) (Money, error)`
- `Add(other Money)`, `Sub(other Money)`, `Neg()`, `Abs()`, `Cmp(other Money)`, `Equal(other Money)`.
- `Mul(factor string)` and `Quo(divisor string)` parse scalar decimal strings internally. They do not expose `govalues/decimal.Decimal` in #35 public API. Malformed scalar input maps to `ErrInvalidAmount`; divide by zero maps to `ErrDivideByZero`; overflow maps to `ErrOverflow`.
- `Sum(currency Currency, values ...Money) (Money, error)`

Currency mismatch:

- `Add`, `Sub`, `Cmp`, `Sum` must return an error compatible with `errors.Is(err, ErrCurrencyMismatch)`.
- No operation silently coerces currencies.
- `Sum(currency)` with no values returns `Zero(currency)`.
- `Sum` rejects invalid currency, zero-value money members, and mixed-currency members with typed sentinel errors.

### Serialization/parsing/formatting

필수:

- `Money` implements `encoding.TextMarshaler`, `encoding.TextUnmarshaler`, `json.Marshaler`, `json.Unmarshaler`.
- JSON must be stable and explicit. Proposed shape:

```json
{"amount":"12.34","currency":"USD"}
```

- Text format must be stable and documented. Proposed shape: `USD 12.34`.
- Parsing must reject unknown currency, malformed amount, and ambiguous empty input with typed errors.
- Parse and unmarshal tests must include oversized but bounded inputs. The package must not allocate unbounded internal buffers beyond the supplied input size. Request/body size limits remain caller-owned and must be documented in README.
- Marshal rejects invalid zero-value `Money` receivers with `ErrInvalidMoney`.
- `(*Money).UnmarshalJSON` and `(*Money).UnmarshalText` allow zero-value destinations and populate them on valid input.
- Nil `*Money` unmarshal receivers return `ErrInvalidMoney` instead of panic.

### Exchange rate API

필수 #35 core:

- `NewExchangeRate(base Currency, quote Currency, rate string) (ExchangeRate, error)`
- `Convert(amount Money, rate ExchangeRate) (Money, error)`
- Base/quote mismatch returns typed currency mismatch or invalid rate error.
- `ExchangeRate.Valid() bool`, `Base() Currency`, `Quote() Currency`, `Rate() string`, and `IsZero() bool`.
- `ExchangeRate{}` and zero/negative/malformed rates are invalid and map to `ErrInvalidExchangeRate`.
- Same-currency exchange rates must be exactly `1`; any other value returns `ErrInvalidExchangeRate`.
- `Convert` must test successful direct and reverse conversion, invalid rate, zero-value rate, amount currency outside base/quote, and overflow/error mapping.

명시적 후속:

- ECB/IMF/provider-backed fetching is deferred. JVM implementation delegates provider data to Moneta; Go should not add hidden network IO without context, timeout, cache freshness, retry, and provider-failure semantics.
- A follow-up issue must be opened for provider-backed exchange-rate support if #35 implements only caller-supplied rates.

### FastMoney / minor-unit decision

#35 will not add a separate `FastMoney` public type in the first slice.

Decision:

- Implement minor-unit constructors/extractors on `Money`.
- Document that this covers the `FastMoney` use case of exact minor-unit input/output for the 0.6.0 core.
- Defer a long-backed `FastMoney` type until benchmark or API pressure proves it is needed.

Rationale:

- `govalues/money` already provides an immutable decimal amount with minor-unit constructors.
- A separate type doubles arithmetic, serialization, parsing, and docs surface.
- Premature long-backed type risks mismatch with the core rounding and exchange-rate model.

## 6. Error contract

Sentinel errors:

- `ErrInvalidCurrency`
- `ErrInvalidMoney`
- `ErrInvalidAmount`
- `ErrCurrencyMismatch`
- `ErrDivideByZero`
- `ErrOverflow`
- `ErrInvalidExchangeRate`

Rules:

- Public errors wrap causal upstream errors with `%w` where available.
- Callers must be able to use `errors.Is` on sentinel errors.
- Invalid zero-value use returns a typed error, never panic.
- Divide by zero and overflow remain caller-visible errors.

## 7. Concurrency and stress contract

The package must be value-oriented and safe for concurrent read/use after construction.

Tests:

- Use `testing/concurrency.GoroutineStressTester` to repeatedly parse, construct, serialize, add/subtract same-currency values, and reject cross-currency operations across goroutines.
- Run `go test -race -count=1 ./money`.
- Use `AsyncJobTester` only if a context-aware async/batch/provider API is added.
- If #35 keeps only local value operations, record an explicit review note: `AsyncJobTester N/A: #35 money core has no context-aware async, provider, IO, or cancellation boundary.`

Reason:

- `AsyncJobTester` is meaningful for context/deadline/cancellation job checks. Adding artificial context to CPU-local value constructors would create API noise and weak evidence.
- Provider-backed exchange-rate support is exactly the follow-up area where `AsyncJobTester` becomes mandatory.

## 8. Documentation scope

Update:

- `money/README.md`
- `money/README.ko.md`
- package `doc.go`
- root `README.md`
- root `README.ko.md`
- `CHANGELOG.md`
- `WIP.md`

README must document:

- precision model and `govalues/money`/`govalues/decimal` adoption.
- half-even/currency scale rounding.
- JSON/text serialization shape.
- minor-unit behavior.
- currency mismatch behavior.
- why this is not a full accounting system.
- exchange-rate provider deferral and follow-up issue link.
- stress/race validation.

## 9. Follow-up issues

Must create/link after spec/plan if implementation defers scope:

1. Provider-backed exchange-rate conversion:
   - milestone: `0.6.1` unless milestone triage changes.
   - metadata: assignee `debop`, labels `priority: p1`, `area: utilities`, `type: research`.
   - link back to #35 and the #35 PR.
   - context-aware provider API, timeout, cache freshness, retry/fallback, provider source contract, `AsyncJobTester` cancellation tests.
2. Full locale-to-currency mapping:
   - milestone: `0.6.1` or later.
   - metadata: assignee `debop`, labels `priority: p2`, `area: utilities`, `type: research`.
   - link back to #35 and the #35 PR.
   - evaluate CLDR/x/text source, ambiguity, historical currency changes.
3. Separate long-backed `FastMoney`:
   - milestone: later than `0.6.0`.
   - metadata: assignee `debop`, labels `priority: p2`, `area: utilities`, `type: research`.
   - link back to #35 and the #35 PR.
   - require benchmark evidence against current `Money` minor-unit path and a clear API need.

## 10. Risks and failure modes

| Risk | Severity | Mitigation |
|---|---|---|
| Silent currency coercion corrupts monetary totals. | P0 | Every binary same-currency operation checks currency equality and returns `ErrCurrencyMismatch`. |
| Float constructor implies financial exactness. | P1 | README and docs prefer string/minor-unit constructors; tests assert deterministic documented behavior. |
| JSON/text format changes after release. | P1 | Define stable shape before implementation and test round-trip compatibility. |
| Upstream dependency error semantics leak too directly. | P1 | Wrap/map upstream errors into repo-owned sentinels. |
| Provider-backed conversion hides network/cache failures. | P1 | Defer providers to follow-up with context-aware design. |
| Locale lookup overpromises CLDR completeness. | P2 | Limit #35 locale support to documented common/current-region mapping and record follow-up. |
| Separate `FastMoney` doubles API surface prematurely. | P2 | Use minor-unit constructors/extractors first; require benchmark-driven follow-up. |

## 11. Acceptance criteria mapping

| Issue criterion | Spec decision |
|---|---|
| Compare candidate packages. | Section 3 compares precision, rounding, serialization, maintenance, license, concurrency/API shape. |
| Decide implement/adopt/defer. | Section 4 and 5 adopt `govalues/money` core with owned API layer. |
| Money implemented if acceptable. | Implement `Money`, `Currency`, `ExchangeRate`, constructors, operations, parsing/formatting, serialization. |
| FastMoney/minor-unit decision. | Implement minor-unit path; defer separate `FastMoney` type. |
| Currency units. | Implement code validation/constants and limited locale lookup. |
| Rounding and aggregation. | Implement currency-scale round/round-to and `Sum`. |
| Exchange-rate conversion. | Implement caller-supplied `ExchangeRate`; defer providers with follow-up issue link. |
| Tests. | Unit, table, serialization, parsing, error, goroutine stress, race tests. |
| README. | Document precision, rounding, serialization, not accounting system. |
| Stress helper requirement. | `GoroutineStressTester` required; `AsyncJobTester` required only if context-aware API is added, otherwise N/A evidence note. |

## 12. Draft task list

1. Add `github.com/govalues/money v0.2.4` and transitive decimal dependency through `go get`.
2. Implement `money` package public types, sentinels, constructors, currency helpers, and locale helper.
3. Implement arithmetic, rounding, comparison, aggregation, minor-unit extraction, and exchange-rate conversion.
4. Implement JSON/text marshal/unmarshal and parsing/formatting.
5. Create follow-up issues for provider-backed exchange rates, full locale mapping, and optional `FastMoney`, then use their links in README and PR body.
6. Add table-driven unit tests for success/failure/zero-value/currency mismatch/serialization/minor units/rounding, scalar arithmetic, `Sum` empty/success/mixed-currency/zero-value cases, exchange-rate direct/reverse/invalid/mismatch/overflow cases, float NaN/Inf rejection, locale normalization, and bounded oversized parse/unmarshal inputs.
7. Add runnable `money_example_test.go` examples for construction, arithmetic/mismatch handling, JSON/text round trip, aggregation, and caller-supplied exchange-rate conversion.
8. Add `GoroutineStressTester` tests and race validation. Add explicit `AsyncJobTester N/A` review evidence unless a context-aware API is added.
9. Update package/root README pairs, `CHANGELOG.md`, `WIP.md`.
10. Run `go mod tidy`, `git diff --check`, `golangci-lint config verify` when available/configured, `go test -count=1 ./money`, `go test -race -count=1 ./money`, and `make ci`.
11. Run Step 6-R 7-Tier review, fix P0/P1, then create PR with metadata matching issue #35.

## 13. Open questions

No blocking user question remains. Material design choices are explicit and will be challenged in Step 2-R:

- Wrapper API over type alias.
- No standalone `FastMoney` in #35.
- Caller-supplied exchange rate only, provider-backed support deferred.
- `AsyncJobTester` N/A unless context-aware API is introduced.
