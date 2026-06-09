# money

[English](README.md) | [한국어](README.ko.md)

`money` provides Go-native value APIs for ISO 4217 currencies, decimal-backed
money amounts, aggregation, serialization, and caller-supplied exchange-rate
conversion.

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
| Exchange conversion | `NewExchangeRate` and `Convert` | Rates are caller-supplied; provider-backed fetching is deferred to #178. |
| Full locale mapping | Deferred | Limited common locale lookup exists now; full mapping is tracked in #179. |
| Long-backed FastMoney | Deferred | Benchmark-driven evaluation is tracked in #180. |

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

## Behavior

- `money` is not a full accounting system. It does not provide ledgers,
  posting rules, tax policy, financial calendars, provider-backed FX lookup, or
  jurisdiction-specific rounding policies.
- The precision model is based on `github.com/govalues/money` and
  `github.com/govalues/decimal`. Values are immutable and decimal-backed, but
  they are not arbitrary-precision unbounded numbers.
- Public API types are bluetape-go wrappers. Callers do not depend on upstream
  concrete types in public signatures.
- Zero-value `Money{}`, `Currency{}`, and `ExchangeRate{}` are invalid sentinel
  values. Use `Zero(currency)` when a valid zero money amount is needed.
- `ParseCurrency("XXX")`, `ParseCurrency("999")`, and construction with
  no-currency values return `ErrInvalidCurrency`.
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

## Test

```bash
go test -count=1 ./money
go test -race -count=1 ./money
```
