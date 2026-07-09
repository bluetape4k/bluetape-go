# Issue #422 OpenTelemetry Bridge Guidance

Issue: [#422](https://github.com/bluetape4k/bluetape-go/issues/422)  
Date: 2026-07-09  
Milestone: Backlog

## Decision

Do not schedule a first-party OpenTelemetry exporter, handler wrapper, global
logger registry, or cross-package observability facade for `bluetape-go`.

The existing direction remains correct:

- library packages expose caller-owned hooks only when they already have stable
  domain events;
- applications own `log/slog` handler configuration, OpenTelemetry providers,
  exporters, sampling, batching, shutdown, and deployment-specific resource
  attributes;
- examples may mention the official OpenTelemetry `otelslog` bridge as an
  application-level handler choice, but core packages must not import
  OpenTelemetry for logging by default.

## Demand Check

| Evidence | Result |
|---|---|
| [`bluetape-go-workshop` #139](https://github.com/bluetape4k/bluetape-go-workshop/issues/139) | Confirms workshop demand for Go-native `slog` logging examples, not an OpenTelemetry-specific bridge. |
| `bluetape-go` #361 | Already turned the `slog` decision into package examples and README guidance. |
| `resilience.OnEvent` | The only current package-local stable event hook that is ready for logging, metrics, or tracing bridge examples. |
| Current repo search | No current package needs a first-party OpenTelemetry adapter or exporter dependency. |

This satisfies #422 by linking the closest workshop use case while explicitly
not scheduling a broader adapter until a concrete runtime package asks for it.

## Official OpenTelemetry Go Notes

- `go.opentelemetry.io/contrib/bridges/otelslog` exposes `NewHandler` and
  `NewLogger` for adapting `log/slog` records to OpenTelemetry logs.
- `otelslog.WithLoggerProvider` lets applications pass an explicit
  `log.LoggerProvider`; otherwise the handler uses the global provider.
- OpenTelemetry provider/exporter setup is an application initialization
  concern. Library packages should not silently mutate global providers or own
  exporter lifecycle.

Sources:

- <https://github.com/open-telemetry/opentelemetry-go-contrib/blob/main/bridges/otelslog/handler.go>
- <https://github.com/open-telemetry/opentelemetry-go/blob/main/CONTRIBUTING.md>

## Guidance For `bluetape-go`

1. Keep package hooks package-local and caller-owned.
2. Keep `log/slog` as the documented structured logging contract.
3. Mention `otelslog` only as an application bridge option in docs that already
   discuss `slog` bridge behavior.
4. Do not add OpenTelemetry dependencies to production packages for generic
   logging guidance.
5. If a future package needs telemetry, first document stable event names,
   low-cardinality labels, context ownership, synchronous/asynchronous hook
   behavior, and failure isolation.

## Outcome

Close #422 after the `resilience` README pair records the optional
application-level `otelslog` bridge boundary. No implementation issue is needed
from this research note.
