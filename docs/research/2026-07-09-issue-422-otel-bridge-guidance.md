# Issue #422 OpenTelemetry Bridge Guidance

Issue: [#422](https://github.com/bluetape4k/bluetape-go/issues/422)  
Date: 2026-07-09  
Milestone: Backlog

## 결정

`bluetape-go`에는 first-party OpenTelemetry exporter, handler wrapper, global logger registry, cross-package observability facade를
schedule하지 않는다.

기존 방향을 유지한다.

- library package는 stable domain event가 이미 있을 때만 caller-owned hook을 노출한다.
- application이 `log/slog` handler configuration, OpenTelemetry provider, exporter, sampling, batching, shutdown,
  deployment-specific resource attribute를 소유한다.
- example은 official OpenTelemetry `otelslog` bridge를 application-level handler 선택지로 언급할 수 있지만, core package는
  logging 때문에 OpenTelemetry를 기본 import하지 않는다.

## Demand Check

| Evidence | Result |
|---|---|
| [`bluetape-go-workshop` #139](https://github.com/bluetape4k/bluetape-go-workshop/issues/139) | workshop demand는 OpenTelemetry-specific bridge가 아니라 Go-native `slog` logging example이다. |
| `bluetape-go` #361 | 이미 `slog` decision을 package example과 README guidance로 바꿨다. |
| `resilience.OnEvent` | 현재 package-local stable event hook 중 logging, metrics, tracing bridge example로 준비된 유일한 surface다. |
| Current repo search | 현재 first-party OpenTelemetry adapter 또는 exporter dependency가 필요한 package가 없다. |

이로써 #422는 가장 가까운 workshop use case를 link하면서 concrete runtime package가 요구하기 전까지 broad adapter를 schedule하지
않는다고 명확히 한다.

## Official OpenTelemetry Go Notes

- `go.opentelemetry.io/contrib/bridges/otelslog`는 `log/slog` record를 OpenTelemetry log로 adapt하는 `NewHandler`와
  `NewLogger`를 노출한다.
- `otelslog.WithLoggerProvider`는 application이 명시적 `log.LoggerProvider`를 넘길 수 있게 한다. 그렇지 않으면 handler는 global
  provider를 쓴다.
- OpenTelemetry provider/exporter setup은 application initialization concern이다. library package가 global provider를 조용히
  mutate하거나 exporter lifecycle을 소유하면 안 된다.

Sources:

- <https://github.com/open-telemetry/opentelemetry-go-contrib/blob/main/bridges/otelslog/handler.go>
- <https://github.com/open-telemetry/opentelemetry-go/blob/main/CONTRIBUTING.md>

## Guidance For `bluetape-go`

1. package hook은 package-local 및 caller-owned로 유지한다.
2. documented structured logging contract는 `log/slog`로 유지한다.
3. `slog` bridge behavior를 이미 다루는 docs에서만 `otelslog`를 application bridge option으로 언급한다.
4. generic logging guidance 때문에 production package에 OpenTelemetry dependency를 추가하지 않는다.
5. future package가 telemetry를 필요로 하면 먼저 stable event name, low-cardinality label, context ownership,
   synchronous/asynchronous hook behavior, failure isolation을 문서화한다.

## Outcome

`resilience` README pair가 optional application-level `otelslog` bridge boundary를 기록한 뒤 #422를 닫는다. 이 research note에서
새 implementation issue는 필요하지 않다.
