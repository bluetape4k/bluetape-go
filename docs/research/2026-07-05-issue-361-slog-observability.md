# Issue #361 slog Observability Decisions

Date: 2026-07-05

Issue #361 turns the accepted Go logging direction into concrete examples and
documentation. The chosen boundary is standard-library `log/slog`, configured
by applications and connected to library packages through caller-owned hooks.

## Decisions

| Area | Decision | Reasoning |
|---|---|---|
| Library logging API | No global bluetape-go logger registry | Libraries should not mutate global slog defaults or own application logging state. |
| Third-party logging | No zap/zerolog/logrus dependency | The standard-library `log/slog` contract is enough for examples and bridges. |
| `resilience.OnEvent` | Add `slog.LogAttrs` bridge example | `OnEvent` already exposes low-cardinality event fields and runs synchronously on the protected call path. |
| Public README snippets | Replace `log.Printf` with structured `slog` | Caller guidance should show stable attributes instead of formatted strings. |
| Expensive debug logging | Document `logger.Enabled(ctx, slog.LevelDebug)` guard | Attribute computation remains caller-owned and should not happen when debug is disabled. |

## Touched Surfaces

- Root `README.md` and `README.ko.md`: logging ownership policy.
- `resilience/README.md` and `resilience/README.ko.md`: `slog` bridge snippet
  and synchronous hook boundary.
- `resilience/resilience_example_test.go`: compile-checked `OnEvent` to
  `slog.LogAttrs` example.
- `money`, `workflow`, and `workreport` README pairs: public snippets now use
  structured `slog` calls.

## Non-Goals

- No logging facade package.
- No MDC-shaped helper.
- No OpenTelemetry exporter or slog handler wrapper.
- No injected logger option on package APIs without a concrete package need.

## Verification Target

- `go test -count=1 ./resilience ./money ./workflow ./workreport`
- `rg -n "log\\.Printf|log\\.Print|log\\.Fatal" README.md README.ko.md **/README.md **/README.ko.md`
- Full local gate before PR merge.
