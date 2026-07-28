# Issue #178 Money Exchange Rate Providers Design

> 한국어 요구사항 경계: 이 spec/design/test-spec 문서는 한국어 독자가 요구사항을 추적할 수 있도록 목적과 검증 경계를 한국어로 보강한다. API 이름, command, code identifier, issue/PR 번호, compatibility matrix, acceptance keyword, DoD/test evidence는 요구사항 약화를 막기 위해 원문 그대로 보존한다. 변경자는 아래 literal contract를 삭제하거나 의미를 약하게 바꾸지 않아야 한다.


## 1. 문제 정의

Issue #178은 #35에서 의도적으로 미룬 provider-backed exchange-rate conversion을 `money` package에 추가한다.

목표는 회계 시스템이나 trading-rate engine이 아니라, 현재 `money.ExchangeRate` core 위에 다음을 얹는 것이다.

- `context.Context` 기반 provider API.
- 명시적 timeout, retry, cache freshness, fallback semantics.
- provider source/observation metadata가 포함된 quote 값.
- network, cache, stale data, parse, unsupported currency failure를 caller에게 숨기지 않는 error contract.
- ECB daily reference-rate provider의 첫 구현.

## 2. 현재 근거

### GitHub issue

- `gh issue view 178` 확인 결과, #178은 milestone `0.6.1`, labels `priority: p1`, `area: utilities`, `type: research`, assignee `debop`이다.
- Acceptance는 provider interface/default behavior 문서화, timeout/cancellation/error path 테스트, README freshness/failure/non-accounting boundary 설명을 요구한다.
- 구현은 #35의 caller-supplied `ExchangeRate` core landing 이후 follow-up이다.

### Current source

- `money/exchange_rate.go`는 `NewExchangeRate(base, quote Currency, rate string)`와 `Convert(amount Money, rate ExchangeRate)`를 제공한다.
- `Convert`는 value-only API이며 `context`, network, cache, provider semantics를 받지 않는다.
- `money/errors.go`에는 `ErrInvalidExchangeRate`까지 존재하지만 provider/cache/network/stale error는 아직 없다.
- `money/README.md`와 `money/README.ko.md`는 provider-backed FX lookup을 #178로 미룬다고 명시한다.
- Baseline test:

```bash
go test -count=1 ./money ./testing/concurrency
```

### Prior design

- #35 spec은 `govalues/money`를 core engine으로 채택하고, public API는 `bluetape-go` wrapper가 소유하도록 결정했다.
- #35 Step 2-R review는 provider-backed exchange-rate가 network/cache failure를 숨기면 P1이라고 기록했다.
- `testing/concurrency.AsyncJobTester`는 context-aware async cancellation/deadline test에 이미 존재한다.

### Research evidence

- Repo-local research note: `docs/superpowers/research/2026-06-14-issue-178-money-exchange-rate-providers-research.md`.
- Wiki preservation note: `/Users/debop/work/bluetape4k/bluetape4k-wiki/research/2026-06-14-bluetape-go-money-exchange-rate-provider-research.md`.
- Wiki evidence: `gno update`, `gno embed --collection bluetape4k-wiki`, and `gno search "bluetape-go money exchange-rate provider ECB IMF" -c bluetape4k-wiki -n 5 --files` found the note.

## 3. Provider source decision

### ECB

Adopt for the first implementation.

Evidence:

- ECB publishes euro reference rates around 16:00 CET on working days, except TARGET closing days.
- Rates are quoted against EUR as base currency.
- ECB states these reference rates are for information purposes and discourages transaction use.
- The daily XML endpoint is compact and stable enough for a first-party standard-library parser.
- The Data Portal API supports richer CSV/JSON query patterns, but the daily XML endpoint is simpler for the first provider.

Implication:

- `ECBProvider` fetches daily EUR-base rates from `https://www.ecb.europa.eu/stats/eurofxref/eurofxref-daily.xml`.
- EUR/EUR is synthesized as rate `1`.
- Non-EUR cross rates are computed from one ECB snapshot with source metadata preserved.
- Quote freshness is caller-visible because weekends and TARGET closing days can produce older observations.

### IMF

Defer to follow-up #231.

Reason:

- IMF uses SDMX 2.1/3.0 APIs and its Exchange Rates dataset includes historical, USD, SDR, EUR, national-currency, period-average, and end-of-period semantics.
- Those semantics are valuable but heavier than the first provider contract.
- #231 will evaluate IMF-specific source metadata and rate families after #178 stabilizes the provider API.

### Bloomberg

Defer to follow-up #232.

Reason:

- Bloomberg access depends on licensed enterprise products/API surfaces and entitlement/security/deployment topology.
- Provider behavior must account for credentials, entitlements, data usage monitoring, and CI without real Bloomberg credentials.
- #232 will evaluate Bloomberg-specific contract tests and integration boundaries after #178.

## 4. Design diagram

```mermaid
flowchart LR
    Caller[Caller] -->|context + Money + target Currency| ConvertWithProvider
    ConvertWithProvider[ConvertWithProvider]
    ConvertWithProvider -->|Rate(ctx, base, quote)| Provider[ExchangeRateProvider]
    Provider --> Cache{Fresh cache?}
    Cache -->|hit| Quote[ExchangeRateQuote]
    Cache -->|miss/stale| Retry[Fetch with timeout/retry]
    Retry --> HTTP[ECB daily XML]
    HTTP --> Snapshot[ECB snapshot\nobserved + fetched]
    Snapshot --> Quote
    Quote --> Core[Convert(Money, ExchangeRate)]
    Core --> Result[Money + Quote]
    Retry -->|network/parse/context failure| Error[typed provider error]
    Cache -->|stale fallback allowed| Quote
    Cache -->|stale fallback disallowed| Error
```

Diagram intent: caller-visible provider IO, cache freshness, retry/fallback, and value-only conversion boundaries remain separate.

## 5. 설계 옵션

### Option 1: First-party provider contract + ECB provider

Add a small `ExchangeRateProvider` contract and a first-party ECB daily provider using `net/http`, `encoding/xml`, and existing `NewExchangeRate`/`Convert`.

장점:

- No new dependency.
- `context`, timeout, retry, cache, source metadata를 repo가 직접 통제한다.
- Failure modes를 value-only `Convert`와 분리할 수 있다.
- ECB daily XML shape가 작아 unit/fake HTTP tests가 쉽다.

단점:

- Historical data, IMF semantics, commercial providers는 후속으로 분리된다.
- Cross-rate computation을 wrapper가 직접 소유해야 한다.

결정: Adopt. 사용자가 2026-06-14에 Approach 1을 승인했다.

### Option 2: General multi-provider framework from the start

ECB, IMF, Bloomberg-style provider를 모두 수용하는 generic framework를 먼저 설계한다.

장점:

- 장기 확장 지점을 한 번에 고민할 수 있다.
- provider별 metadata 차이를 초기부터 모델링할 수 있다.

단점:

- #178의 P1 범위가 커진다.
- IMF/Bloomberg는 data family, credential, entitlement semantics가 ECB와 달라 first slice를 지연시킨다.

결정: Reject for #178. IMF는 #231, Bloomberg는 #232로 분리한다.

### Option 3: Existing Go exchange-rate package dependency

`openprovider/ecbrates`, `go-finance`, `exrates`, `goexrates`, `ecbratex` 같은 package를 채택한다.

장점:

- 초기 구현량이 줄어들 수 있다.

단점:

- 조사한 후보들이 archived, stale, unreleased, broader finance toolkit, low adoption 중 하나에 걸렸다.
- Provider contract의 timeout/cache/failure semantics를 dependency가 충분히 보장하지 않는다.

결정: Reject. 첫 구현은 표준 라이브러리 기반 first-party provider로 간다.

## 6. Public API design

### Provider contract

```go
type ExchangeRateProvider interface {
    Rate(ctx context.Context, base Currency, target Currency) (ExchangeRateQuote, error)
}
```

Rules:

- `ctx == nil`은 repo의 `cache`, `concurrency`, `workflow`, `ratelimit`, `resilience` package 관례와 같이 `context.Background()`로 normalize한다.
- Invalid `Currency{}` or no-currency input returns `ErrInvalidCurrency`.
- Same-currency rate returns valid rate `1` without network fetch.
- Provider must not return a quote with invalid `ExchangeRate`.
- Provider implementations wrap causal errors with `%w`.

### Quote value

```go
type ExchangeRateQuote struct {
    Rate       ExchangeRate
    Source     string
    ObservedAt time.Time
    FetchedAt  time.Time
    ExpiresAt  time.Time
    Stale      bool
    RefreshError error
}
```

Rules:

- `Source` for the first implementation is `ECB`.
- `ObservedAt` is the provider observation date from ECB.
- `FetchedAt` is the local fetch time.
- `ExpiresAt` is derived from cache freshness policy.
- `Stale` is true only when an allowed stale fallback returned an older snapshot after a provider/cache/fetch problem.
- `RefreshError` is nil for fresh quotes and set to the wrapped refresh/fetch failure when stale fallback is returned. Callers can use `errors.Is` on it.
- Zero-value `ExchangeRateQuote{}` is invalid.

### Provider-backed conversion

```go
func ConvertWithProvider(ctx context.Context, amount Money, target Currency, provider ExchangeRateProvider) (Money, ExchangeRateQuote, error)
```

Rules:

- This is deliberately separate from `Convert(Money, ExchangeRate)`.
- It normalizes nil context with `context.Background()`.
- It rejects nil or typed-nil providers with `ErrExchangeRateProvider`.
- It fetches a quote with `provider.Rate(ctx, amount.Currency(), target)`.
- It then calls existing `Convert(amount, quote.Rate)`.
- It returns both converted money and the quote used.
- It does not hide provider/cache/network/stale errors behind a zero `Money`.

### ECB provider construction

```go
type ECBProviderOptions struct {
    Client            *http.Client
    Endpoint          string
    Timeout           time.Duration
    CacheTTL          time.Duration
    MaxStale          time.Duration
    RetryCount        int
    RetryBackoff      time.Duration
    AllowStaleOnError bool
    Now               func() time.Time
}

func NewECBProvider(options ECBProviderOptions) (*ECBProvider, error)
```

Rules:

- Defaults are explicit and documented in README.
- `Client` is optional; if nil, provider creates or uses a safe client path with timeout behavior controlled by options/context.
- `Endpoint` defaults to ECB daily XML and is injectable for tests.
- `Timeout` applies per fetch and must not override a stricter caller deadline.
- `RetryCount` excludes the first attempt.
- Retry must not retry `context.Canceled` or caller-owned `context.DeadlineExceeded`.
- `Now` is injectable for deterministic freshness tests.
- Negative `Timeout`, `CacheTTL`, `MaxStale`, `RetryCount`, or `RetryBackoff` values are invalid and return `ErrExchangeRateProvider`.
- Empty endpoint after trimming is invalid; non-HTTP(S) endpoint schemes are invalid unless a test-only `httptest` URL uses HTTP.

## 7. Error contract

Add provider-specific sentinel errors:

```go
var (
    ErrExchangeRateProvider = errors.New("money: exchange rate provider")
    ErrExchangeRateUnavailable = errors.New("money: exchange rate unavailable")
    ErrExchangeRateStale = errors.New("money: exchange rate stale")
    ErrUnsupportedExchangeRate = errors.New("money: unsupported exchange rate")
)
```

Rules:

- Network, HTTP status, XML parse, and malformed provider payload errors wrap `ErrExchangeRateProvider`.
- Missing currency in a provider snapshot returns `ErrUnsupportedExchangeRate`.
- Cache miss with no successful fetch returns `ErrExchangeRateUnavailable`.
- Stale snapshot without stale fallback permission returns `ErrExchangeRateStale`.
- Caller cancellation/deadline preserves `errors.Is(err, context.Canceled)` or `errors.Is(err, context.DeadlineExceeded)`.

## 8. Cache and freshness semantics

- Cache unit is provider snapshot, not per-pair quote. One ECB XML fetch can serve all currency pairs in the snapshot.
- Cache must be protected by `sync.RWMutex` or equivalent.
- Fresh cache hit returns without HTTP.
- Stale cache with `AllowStaleOnError=false` returns stale error if fetch cannot refresh.
- Stale cache with `AllowStaleOnError=true` may return stale quote after failed refresh, but `ExchangeRateQuote.Stale` must be true and `ExchangeRateQuote.RefreshError` must expose the wrapped refresh failure.
- The provider must not start background goroutines for refresh in #178. All IO is caller-driven through `Rate(ctx, ...)`.

## 9. ECB cross-rate semantics

ECB rates are EUR-base values: `1 EUR = N quote`.

Supported conversions:

- EUR -> X: use ECB rate for X.
- X -> EUR: use same `ExchangeRate` and existing reverse conversion.
- X -> Y: compute `Y per X = ecb[Y] / ecb[X]`, then call `NewExchangeRate(X, Y, computedRate)`.
- X -> X: synthesize `1`.

Constraints:

- Use `govalues` decimal/money behavior through existing `NewExchangeRate`; do not introduce float math.
- Unsupported or absent ECB currencies return `ErrUnsupportedExchangeRate`.
- Same-currency conversion does not hit network.

## 10. Tests

Required tests:

- Interface and nil/invalid provider behavior.
- ECB options validation for negative durations, negative retry count, empty endpoint, invalid endpoint scheme, and nil clock fallback.
- Same-currency no-fetch path.
- ECB XML success path for EUR->USD, USD->EUR, and USD->KRW cross rate.
- Unsupported currency in snapshot.
- HTTP non-2xx and body close.
- Malformed XML and malformed rate value.
- Cache fresh hit avoids second HTTP request.
- Stale disallowed returns `ErrExchangeRateStale`.
- Stale allowed returns quote with `Stale=true` and non-nil `RefreshError`.
- Retry succeeds after transient failure.
- Retry stops on `context.Canceled` and caller deadline.
- `ConvertWithProvider` returns money plus quote and does not swallow provider errors.
- Race/stress test for concurrent `Rate` calls and cache refresh using `testing/concurrency.GoroutineStressTester`.
- Cancellation/deadline coverage with `testing/concurrency.AsyncJobTester`.

Validation commands:

```bash
go test -count=1 ./money ./testing/concurrency
go test -race -count=1 ./money ./testing/concurrency
go test -count=1 ./...
git diff --check
make ci
```

## 11. Documentation

Update:

- `money/README.md`
- `money/README.ko.md`
- `money/doc.go`
- Root `README.md`
- Root `README.ko.md`

Docs must state:

- ECB provider uses ECB euro reference rates.
- ECB rates are informational; this is not a trading, accounting, ledger, tax, or financial-calendar system.
- Provider-backed conversion is context-aware and separate from value-only `Convert`.
- Freshness, stale fallback, timeout, retry, and source metadata are caller-visible.
- IMF provider is follow-up #231.
- Bloomberg provider is follow-up #232.

README diagram impact:

- If README adds provider flow visuals, use `$bluetape4k-diagram` and create final PNG/SVG assets under `docs/images/readme-diagrams/`.
- The final README asset must not be raw Mermaid or plain Graphviz. It must follow the diagram skill's source-derived model, PNG render, and visual validation gates.

## 12. Risks and controls

| Risk | Priority | Control |
|---|---:|---|
| Provider IO is hidden behind value-only API. | P1 | Keep `Convert` unchanged; provider conversion requires `context` and returns quote/error. |
| Retry ignores caller cancellation. | P1 | Do not retry `context.Canceled` or caller `context.DeadlineExceeded`; test with `AsyncJobTester`. |
| Stale ECB data is mistaken for fresh data. | P1 | Quote exposes observed/fetched/expires/stale metadata. README documents TARGET/weekend behavior. |
| Cross-rate computation uses float math. | P1 | Use decimal string/rational path and validate through `NewExchangeRate`. |
| Cache races under concurrent calls. | P1 | Guard snapshot state; prove with `GoroutineStressTester` and race test. |
| Provider dependency creates maintenance burden. | P2 | No new provider dependency in #178. |
| README overstates accounting/trading support. | P1 | Document non-accounting and informational-reference boundaries explicitly. |

## 13. Acceptance criteria

- Provider interface and ECB default behavior are documented.
- Timeout, cancellation, retry, cache miss/stale, network, parse, unsupported currency, and conversion error paths are tested.
- README pair documents freshness, failure modes, source metadata, and non-accounting-system boundaries.
- #231 and #232 are linked as follow-up provider expansions.
- Step 2-R, Step 3-R, Step 6-R, and Step 7-R use the 7-Tier frame. In this session, native subagents are replaced by main-role fallback per user instruction because native subagent lifecycle has repeatedly stalled; artifacts must record that fallback.
- Final gate requires `P0=0 P1=0`.

## 14. Out of scope

- IMF provider implementation (#231).
- Bloomberg provider implementation (#232).
- Persisted/disk/distributed cache.
- Background refresh goroutine.
- Paid/commercial provider credentials.
- Full accounting ledger, tax, posting, settlement, or financial-calendar behavior.
- Locale-to-currency expansion (#179).
- Long-backed FastMoney model (#180).
