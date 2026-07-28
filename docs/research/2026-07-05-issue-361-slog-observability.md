# Issue #361 slog Observability Decisions

Date: 2026-07-05

Issue #361은 승인된 Go logging 방향을 concrete example과 documentation으로 바꾼다. 선택한 경계는 standard-library
`log/slog`이며, application이 이를 configure하고 caller-owned hook을 통해 library package와 연결한다.

## Decisions

| Area | Decision | Reasoning |
|---|---|---|
| Library logging API | global bluetape-go logger registry 없음 | library가 global slog default를 mutate하거나 application logging state를 소유하면 안 된다. |
| Third-party logging | zap/zerolog/logrus dependency 없음 | standard-library `log/slog` contract만으로 example과 bridge에 충분하다. |
| `resilience.OnEvent` | `slog.LogAttrs` bridge example 추가 | `OnEvent`는 low-cardinality event field를 이미 노출하고 protected call path에서 synchronously 실행된다. |
| Public README snippets | `log.Printf`를 structured `slog`로 교체 | caller guidance는 formatted string보다 stable attribute를 보여 줘야 한다. |
| Expensive debug logging | `logger.Enabled(ctx, slog.LevelDebug)` guard 문서화 | attribute computation은 caller-owned이며 debug가 disabled이면 실행되지 않아야 한다. |

## Touched Surfaces

- root `README.md` 및 `README.ko.md`: logging ownership policy.
- `resilience/README.md` 및 `resilience/README.ko.md`: `slog` bridge snippet과 synchronous hook boundary.
- `resilience/resilience_example_test.go`: compile-checked `OnEvent` to `slog.LogAttrs` example.
- `money`, `workflow`, `workreport` README pair: public snippet이 structured `slog` call을 사용한다.

## Non-Goals

- logging facade package 없음.
- MDC-shaped helper 없음.
- OpenTelemetry exporter 또는 slog handler wrapper 없음.
- concrete package need 없이 package API에 injected logger option을 추가하지 않음.

## Verification Target

- `go test -count=1 ./resilience ./money ./workflow ./workreport`
- `rg -n "log\\.Printf|log\\.Print|log\\.Fatal" README.md README.ko.md **/README.md **/README.ko.md`
- PR merge 전 full local gate.
