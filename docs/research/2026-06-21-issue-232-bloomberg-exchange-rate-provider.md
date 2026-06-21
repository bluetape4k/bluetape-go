# Issue #232 Bloomberg Exchange-Rate Provider Evaluation

Issue: #232
Milestone: `0.6.2`
Date: 2026-06-21

## Decision

Do not add a default Bloomberg-backed provider, dependency, credential path, or
paid-access test path to the core `money` package in this milestone.

Bloomberg-backed rates can be supported only as a future licensed adapter behind
the existing `ExchangeRateProvider` contract. That adapter should be owned by a
customer application or a separate optional integration boundary that accepts a
customer-provided Bloomberg session/client/fetcher and proves behavior with
fakes and contract tests in CI.

For teams that already have Bloomberg Professional or enterprise data access,
the Bloomberg path is still the most practical premium-data option: it can be
faster operationally than onboarding a new public data source because the
organization already owns the entitlements, field conventions, monitoring, and
support channel. This decision keeps that path viable while preventing licensed
customer infrastructure from becoming default package behavior.

## Official Source Evidence

- Bloomberg Server API (SAPI):
  `https://professional.bloomberg.com/products/data/data-connectivity/server-api/`.
  SAPI is positioned for proprietary or Bloomberg-approved server applications
  that consume Bloomberg real-time market data, historical data, reference
  data, and calculation capabilities. The page also names entitlements
  management, activity monitoring, data usage monitoring, mutually
  authenticated SSL sessions, failover, load balancing, scalability, and SDKs
  for C++, .NET, VBA COM, Java, and Python.
- Bloomberg B-PIPE real-time market data feed:
  `https://professional.bloomberg.com/products/data/enterprise-catalog/real-time-data-feed/`.
  B-PIPE is a consolidated real-time market data feed with broad instrument,
  exchange, contributor, reference data, market-depth, and entitlement
  management coverage.
- Bloomberg Data License:
  `https://professional.bloomberg.com/products/data/data-management/data-license/`.
  Data License delivers enterprise reference, pricing, risk, historical, and
  related datasets through REST API, SFTP, and cloud delivery, including Per
  Security and Bulk delivery modes.
- Bloomberg BLPAPI documentation:
  `https://bloomberg.github.io/blpapi-docs/`. The public documentation index
  points developers to BLPAPI guides and SDK documentation for C++, C#/.NET,
  Java, and Python, with SDK downloads behind Bloomberg Professional Services
  support.

## Access Model Fit

| Access model | Fit for `ExchangeRateProvider` | Notes |
|---|---|---|
| SAPI / BLPAPI | Best conceptual fit for a request/response provider in licensed server applications; preferred when the caller already operates Bloomberg access. | Requires customer-owned Bloomberg product access, entitlements, authenticated network/session setup, and operational monitoring. |
| B-PIPE | Fit only when the caller already operates a real-time feed and maintains a local snapshot/cache. | Streaming feed semantics are broader than a simple exchange-rate lookup and require entitlement and usage-reporting ownership. |
| Data License | Fit for scheduled reference or historical datasets, not low-latency request-time conversion. | REST/SFTP/cloud delivery can populate caller-owned reference tables that a separate provider reads. |

The supported design direction is therefore an optional adapter surface, not a
new default constructor such as `money.NewBloombergProvider` in the core package.

## Provider Contract Spec

A future adapter must use the existing provider contract:

- `Rate(ctx, base, target)` must honor caller cancellation and deadlines.
- `ConvertWithProvider` remains the only conversion integration point.
- The pure `NewExchangeRate` and `Convert` value path must stay network-free.
- Bloomberg-specific source details must be encoded in
  `ExchangeRateQuote.Source` unless a broader structured metadata API is added
  for all providers.

Suggested source metadata shape:

| Example | Meaning |
|---|---|
| `Bloomberg:SAPI:USD Curncy:PX_LAST:BGN` | SAPI request for a Bloomberg currency security and field using a configured pricing source. |
| `Bloomberg:BPIPE:EUR Curncy:BID:realtime` | B-PIPE-derived local snapshot from a real-time entitlement-controlled feed. |
| `Bloomberg:DataLicense:USD Curncy:PX_LAST:batch` | Data License batch or scheduled dataset source. |

The exact security, field, and pricing-source choices must remain
caller-configured because Bloomberg entitlements and data-license terms are
customer-specific.

## Failure Mapping

| Failure | Required mapping |
|---|---|
| Caller cancellation or deadline | Return the original context error so `errors.Is` works. Do not serve stale data for caller-owned context failure. |
| Bloomberg session/authentication/configuration failure | Wrap `ErrExchangeRateProvider` with the underlying cause. |
| Entitlement or permission denial | Wrap `ErrExchangeRateProvider` and include a sanitized entitlement-denied diagnostic. Do not collapse it into nil or an empty rate. |
| Security/field/pair not available | Return `ErrExchangeRateUnavailable` when the requested data is valid but absent. Return `ErrUnsupportedExchangeRate` when the pair or configured Bloomberg mapping is not supported. |
| Stale snapshot beyond caller tolerance | Wrap `ErrExchangeRateStale` and keep the refresh failure visible. |
| Network, protocol, malformed response, or adapter failure | Wrap `ErrExchangeRateProvider` with sanitized bounded diagnostics. |

## Freshness Rules

- `FetchedAt` records the local fetch or snapshot read time.
- `ObservedAt` records Bloomberg's observation, tick, valuation, or batch
  dataset timestamp when available.
- `ExpiresAt` is derived from caller-owned `CacheTTL`.
- `Stale` and `RefreshError` follow the existing ECB/IMF stale fallback shape.
- `MaxStale` and `AllowStaleOnError` remain caller-controlled.
- SAPI request/response, B-PIPE streaming snapshot, and Data License batch
  records must not share one implicit freshness model.

## Test Strategy

- Core `money` tests must not require Bloomberg credentials, Bloomberg SDKs,
  paid products, or network access.
- Adapter behavior should be tested with fakes for session lifecycle,
  entitlements, request/response data, stale cache fallback, malformed payloads,
  field exceptions, cancellation, deadlines, and bounded diagnostics.
- Optional live integration tests, if ever added, must be opt-in behind a build
  tag and environment variables such as `BLOOMBERG_*`; they must stay outside
  default CI.
- README examples must show the adapter as customer-owned infrastructure, not as
  a ready-to-use public data source.

## Security and Operations

- Do not document or commit real Bloomberg credentials, certificates, UUIDs,
  terminal sessions, SAPI appliance details, or entitlement identifiers.
- Treat mutual TLS, customer network topology, Bloomberg product contracts,
  entitlement management, audit logging, and data usage monitoring as deployment
  prerequisites.
- Keep entitlement, usage, stale-data, cache, and provider failures visible to
  callers because these are operational controls, not incidental errors.

## Outcome

#232 is resolved by documenting the Bloomberg access boundary and deferring
implementation. The next implementation issue, if opened, should target a
separate optional licensed adapter with fake-backed contract tests rather than a
default dependency in `money`.
