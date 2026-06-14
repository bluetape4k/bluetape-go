# Issue #179 Locale Currency Mapping Design

## Goal

`money.CurrencyByLocale`를 작은 수동 region map에서 `golang.org/x/text`의 BCP47/CLDR 데이터 기반 lookup으로 확장한다. 이 API는 현재 지역의 대표 통화를 편의상 찾는 함수로 유지하며, 회계/세무/법정통화 권위를 대체하지 않는다.

## Source Evidence

- GitHub issue: #179 `Add full locale-to-currency mapping for money`
- Existing implementation: `money/currency.go`
  - `CurrencyByLocale(tag string) (Currency, error)`
  - 현재 `localeRegion` + `regionCurrencies` map으로 `KR`, `US`, `JP`, `CN`, 일부 EUR 지역만 지원한다.
- Existing tests: `money/currency_test.go`
  - common locale, unsupported tag, invalid region coverage가 있다.
- Existing docs: `money/README.md`, `money/README.ko.md`
  - full locale mapping은 #179 deferred로 표시되어 있다.
- Official Go docs evidence:
  - `golang.org/x/text/language.Parse` parses and canonicalizes BCP47 tags.
  - `language.Tag.Region()` can infer likely regions, and that inference is subject to change.
  - `golang.org/x/text/currency.FromRegion` reports the current legal tender for a region according to CLDR.
  - `currency.Query(currency.Region(r))` enumerates legal tender units for a region, including multi-tender regions.

## Diagram

![money locale currency resolution flow](../../images/readme-diagrams/money-locale-currency-resolution-flow.png)

Diagram source and evidence:

- Generator: `scripts/generate-money-locale-currency-diagram.mjs`
- Final assets:
  - `docs/images/readme-diagrams/money-locale-currency-resolution-flow.svg`
  - `docs/images/readme-diagrams/money-locale-currency-resolution-flow.png`
- Graphviz evidence:
  - `docs/images/readme-diagrams/money-locale-currency-resolution-flow.dot`
  - `docs/images/readme-diagrams/money-locale-currency-resolution-flow.plain`
  - `docs/images/readme-diagrams/money-locale-currency-resolution-flow-graphviz.svg`
  - `docs/images/readme-diagrams/money-locale-currency-resolution-flow-graphviz.png`
- Geometry gate:
  - `nodes=9 routes=8 segments=10`
  - `badEndpointAngle=0 badBends=0 interiorCrossings=0 nodeOverlaps=0 laneClearance=0`
  - `margins=L48/R48/T48/B48 titleGap=58`
- Visual gate: rendered PNG inspected; labels, cards, footer, and connector lanes do not overlap.

## Chosen Approach

Use `golang.org/x/text/language` for tag parsing and `golang.org/x/text/currency.Query` for current legal tender lookup.

Resolution policy:

1. Parse the input as a BCP47 tag after trimming spaces and normalizing `_` to `-`.
2. Extract only an explicitly supplied region subtag.
3. Reject tags without an explicit region.
4. Query current CLDR legal tender units for the region.
5. Return a `Currency` only when exactly one current tender currency exists.
6. Reject zero-tender and multi-tender regions with `ErrInvalidCurrency`.
7. Preserve existing sentinel error behavior so callers can use `errors.Is(err, ErrInvalidCurrency)`.

This keeps compatibility with #35 behavior: `ko`, `und`, `en-001`, and unknown regions remain invalid instead of silently guessing.

## Rejected Approaches

### Approach 2: Use `currency.FromTag` Directly

Rejected because `FromTag` can infer a likely region when none is explicitly present. That would make `CurrencyByLocale("en")` return a guessed currency and would break the current “region required” behavior.

### Approach 3: Generate a Repo-Local CLDR Snapshot

Rejected for this issue because it adds generator maintenance and data update policy before the repo needs custom CLDR curation. `golang.org/x/text` is already in the dependency graph and carries the CLDR-backed region/currency data needed for #179.

### Approach 4: Keep Expanding the Manual Map

Rejected because it cannot satisfy “full locale-to-currency mapping” without becoming stale and incomplete. It also cannot represent historical/no-tender/multi-tender ambiguity without additional metadata.

## API Contract

No new public function is required for the first implementation slice.

`CurrencyByLocale(tag string) (Currency, error)` remains the public entry point.

Contract details:

- Accepts BCP47 tags using hyphen or underscore separators.
- Requires an explicit two-letter or three-letter region subtag.
- Uses current CLDR legal tender data.
- Returns `ErrInvalidCurrency` for invalid syntax, missing region, unsupported/no-tender region, multi-tender ambiguity, and no-currency results.
- Does not return historical currencies by default.
- Does not decide accounting, trading, tax, settlement, or jurisdiction policy.

## Examples

Expected successful cases:

- `ko-KR` -> `KRW`
- `en_US` -> `USD`
- `ja-JP` -> `JPY`
- `de-DE` -> `EUR`
- `en-GB` -> `GBP`
- `fr-CA` -> `CAD`

Expected rejected cases:

- `ko` -> missing explicit region
- `und` -> missing explicit region
- `en-001` -> world region / no direct tender
- `aa-BB` -> unknown region
- `es-PA` -> multi-tender region, caller must choose `PAB` or `USD`
- `en-AQ` -> no current legal tender

## Testing Strategy

Add table-driven tests for:

- Existing #35 compatibility cases.
- New common current-region cases such as `en-GB`, `fr-CA`, `en-AU`, `pt-BR`, `hi-IN`, and `es-MX`.
- Separator/case normalization.
- Missing explicit region.
- Unknown region.
- No-tender region.
- Multi-tender ambiguity.
- Error wrapping with `errors.Is(err, ErrInvalidCurrency)`.

Concurrency/race coverage:

- Use repo-local `testing/concurrency.GoroutineStressTester` for concurrent `CurrencyByLocale` calls across success and rejection paths.
- Run `go test -race -count=1 ./money ./testing/concurrency`.

Validation commands:

```bash
node scripts/generate-money-locale-currency-diagram.mjs
go test -count=1 ./money ./testing/concurrency
go test -race -count=1 ./money ./testing/concurrency
go test -count=1 ./...
make fmt-check
make tidy-check
make vet
make lint
make ci
```

## Documentation Strategy

Update `money/README.md` and `money/README.ko.md`:

- Replace the #179 deferred row with the active `CurrencyByLocale` guidance.
- Explain that locale mapping is a current-region convenience.
- Document that ambiguous or no-tender regions are rejected.
- Keep English diagram assets shared across localized README files if the diagram is embedded in README.

Update root README package summary only if the public money summary needs the fuller locale mapping called out.

## Risks And Mitigations

| Risk | Severity | Mitigation |
|---|---:|---|
| Language-only tags start returning guessed currencies. | P1 | Require explicit region; do not use likely-region inference. |
| Multi-tender regions silently choose one currency. | P1 | Count current legal tender units with `currency.Query`; reject when count is not exactly one. |
| CLDR data changes in a future `x/text` update. | P2 | Document source and update strategy; tests cover representative policy cases rather than every region. |
| Dependency status is hidden because `x/text` is currently indirect. | P2 | Import `golang.org/x/text/language` and `golang.org/x/text/currency` directly; `go mod tidy` should make it direct if needed. |
| Docs imply legal/accounting authority. | P1 | README caveat and diagram footer state that callers own legal/accounting policy. |

## Acceptance Criteria

- `CurrencyByLocale` is backed by `x/text` BCP47 and CLDR tender data.
- Backward-compatible supported cases still pass.
- Unsupported, historical/no-tender, and ambiguous regions are covered by tests.
- README files explain the current-region convenience boundary.
- Goroutine stress and race tests cover the lookup contract.
- Diagram assets are generated, rendered, inspected, and referenced from this spec.

## Spec Self-Review

- Placeholder scan: PASS. No unresolved placeholder remains.
- Internal consistency: PASS. The contract, diagram, and testing strategy all use explicit-region plus CLDR current tender lookup.
- Scope check: PASS. This is one focused `money` package change, not a new module.
- Ambiguity check: PASS. Missing region, no-tender region, and multi-tender region behavior are explicitly specified.
