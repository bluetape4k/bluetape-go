# Issue #178 Money Exchange Rate Provider Research

## Scope

- Issue: #178, `Add provider-backed money exchange rates`
- Package: `money`
- Baseline: `origin/develop` at `9b5464a`
- Worktree: `.worktrees/issue-178-money-exchange-rate-providers`
- Date: 2026-06-14

## Current repo evidence

- `money/exchange_rate.go` currently exposes caller-supplied `ExchangeRate` and `Convert(Money, ExchangeRate)`.
- `money/README.md` and `money/README.ko.md` explicitly defer provider-backed FX lookup to #178.
- #35 spec and Step 2-R review require provider behavior to be context-aware and to avoid hiding network/cache failures behind value-only money APIs.
- Baseline command passed:

```bash
go test -count=1 ./money ./testing/concurrency
```

## Official provider source evidence

### ECB

Primary sources:

- https://www.ecb.europa.eu/stats/policy_and_exchange_rates/euro_reference_exchange_rates/html/index.en.html
- https://www.ecb.europa.eu/stats/eurofxref/eurofxref-daily.xml
- https://data.ecb.europa.eu/help/api/data
- https://data-api.ecb.europa.eu/service/data/EXR/D.USD+JPY.EUR.SP00.A?lastNObservations=1&format=csvdata

Observed behavior:

- ECB reference rates are published on working days around 16:00 CET and are quoted against EUR as the base currency.
- ECB warns that the reference rates are informational and discourages transaction use.
- The daily XML endpoint is compact and simple: one `Cube time` date with `Cube currency` and `rate` entries.
- The ECB Data Portal API supports query parameters such as `lastNObservations`, `startPeriod`, `endPeriod`, `format=csvdata`, and conditional/update patterns.
- A live smoke request on 2026-06-14 returned `Cube time='2026-06-12'` with USD, JPY, KRW, and other rates.

Design implications:

- First implementation can use an ECB daily-reference provider with no third-party dependency.
- EUR must be treated as the provider base. EUR/EUR should be synthesized as rate `1`.
- Non-EUR cross rates can be computed from the same provider snapshot, but metadata must record ECB as the source and the ECB observation date.
- Provider docs must state that ECB rates are informational and not an accounting/trading system boundary.
- Freshness must be caller-visible because weekend/TARGET closing days produce older observations.

### IMF

Primary sources:

- https://data.imf.org/en/Resource-Pages/IMF-API
- https://data.imf.org/en/datasets/IMF.STA%3AER

Observed behavior:

- IMF data is exposed through SDMX 2.1 and SDMX 3.0 APIs.
- The IMF Exchange Rates dataset contains historical data between USD, SDR, EUR, and national currencies, including period-average and end-of-period rates.
- API exploration currently routes through IMF's portal/swagger surface.

Design implications:

- IMF is valuable for historical and analytical coverage, but it is too heavy for the first provider because the issue asks for explicit timeout, retry, cache freshness, and source semantics in a small Go money package.
- IMF should remain a future provider candidate after the first provider contract is stable.

## Go package candidates

Checked with `go list -m -versions` and `gh repo view` on 2026-06-14.

| Candidate | Versions | Repo status | Decision |
|---|---:|---|---|
| `github.com/openprovider/ecbrates` | no semver versions from `go list` | Not archived; MIT; latest release 2016; last push 2016. | Reject. Too stale to add as a dependency. |
| `github.com/pieterclaerhout/go-finance` | `v1.0.0`..`v1.0.4` | Not archived; Apache-2.0; last release 2019; last push 2025. | Reject for #178. Broader finance toolkit and older release cadence; parsing ECB directly is smaller. |
| `github.com/adrg/exrates` | `v0.0.1`..`v0.0.3` | Archived; MIT. | Reject. Archived dependency. |
| `github.com/lmikolajczak/goexrates` | no semver versions from `go list` | Not archived; MIT; no latest release metadata. | Reject. Small package, no release metadata. |
| `github.com/jieggii/ecbratex` | no semver versions from `go list` | Not archived; MIT; low adoption and no releases. | Reject. Too small and unreleased for public dependency. |

## Recommended approach

Implement a first-party provider contract and a first-party ECB reference-rate provider:

1. Add context-aware provider API returning a rich `ExchangeRateQuote`.
2. Keep `Convert` value-only and add provider conversion as an explicit context-aware operation.
3. Add provider options for timeout, max age, retry count, fallback cache, source name, and HTTP client.
4. Use standard-library HTTP/XML parsing for the ECB daily endpoint.
5. Use an in-memory snapshot cache that surfaces stale/miss/fetch errors instead of hiding them.
6. Test cancellation/deadline paths with `testing/concurrency.AsyncJobTester`.
7. Use `GoroutineStressTester` plus `go test -race` for cache/provider shared-state behavior.

## Risks to carry into spec

| Risk | Severity | Control |
|---|---:|---|
| Network/cache failure is hidden as a value-only conversion result. | P1 | Provider API returns quote plus error; provider conversion remains context-aware. |
| Weekend/TARGET closure produces older ECB observation date. | P1 | Quote exposes `ObservedAt`, `FetchedAt`, `Source`, and freshness/staleness status. |
| Retry loops ignore caller cancellation. | P1 | Retry stops on caller `context.Canceled` and `context.DeadlineExceeded`; tests use `AsyncJobTester`. |
| Cross-rate math produces invalid same-currency or no-currency rates. | P1 | Reuse `NewExchangeRate`; synthesize same-currency rate `1`; reject invalid `Currency{}`/`XXX`. |
| New dependency expands public maintenance burden. | P2 | Do not add a provider dependency for the first implementation. |
