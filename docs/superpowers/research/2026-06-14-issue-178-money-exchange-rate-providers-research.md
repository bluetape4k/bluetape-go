# Issue #178 Money Exchange Rate Provider Research

## Scope

- Issue: #178, `Add provider-backed money exchange rates`
- Package: `money`
- Baseline: `origin/develop` at `9b5464a`
- Worktree: `.worktrees/issue-178-money-exchange-rate-providers`
- Date: 2026-06-14

## Current repo evidence

- `money/exchange_rate.go`는 현재 caller-supplied `ExchangeRate`와 `Convert(Money, ExchangeRate)`를 노출한다.
- `money/README.md`와 `money/README.ko.md`는 provider-backed FX lookup을 #178로 명시적으로 미룬다.
- #35 spec과 Step 2-R review는 provider behavior가 context-aware여야 하고 network/cache failure를
  value-only money API 뒤에 숨기면 안 된다고 요구한다.
- baseline command는 통과했다.

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

- ECB reference rate는 working day 약 16:00 CET에 published되며 EUR base currency 기준이다.
- ECB는 reference rate가 informational이며 transaction use를 권장하지 않는다고 경고한다.
- daily XML endpoint는 compact하고 단순하다. 하나의 `Cube time` date와 `Cube currency`/`rate`
  entry를 가진다.
- ECB Data Portal API는 `lastNObservations`, `startPeriod`, `endPeriod`, `format=csvdata`,
  conditional/update pattern 같은 query parameter를 지원한다.
- 2026-06-14 live smoke request는 USD, JPY, KRW 등을 포함한 `Cube time='2026-06-12'`를 반환했다.

Design implications:

- 첫 구현은 third-party dependency 없이 ECB daily-reference provider를 사용할 수 있다.
- EUR은 provider base로 취급해야 한다. EUR/EUR은 rate `1`로 synthesize한다.
- 같은 provider snapshot에서 non-EUR cross rate를 계산할 수 있지만, metadata는 source가 ECB이고
  ECB observation date가 무엇인지 기록해야 한다.
- provider docs는 ECB rate가 informational이고 accounting/trading system boundary가 아니라고 말해야 한다.
- weekend/TARGET closing day에는 오래된 observation이 나올 수 있으므로 freshness는 caller-visible해야 한다.

### IMF

Primary sources:

- https://data.imf.org/en/Resource-Pages/IMF-API
- https://data.imf.org/en/datasets/IMF.STA%3AER

Observed behavior:

- IMF data는 SDMX 2.1 및 SDMX 3.0 API를 통해 노출된다.
- IMF Exchange Rates dataset은 USD, SDR, EUR, national currency 사이의 historical data를 포함하며
  period-average와 end-of-period rate가 있다.
- API exploration은 현재 IMF portal/swagger surface를 경유한다.

Design implications:

- IMF는 historical/analytical coverage에는 가치가 있지만, 첫 provider로는 무겁다. 이 issue는 작은
  Go money package에서 timeout, retry, cache freshness, source semantics를 명시해야 한다.
- IMF는 첫 provider contract가 안정된 뒤 future provider candidate로 남긴다.

## Go package candidates

2026-06-14에 `go list -m -versions`와 `gh repo view`로 확인했다.

| Candidate | Versions | Repo status | Decision |
|---|---:|---|---|
| `github.com/openprovider/ecbrates` | no semver versions from `go list` | Not archived; MIT; latest release 2016; last push 2016. | 거절. dependency로 추가하기엔 너무 stale하다. |
| `github.com/pieterclaerhout/go-finance` | `v1.0.0`..`v1.0.4` | Not archived; Apache-2.0; last release 2019; last push 2025. | #178에서는 거절. 더 넓은 finance toolkit이고 release cadence가 오래됐다. ECB를 직접 parse하는 편이 작다. |
| `github.com/adrg/exrates` | `v0.0.1`..`v0.0.3` | Archived; MIT. | archived dependency라 거절한다. |
| `github.com/lmikolajczak/goexrates` | no semver versions from `go list` | Not archived; MIT; no latest release metadata. | 작은 package지만 release metadata가 없어 거절한다. |
| `github.com/jieggii/ecbratex` | no semver versions from `go list` | Not archived; MIT; low adoption and no releases. | 작고 unreleased라 public dependency로 부적합하다. |

## Recommended approach

first-party provider contract와 first-party ECB reference-rate provider를 구현한다.

1. rich `ExchangeRateQuote`를 반환하는 context-aware provider API를 추가한다.
2. `Convert`는 value-only로 유지하고, provider conversion은 explicit context-aware operation으로 추가한다.
3. timeout, max age, retry count, fallback cache, source name, HTTP client option을 추가한다.
4. ECB daily endpoint는 standard-library HTTP/XML parsing으로 처리한다.
5. stale/miss/fetch error를 숨기지 않는 in-memory snapshot cache를 사용한다.
6. cancellation/deadline path는 `testing/concurrency.AsyncJobTester`로 test한다.
7. cache/provider shared-state behavior는 `GoroutineStressTester`와 `go test -race`로 검증한다.

## Risks to carry into spec

| Risk | Severity | Control |
|---|---:|---|
| network/cache failure가 value-only conversion result로 숨겨진다. | P1 | provider API는 quote와 error를 반환하고 provider conversion은 context-aware로 유지한다. |
| weekend/TARGET closure로 오래된 ECB observation date가 나온다. | P1 | quote가 `ObservedAt`, `FetchedAt`, `Source`, freshness/staleness status를 노출한다. |
| retry loop가 caller cancellation을 무시한다. | P1 | retry는 caller `context.Canceled`와 `context.DeadlineExceeded`에서 멈춘다. test는 `AsyncJobTester`를 사용한다. |
| cross-rate math가 invalid same-currency 또는 no-currency rate를 만든다. | P1 | `NewExchangeRate`를 재사용하고 same-currency rate `1`을 synthesize하며 invalid `Currency{}`/`XXX`를 거절한다. |
| 새 dependency가 public maintenance burden을 늘린다. | P2 | 첫 구현에서는 provider dependency를 추가하지 않는다. |
