# Issue #231 IMF 환율 Provider 연구

## 결정

`money.ExchangeRateProvider`용 좁은 IMF Exchange Rates provider를 추가한다.
첫 구현은 하나의 domestic currency와 하나의 USD/EUR pivot을 지원하고,
IMF ER rate family(`EOP_RT` 또는 `PA_RT`)와 frequency를 설정 가능하게 둔다.
순수 값 API인 `Convert`를 확장하지 않으며, domestic-to-domestic cross rate도
계산하지 않는다.

## 공식 소스 증거

- IMF API resource page:
  `https://data.imf.org/en/Resource-Pages/IMF-API`. IMF data는 SDMX 2.1과
  SDMX 3.0 API family로 제공되며, 새 endpoint root는 `https://api.imf.org`다.
- IMF Exchange Rates dataset page:
  `https://data.imf.org/en/datasets/IMF.STA%3AER`. ER dataset은 USD, SDR, EUR,
  national currency 사이의 historical exchange rate data를 포함하며,
  period-average와 end-of-period rate를 포함한다.
- IMF endpoint registry:
  `https://browser.sdmx.io/agencies/IMF`. Registry에는
  `https://api.imf.org/external/sdmx/2.1`와
  `https://api.imf.org/external/sdmx/3.0`가 있다.
- Live SDMX 2.1 structure discovery on 2026-06-21:
  - Dataflow: `IMF.STA:ER(4.0.1)`.
  - DSD: `IMF.STA:DSD_ER_PUB(4.0.0)`.
  - Dimensions: `COUNTRY`, `INDICATOR`, `TYPE_OF_TRANSFORMATION`, `FREQUENCY`.
  - Relevant indicators: `XDC_USD`, `USD_XDC`, `XDC_EUR`, `EUR_XDC`,
    `XDC_XDR`, `XDR_XDC`, `USD_XDR`, `XDR_USD`.
  - Transformations: `EOP_RT` end-of-period, `PA_RT` period average.
  - Frequencies: `D`, `M`, `Q`, `A`.

## Provider 계약

`NewIMFProvider`는 ECB provider 계약을 따른다.

- `Rate(ctx, base, target)`는 currency를 검증하고 caller cancellation/deadline을 존중한다.
- HTTP, parse, stale, unsupported pair, cancellation, deadline failure는
  `errors.Is`에서 sentinel error가 보이도록 유지한다.
- HTTP success response는 XML decode 전에 크기를 제한한다. HTTP error diagnostic은
  제한된 sanitized excerpt만 유지한다.
- `RetryCount`는 IMF 429와 5xx status failure만 재시도한다. Context error, 4xx
  failure, 결정적 parse/validation failure는 재시도하지 않는다.
- 성공한 quote는 `Source`, `ObservedAt`, `FetchedAt`, `ExpiresAt`, `Stale`,
  `RefreshError`를 채운다.
- Stale fallback은 `AllowStaleOnError`로 명시적으로 opt-in한다.
- Freshness는 caller가 `CacheTTL`과 `MaxStale`로 설정한다.

IMF 전용 source metadata는 `ExchangeRateQuote.Source`에 인코딩한다.

| Example | Meaning |
|---|---|
| `IMF ER:XDC_USD:EOP_RT:M` | Domestic currency per US dollar, end-of-period, monthly. |
| `IMF ER:USD_XDC:PA_RT:M` | US dollar per domestic currency, period-average, monthly. |
| `IMF ER:XDC_EUR:EOP_RT:Q` | Domestic currency per euro, end-of-period, quarterly. |

기존 `ExchangeRateQuote` 형태에는 structured source metadata map이 없다.
따라서 provider는 새 public field를 추가하지 않고 안정적인 source string에
source 세부 정보를 보존한다.

## 범위 경계

- USD와 EUR pivot을 먼저 구현한다. 현재 `github.com/govalues/money` backend로
  `money.Currency`가 두 currency를 생성하고 변환할 수 있기 때문이다.
- 기본 domestic-currency map은 의도적으로 작게 둔다: `AUD`, `CAD`, `CHF`,
  `CNY`, `GBP`, `JPY`, `KRW`. Caller는 `IMFProviderOptions.CountryCodes`를
  확장할 수 있지만, custom IMF country code는 URL construction 전에
  세 글자의 uppercase alphanumeric으로 제한한다.
- IMF ER은 SDR/XDR family를 게시하지만, 현재 package backend에서
  `ParseCurrency("XDR")`가 실패한다. SDR 공개는 안전한 public conversion path가
  되기 전에 별도의 currency backend 결정이 필요하다.
- 이 slice에서는 domestic-to-domestic cross rate를 계산하지 않는다. 이는 두 개의
  IMF country query와 source family, observation period alignment,
  stale mismatch handling에 대한 caller-visible semantic decision이 필요하다.
- 이 provider는 reference-data infrastructure다. trading-rate, accounting,
  ledger, tax, settlement, jurisdiction-specific rounding system이 아니다.

## 테스트 계획

- provider option과 safe default를 검증한다.
- USD/EUR domestic pivot pair에 대한 IMF SDMX path/query construction을 검증한다.
- same-currency no-fetch 동작을 검증한다.
- unsupported domestic-to-domestic 및 pivot-less pair를 검증한다.
- HTTP, XML, missing observation, invalid period, invalid rate, zero rate failure를
  검증한다.
- stale fallback이 `RefreshError`를 노출하고 너무 오래된 stale data를 거부하는지
  검증한다.
- cancellation과 provider-owned timeout을
  `testing/concurrency.AsyncJobTester`.
- `go test -count=1 ./money`, `go test -race -count=1 ./money`, `make ci`를 실행한다.

## 후속 작업

- Bloomberg-backed exchange-rate evaluation은 #232가 담당한다.
- 향후 XDR issue는 IMF API availability만이 아니라 현재 `Currency` backend support에서
  출발해야 한다.
