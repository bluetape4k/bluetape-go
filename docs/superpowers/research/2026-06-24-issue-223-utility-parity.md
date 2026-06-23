# Issue #223 Utility Parity Boundary

Issue: #223
Parent Epic: #221
Date: 2026-06-24

## Decision

Close #223 as a Go-native boundary decision, not as a new public API slice.

The current `core`, `measure`, and `money` packages already cover the small
utility parity that has repeated bluetape-go demand:

- `core`: `Clamp`, prefixed hex validation, explicit range values, wildcard
  matching, XXH64 helpers, `Quarter`, `YearQuarter`, `DatesUntil`, and
  `DatesInclusive`.
- `measure`: typed unit values, parsing, formatting, compound units, storage
  and binary-size units, and affine temperature helpers.
- `money`: ISO currency parsing, decimal-backed amounts, pure exchange-rate
  conversion, provider-backed ECB/IMF conversion, locale currency mapping, and
  documented FastMoney non-goal evidence.

Adding more helpers now would mostly clone Kotlin/JVM convenience APIs without a
current Go caller. The safer closure is to document the candidate inventory,
keep existing package boundaries, and file follow-up research issues for larger
domains.

## Candidate Inventory

| Source module | Candidate shape | Go decision |
|---|---|---|
| `bluetape4k/logging` | SLF4J logger factory helpers, trace extensions, MDC and coroutine MDC propagation | Defer. Go should use `log/slog`, `context.Context`, and explicit instrumentation hooks. No global logger state or framework coupling in #223. Follow-up: #275. |
| `infra/micrometer`, `infra/opentelemetry` | Metrics and tracing integration patterns | Defer to observability research. Any future API must stay hook-based and caller-owned. Follow-up: #275. |
| `utils/javatimes` | temporal intervals, period collections, quarter/month/week/day/hour ranges, calendar visitors, coroutine range flows | Keep only narrow `core` coverage already present: quarter values and date iteration. Broad period/calendar frameworks are a non-goal for this milestone. |
| `utils/math` | descriptives, histograms, moving averages, regression, ranking, precision, equations, integration, interpolation, geometry, special functions | Defer. Standard library and caller-owned calculations are preferred until repeated package demand is proven. Follow-up: #277. |
| `utils/measured` | typed physical quantities and units | Already represented by `measure`; no new parity needed in #223. |
| `utils/money` | Moneta/JSR-354 construction helpers, operators, rounding, conversion, FastMoney helpers | Already represented by `money` with Go-owned wrappers and prior #180 FastMoney evidence. JVM operator/convenience helpers remain non-goals. |
| `utils/geo`, `utils/science` | geocode values, geohash, GeoIP readers, Bing/Google lookup, WGS84/UTM projection, shapefile/NetCDF integration | Defer. Pure values, provider-backed IO, and heavyweight GIS/data dependencies need separate design. Follow-up: #276. |

## Rejected For #223

- `slog` wrapper package with global defaults: this would make logging state
  implicit and conflict with caller-owned logging.
- Broad math/statistics package: the JVM module is large and partly backed by
  Apache Commons Math style APIs; no current bluetape-go caller proves the
  package boundary.
- Java-time period framework port: Go's `time` plus existing `core`
  `YearQuarter` and date iteration cover the repeated needs found in the repo.
- Geo/science implementation: useful but dependency-heavy and too broad for
  closure milestone work.
- Money operator/convenience aliases and FastMoney parity: `money` already
  exposes Go-native constructors and #180 keeps one public `Money` type.

## Follow-ups Filed

- #275 `research: Evaluate slog and observability hook boundaries`
- #276 `research: Evaluate geo and coordinate utility scope`
- #277 `research: Evaluate focused statistics and math utility scope`

## Validation Plan

This is a docs/research closure. Required local gates are repository hygiene
checks plus Step 6-R review:

- `git diff --check`
- `make fmt-check`
- `make tidy-check`
- `make vet`
- `golangci-lint cache clean && make lint`
- `make test`
- `make race`
