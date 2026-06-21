# money

[English](README.md) | [한국어](README.ko.md)

`money` provides Go-native value APIs for ISO 4217 currencies, decimal-backed
money amounts, aggregation, serialization, caller-supplied exchange-rate
conversion, and context-aware provider-backed conversion.

![money exchange-rate provider flow](../docs/images/readme-diagrams/money-exchange-rate-provider-flow.png)

## Import

```go
import "github.com/bluetape4k/bluetape-go/money"
```

## Selection Guide

| Need | Use | Notes |
|---|---|---|
| Currency code parsing | `ParseCurrency` or `MustParseCurrency` | Accepts ISO alphabetic or numeric codes, but rejects no-currency `XXX`/`999`. |
| Deterministic financial input | `New` | Prefer decimal strings such as `"12.34"`. |
| Major-unit integer input | `NewFromInt64` | `NewFromInt64(12, USD)` is `USD 12.00`; `NewFromInt64(12, KRW)` is `KRW 12`. |
| Minor-unit integer input | `NewMinor` | `NewMinor(12, USD)` is `USD 0.12`; `NewMinor(12, JPY)` is `JPY 12`. |
| Aggregation | `Sum` | Empty input returns the zero amount for the requested currency. |
| Caller-supplied exchange conversion | `NewExchangeRate` and `Convert` | Pure value path. No network, cache, or provider IO. |
| Provider-backed exchange conversion | `ExchangeRateProvider`, `NewECBProvider`, `NewIMFProvider`, and `ConvertWithProvider` | Context-aware provider path with source, freshness, stale fallback, and refresh failure metadata. |
| Locale-to-currency convenience | `CurrencyByLocale` | Uses explicit-region BCP47 tags and CLDR current legal tender data. Ambiguous or no-tender regions are rejected. |
| Long-backed FastMoney | Not added | #180 benchmark evidence keeps `Money` as the public API; use `NewMinor` and `MinorUnits` for minor-unit paths. |

## Money vs FastMoney

`Money` remains the public amount type. #180 measured the minor-unit and
representative hot paths and did not add a separate long-backed `FastMoney`
type. Use `NewMinor` for integer minor-unit input and `MinorUnits` for integer
extraction.

![money FastMoney evaluation decision flow](../docs/images/readme-diagrams/money-fastmoney-evaluation-decision-flow.png)

![money FastMoney evaluation benchmark](../docs/images/readme-charts/money-fastmoney-evaluation-benchmark.png)

The benchmark snapshot is local evidence, not a production ranking. The raw
output is stored in `docs/research/outputs/issue-180/money-fastmoney-evaluation-bench.txt`.

## Usage

```go
price, err := money.New("12.34", money.USD)
if err != nil {
    return err
}
tax, err := money.New("0.66", money.USD)
if err != nil {
    return err
}
total, err := price.Add(tax)
if err != nil {
    return err
}

payload, err := json.Marshal(total)
```

Provider-backed exchange conversion keeps IO explicit:

```go
provider, err := money.NewECBProvider(money.ECBProviderOptions{
    Timeout:           3 * time.Second,
    CacheTTL:          24 * time.Hour,
    MaxStale:          72 * time.Hour,
    AllowStaleOnError: true,
})
if err != nil {
    return err
}

usd, err := money.New("2.00", money.USD)
if err != nil {
    return err
}
krw, quote, err := money.ConvertWithProvider(ctx, usd, money.KRW, provider)
if err != nil {
    return err
}
if quote.Stale {
    log.Printf("using stale %s quote observed at %s: %v",
        quote.Source,
        quote.ObservedAt.Format(time.DateOnly),
        quote.RefreshError,
    )
}
_ = krw
```

IMF-backed rates use the same provider contract:

```go
provider, err := money.NewIMFProvider(money.IMFProviderOptions{
    RateFamily:        money.IMFRateEndOfPeriod,
    Frequency:         money.IMFFrequencyMonthly,
    Timeout:           3 * time.Second,
    CacheTTL:          24 * time.Hour,
    MaxStale:          72 * time.Hour,
    AllowStaleOnError: true,
})
if err != nil {
    return err
}
_ = provider
```

Locale currency mapping is a current-region convenience:

![money locale currency resolution flow](../docs/images/readme-diagrams/money-locale-currency-resolution-flow.png)

```go
currency, err := money.CurrencyByLocale("en-GB")
if err != nil {
    return err
}
_ = currency // GBP
```

## Behavior

- `money` is not a full accounting system. It does not provide ledgers,
  posting rules, tax policy, financial calendars, trading rates, settlement
  rules, or jurisdiction-specific rounding policies.
- `NewExchangeRate` and `Convert` remain pure value APIs. They never perform
  network or cache IO.
- `NewECBProvider` uses ECB euro reference rates from the daily XML endpoint.
  ECB reference rates are informational. Do not treat them as a trading,
  accounting, ledger, tax, or settlement authority.
- `NewIMFProvider` uses the IMF Exchange Rates SDMX API for configured
  period-average or end-of-period reference data. IMF data is provider-backed
  reference data, not a full accounting, trading-rate, tax, or settlement
  system.
- `ConvertWithProvider` is context-aware and returns the converted `Money`
  plus the `ExchangeRateQuote` used. The quote exposes `Source`, `ObservedAt`,
  `FetchedAt`, `ExpiresAt`, `Stale`, and `RefreshError`.
- ECB rates are EUR-base. The provider computes EUR direct rates, reverse
  rates, and non-EUR cross rates from the same snapshot without float math.
- IMF rates support one domestic currency and one USD/EUR pivot per request.
  The quote `Source` includes the IMF indicator, rate family, and frequency
  such as `IMF ER:XDC_USD:EOP_RT:M`. Domestic-to-domestic cross rates are a
  non-goal for this provider slice.
- The default IMF domestic-currency map covers `AUD`, `CAD`, `CHF`, `CNY`,
  `GBP`, `JPY`, and `KRW`. Extend `IMFProviderOptions.CountryCodes` when your
  caller needs another IMF country code mapping.
- By default, IMF requests use an 18-month monthly lookback window ending at
  `Now`. Set `StartPeriod` and `EndPeriod` together when callers need a fixed
  IMF period window.
- IMF ER also publishes SDR/XDR families, but this package's current
  `Currency` backend rejects `XDR`; SDR exposure is deferred until the currency
  backend can construct and convert XDR values safely.
- Weekends and TARGET closing days can leave the latest ECB observation older
  than the local fetch time. IMF publication cadence can also lag the local
  fetch time. Configure `CacheTTL`, `MaxStale`, and `AllowStaleOnError`
  according to your caller's risk tolerance.
- Provider failures are caller-visible. Network, HTTP, XML, stale, unsupported
  currency, cancellation, and deadline failures preserve sentinel errors for
  `errors.Is` checks.
- IMF provider retries apply only to HTTP `429` and `5xx` responses when
  `RetryCount` is set. Caller cancellation, caller-owned deadlines, `4xx`
  responses, malformed XML, and invalid observation values are not retried.
- Bloomberg-backed exchange rates are tracked in #232.
- The precision model is based on `github.com/govalues/money` and
  `github.com/govalues/decimal`. Values are immutable and decimal-backed, but
  they are not arbitrary-precision unbounded numbers.
- Public API types are bluetape-go wrappers. Callers do not depend on upstream
  concrete types in public signatures.
- Zero-value `Money{}`, `Currency{}`, and `ExchangeRate{}` are invalid sentinel
  values. Use `Zero(currency)` when a valid zero money amount is needed.
- `ParseCurrency("XXX")`, `ParseCurrency("999")`, and construction with
  no-currency values return `ErrInvalidCurrency`.
- `CurrencyByLocale` requires an explicit BCP47 region and uses CLDR current
  legal tender data through `golang.org/x/text/currency`.
- Locale mapping is a current-region convenience, not an accounting, trading,
  tax, settlement, or legal-tender authority. Regions with no current tender or
  multiple current tender currencies return `ErrInvalidCurrency` so callers can
  choose explicitly.
- Arithmetic requires matching currencies. Currency mismatch returns
  `ErrCurrencyMismatch` for `errors.Is` checks.
- `Round` uses currency-scale half-even rounding. `RoundTo` uses half-even
  rounding at the requested scale while still preserving the currency's minimum
  scale.
- `MinorUnits` returns the currency minor-unit integer after half-even
  currency-scale rounding; values outside `int64` return `ErrOverflow`.
- JSON serialization uses `{"amount":"12.34","currency":"USD"}`. Text
  serialization uses `USD 12.34`.
- Parsing and unmarshal helpers validate bounded money shapes, but HTTP request
  size, body size, and streaming limits remain caller-owned.
- `NewFromFloat64` is provided for ergonomics, but string and minor-unit
  constructors are preferred for deterministic financial input.
- Money values are immutable and safe to pass between goroutines. #35 validates
  representative operations with `testing/concurrency.GoroutineStressTester`
  and `go test -race`.
- #180 benchmark evidence keeps `Money` as the public amount type. A future
  `FastMoney` issue should require a measured hot-path gap plus a public caller
  contract that cannot be served by `NewMinor` and `MinorUnits`.

## Test

```bash
go test -count=1 ./money
go test -race -count=1 ./money
```
