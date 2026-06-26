# Issue #275 Observability Scope

Issue: #275
Parent: #7
Date: 2026-06-26

## Decision

Close #275 as a research-only boundary. Do not add a new `logging`,
`observability`, `metrics`, or OpenTelemetry package for the 0.7.0 research
gate.

The current Go-native direction is:

- application owners configure `log/slog` directly;
- bluetape-go packages accept caller-owned hooks where they already have
  domain events, such as `resilience.EventHandler`;
- package docs may show how to bridge those hooks to logging, metrics, or
  tracing, but core packages must not own exporters or global logging state;
- OpenTelemetry integration remains an application concern until a concrete
  runtime package needs a dedicated adapter.

No implementation follow-up is filed from this issue. The next implementation
milestones should keep observability requirements inside the owning package's
design, not in a cross-cutting facade.

## Source Inventory

| Source | Capability | Go decision |
|---|---|---|
| `bluetape4k/logging` | SLF4J logger factories, trace helpers, MDC, coroutine MDC propagation | Defer. These are JVM framework conveniences. Go callers should pass `context.Context` and configure `log/slog` or their own logger directly. |
| `infra/micrometer` / `infra/opentelemetry` | Metrics and tracing integration patterns | Defer as a generic package. Keep telemetry adapters outside core packages unless a concrete package has stable low-cardinality events. |
| `bluetape4k-leader` observability | Framework-neutral leader events plus Micrometer/Spring adapters | Adopt the pattern, not the modules: first define package-local events, then let adapters live with the owning package or examples. |
| `bluetape-go/resilience` | Synchronous `OnEvent` hooks with stable event categories | Keep. This is the correct current model for first-party package observability. |

## External Evidence

- Go's `log/slog` is standard library structured logging with `Logger`,
  `Handler`, `TextHandler`, `JSONHandler`, levels, groups, `LogAttrs`, and
  context-aware methods.
- `slog` handlers are the intended extension point and must manage concurrent
  calls themselves, which makes a shared bluetape-go handler facade a
  concurrency and ownership liability unless a concrete adapter is required.
- The official Go blog frames `slog` as the shared structured logging framework
  meant to reduce competing logging dependencies in applications.
- OpenTelemetry already provides an `otelslog` bridge that converts
  `slog.Record` values to OpenTelemetry log records, so bluetape-go should not
  wrap the same bridge before it has package-specific value.

## Rejected

- Global logger registry or default logger mutation in bluetape-go packages.
- MDC-shaped context helpers. They would hide ownership and encourage implicit
  logging behavior.
- First-party OpenTelemetry exporter or handler dependency in a core package.
- Generic metrics package without a package-local event contract.
- Logging inside hooks by default. Hooks must be caller-owned and predictable.

## Required Pattern For Future Packages

Future package designs that need observability must document:

1. stable event names and low-cardinality labels;
2. context propagation and cancellation ownership;
3. whether the hook runs synchronously on the protected call path;
4. failure isolation for panicking or slow hooks, if the package owns async
   loops or background workers;
5. examples that bridge events to `log/slog` without adding runtime
   dependencies.

## Validation

- Current repository search shows only `resilience` owns a stable event hook
  surface today.
- `docs/superpowers/research/2026-06-24-issue-223-utility-parity.md` already
  rejected global `slog` wrappers and routed observability scope to this issue.
- This issue closes without follow-up implementation because no current package
  needs an observability facade beyond package-local hooks.
