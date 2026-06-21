# Issue #231 IMF Exchange-Rate Provider Research

## Decision

Add a narrow IMF Exchange Rates provider for `money.ExchangeRateProvider`.
The first implementation supports one domestic currency and one USD/EUR pivot,
with configurable IMF ER rate family (`EOP_RT` or `PA_RT`) and frequency. It
does not expand the pure `Convert` value API, and it does not compute
domestic-to-domestic cross rates.

## Official Source Evidence

- IMF API resource page:
  `https://data.imf.org/en/Resource-Pages/IMF-API`. IMF data is exposed through
  SDMX 2.1 and SDMX 3.0 API families, with the new endpoint rooted under
  `https://api.imf.org`.
- IMF Exchange Rates dataset page:
  `https://data.imf.org/en/datasets/IMF.STA%3AER`. The ER dataset contains
  historical exchange rate data between USD, SDR, EUR, and national currencies,
  including period-average and end-of-period rates.
- IMF endpoint registry:
  `https://browser.sdmx.io/agencies/IMF`. The registry lists
  `https://api.imf.org/external/sdmx/2.1` and
  `https://api.imf.org/external/sdmx/3.0`.
- Live SDMX 2.1 structure discovery on 2026-06-21:
  - Dataflow: `IMF.STA:ER(4.0.1)`.
  - DSD: `IMF.STA:DSD_ER_PUB(4.0.0)`.
  - Dimensions: `COUNTRY`, `INDICATOR`, `TYPE_OF_TRANSFORMATION`, `FREQUENCY`.
  - Relevant indicators: `XDC_USD`, `USD_XDC`, `XDC_EUR`, `EUR_XDC`,
    `XDC_XDR`, `XDR_XDC`, `USD_XDR`, `XDR_USD`.
  - Transformations: `EOP_RT` end-of-period, `PA_RT` period average.
  - Frequencies: `D`, `M`, `Q`, `A`.

## Provider Contract

`NewIMFProvider` follows the ECB provider contract:

- `Rate(ctx, base, target)` validates currencies and honors caller
  cancellation/deadlines.
- HTTP, parse, stale, unsupported pair, cancellation, and deadline failures keep
  sentinel errors visible to `errors.Is`.
- HTTP success responses are capped before XML decode. HTTP error diagnostics
  keep only a bounded sanitized excerpt.
- `RetryCount` retries IMF 429 and 5xx status failures. Context errors, 4xx
  failures, and deterministic parse/validation failures are not retried.
- Successful quotes fill `Source`, `ObservedAt`, `FetchedAt`, `ExpiresAt`,
  `Stale`, and `RefreshError`.
- Stale fallback is opt-in through `AllowStaleOnError`.
- Freshness is caller-configured through `CacheTTL` and `MaxStale`.

IMF-specific source metadata is encoded in `ExchangeRateQuote.Source`:

| Example | Meaning |
|---|---|
| `IMF ER:XDC_USD:EOP_RT:M` | Domestic currency per US dollar, end-of-period, monthly. |
| `IMF ER:USD_XDC:PA_RT:M` | US dollar per domestic currency, period-average, monthly. |
| `IMF ER:XDC_EUR:EOP_RT:Q` | Domestic currency per euro, end-of-period, quarterly. |

The existing `ExchangeRateQuote` shape has no structured source metadata map,
so the provider keeps source details in a stable source string instead of
adding new public fields.

## Scope Boundaries

- USD and EUR pivots are implemented first because `money.Currency` can
  construct and convert them with the current `github.com/govalues/money`
  backend.
- The default domestic-currency map is intentionally small: `AUD`, `CAD`,
  `CHF`, `CNY`, `GBP`, `JPY`, and `KRW`. Callers can extend
  `IMFProviderOptions.CountryCodes`, but custom IMF country codes are restricted
  to three uppercase alphanumeric characters before URL construction.
- IMF ER publishes SDR/XDR families, but `ParseCurrency("XDR")` currently
  fails through the package backend. Exposing SDR needs a separate currency
  backend decision before it can be a safe public conversion path.
- Domestic-to-domestic cross rates are not computed in this slice. They require
  two IMF country queries and a caller-visible semantic decision about source
  family, observation period alignment, and stale mismatch handling.
- The provider is reference-data infrastructure, not a trading-rate,
  accounting, ledger, tax, settlement, or jurisdiction-specific rounding
  system.

## Test Plan

- Validate provider options and safe defaults.
- Verify IMF SDMX path/query construction for USD/EUR domestic pivot pairs.
- Verify same-currency no-fetch behavior.
- Verify unsupported domestic-to-domestic and pivot-less pairs.
- Verify HTTP, XML, missing observation, invalid period, invalid rate, and zero
  rate failures.
- Verify stale fallback exposes `RefreshError` and rejects too-old stale data.
- Verify cancellation and provider-owned timeout with
  `testing/concurrency.AsyncJobTester`.
- Run `go test -count=1 ./money`, `go test -race -count=1 ./money`, and
  `make ci`.

## Follow-Up

- #232 owns Bloomberg-backed exchange-rate evaluation.
- A future XDR issue should start from current `Currency` backend support, not
  from IMF API availability alone.
