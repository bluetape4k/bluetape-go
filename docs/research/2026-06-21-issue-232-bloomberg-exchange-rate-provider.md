# Issue #232 Bloomberg 환율 Provider 평가

Issue: #232
Milestone: `0.6.2`
Date: 2026-06-21

## 결정

이번 milestone에서는 core `money` package에 기본 Bloomberg-backed provider,
dependency, credential path, paid-access test path를 추가하지 않는다.

Bloomberg-backed rate는 기존 `ExchangeRateProvider` 계약 뒤의 future licensed
adapter로만 지원할 수 있다. 해당 adapter는 customer application이 소유하거나,
customer-provided Bloomberg session/client/fetcher를 받는 별도 optional
integration boundary가 소유해야 하며, CI에서는 fake와 contract test로 동작을
증명해야 한다.

이미 Bloomberg Professional 또는 enterprise data access가 있는 팀에게 Bloomberg
경로는 여전히 가장 실용적인 premium-data option이다. 조직이 entitlements, field
convention, monitoring, support channel을 이미 갖고 있다면 새 public data source를
온보딩하는 것보다 운영상 빠를 수 있다. 이 결정은 그 경로를 열어 두되, licensed
customer infrastructure가 default package behavior가 되는 것은 막는다.

## 공식 소스 증거

- Bloomberg Server API (SAPI):
  `https://professional.bloomberg.com/products/data/data-connectivity/server-api/`.
  SAPI는 Bloomberg real-time market data, historical data, reference data,
  calculation capability를 소비하는 proprietary 또는 Bloomberg-approved server
  application용으로 설명된다. 해당 page는 entitlements management, activity
  monitoring, data usage monitoring, mutually authenticated SSL sessions,
  failover, load balancing, scalability, C++/.NET/VBA COM/Java/Python SDK도
  함께 명시한다.
- Bloomberg B-PIPE real-time market data feed:
  `https://professional.bloomberg.com/products/data/enterprise-catalog/real-time-data-feed/`.
  B-PIPE는 instrument, exchange, contributor, reference data, market-depth,
  entitlement management 범위를 넓게 갖는 consolidated real-time market data feed다.
- Bloomberg Data License:
  `https://professional.bloomberg.com/products/data/data-management/data-license/`.
  Data License는 REST API, SFTP, cloud delivery를 통해 enterprise reference,
  pricing, risk, historical 및 관련 dataset을 제공하며, Per Security와 Bulk
  delivery mode를 포함한다.
- Bloomberg BLPAPI documentation:
  `https://bloomberg.github.io/blpapi-docs/`. The public documentation index
  개발자를 C++, C#/.NET, Java, Python용 BLPAPI guide와 SDK documentation으로
  안내하며, SDK download는 Bloomberg Professional Services support 뒤에 있다.

## Access Model 적합성

| Access model | Fit for `ExchangeRateProvider` | Notes |
|---|---|---|
| SAPI / BLPAPI | Best conceptual fit for a request/response provider in licensed server applications; preferred when the caller already operates Bloomberg access. | Requires customer-owned Bloomberg product access, entitlements, authenticated network/session setup, and operational monitoring. |
| B-PIPE | Fit only when the caller already operates a real-time feed and maintains a local snapshot/cache. | Streaming feed semantics are broader than a simple exchange-rate lookup and require entitlement and usage-reporting ownership. |
| Data License | Fit for scheduled reference or historical datasets, not low-latency request-time conversion. | REST/SFTP/cloud delivery can populate caller-owned reference tables that a separate provider reads. |

따라서 지원 가능한 설계 방향은 optional adapter surface다. core package에
`money.NewBloombergProvider` 같은 새 default constructor를 추가하지 않는다.

## Provider 계약 명세

향후 adapter는 기존 provider 계약을 사용해야 한다.

- `Rate(ctx, base, target)` must honor caller cancellation and deadlines.
- `ConvertWithProvider` remains the only conversion integration point.
- The pure `NewExchangeRate` and `Convert` value path must stay network-free.
- Bloomberg-specific source details must be encoded in
  `ExchangeRateQuote.Source` unless a broader structured metadata API is added
  for all providers.

권장 source metadata 형태:

| Example | Meaning |
|---|---|
| `Bloomberg:SAPI:USD Curncy:PX_LAST:BGN` | SAPI request for a Bloomberg currency security and field using a configured pricing source. |
| `Bloomberg:BPIPE:EUR Curncy:BID:realtime` | B-PIPE-derived local snapshot from a real-time entitlement-controlled feed. |
| `Bloomberg:DataLicense:USD Curncy:PX_LAST:batch` | Data License batch or scheduled dataset source. |

정확한 security, field, pricing-source 선택은 caller-configured 상태로 남겨야 한다.
Bloomberg entitlement와 data-license term이 customer-specific이기 때문이다.

## Failure 매핑

| Failure | Required mapping |
|---|---|
| Caller cancellation or deadline | Return the original context error so `errors.Is` works. Do not serve stale data for caller-owned context failure. |
| Bloomberg session/authentication/configuration failure | Wrap `ErrExchangeRateProvider` with the underlying cause. |
| Entitlement or permission denial | Wrap `ErrExchangeRateProvider` and include a sanitized entitlement-denied diagnostic. Do not collapse it into nil or an empty rate. |
| Security/field/pair not available | Return `ErrExchangeRateUnavailable` when the requested data is valid but absent. Return `ErrUnsupportedExchangeRate` when the pair or configured Bloomberg mapping is not supported. |
| Stale snapshot beyond caller tolerance | Wrap `ErrExchangeRateStale` and keep the refresh failure visible. |
| Network, protocol, malformed response, or adapter failure | Wrap `ErrExchangeRateProvider` with sanitized bounded diagnostics. |

## Freshness 규칙

- `FetchedAt`는 local fetch 또는 snapshot read time을 기록한다.
- `ObservedAt`는 가능할 때 Bloomberg의 observation, tick, valuation, batch
  dataset timestamp를 기록한다.
- `ExpiresAt`는 caller-owned `CacheTTL`에서 도출한다.
- `Stale`과 `RefreshError`는 기존 ECB/IMF stale fallback 형태를 따른다.
- `MaxStale`과 `AllowStaleOnError`는 caller-controlled로 둔다.
- SAPI request/response, B-PIPE streaming snapshot, Data License batch record는
  하나의 implicit freshness model을 공유하면 안 된다.

## 테스트 전략

- Core `money` test는 Bloomberg credentials, Bloomberg SDK, paid product,
  network access를 요구하면 안 된다.
- Adapter behavior는 session lifecycle, entitlement, request/response data,
  stale cache fallback, malformed payload, field exception, cancellation,
  deadline, bounded diagnostic에 대한 fake로 테스트해야 한다.
- Optional live integration test를 나중에 추가하더라도 build tag와
  `BLOOMBERG_*` 같은 environment variable 뒤에 opt-in으로 두고, default CI
  밖에 둔다.
- README example은 adapter를 ready-to-use public data source가 아니라
  customer-owned infrastructure로 보여줘야 한다.

## Security 및 Operations

- 실제 Bloomberg credentials, certificate, UUID, terminal session, SAPI appliance
  detail, entitlement identifier를 문서화하거나 commit하지 않는다.
- mutual TLS, customer network topology, Bloomberg product contract,
  entitlement management, audit logging, data usage monitoring은 deployment
  prerequisite으로 취급한다.
- entitlement, usage, stale-data, cache, provider failure는 caller에게 보이게
  유지한다. 이는 incidental error가 아니라 operational control이다.

## 결과

#232는 Bloomberg access boundary를 문서화하고 구현을 defer하는 것으로 해결한다.
다음 구현 issue를 연다면 `money`의 default dependency가 아니라, fake-backed
contract test를 갖춘 별도 optional licensed adapter를 대상으로 해야 한다.
