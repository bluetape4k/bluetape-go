# Issue #275 Observability Scope

Issue: #275
Parent: #7
Date: 2026-06-26

## 결정

#275는 research-only 경계로 닫는다. 0.7.0 research gate에서는 새
`logging`, `observability`, `metrics`, OpenTelemetry 패키지를 추가하지
않는다.

현재 Go-native 방향은 다음과 같다.

- 애플리케이션 소유자가 `log/slog`를 직접 설정한다.
- bluetape-go 패키지는 `resilience.EventHandler`처럼 이미 도메인 이벤트를
  가진 위치에서만 caller-owned hook을 받는다.
- 패키지 문서는 해당 hook을 logging, metrics, tracing에 연결하는 예시는
  보여줄 수 있지만, core 패키지가 exporter나 global logging state를
  소유하면 안 된다.
- OpenTelemetry 통합은 구체적인 runtime 패키지가 dedicated adapter를
  필요로 할 때까지 애플리케이션 관심사로 남긴다.

이 이슈에서 구현 follow-up은 만들지 않는다. 다음 구현 milestone은
observability 요구를 cross-cutting facade가 아니라 소유 패키지 설계
안에 유지해야 한다.

## 소스 인벤토리

| Source | Capability | Go decision |
|---|---|---|
| `bluetape4k/logging` | SLF4J logger factories, trace helpers, MDC, coroutine MDC propagation | Defer. JVM framework convenience에 가깝다. Go caller는 `context.Context`를 전달하고 `log/slog`나 자체 logger를 직접 설정해야 한다. |
| `infra/micrometer` / `infra/opentelemetry` | Metrics and tracing integration patterns | Generic package로는 보류한다. 구체 패키지가 안정적인 low-cardinality event를 갖기 전까지 telemetry adapter는 core package 밖에 둔다. |
| `bluetape4k-leader` observability | Framework-neutral leader events plus Micrometer/Spring adapters | Module이 아니라 pattern만 채택한다. 먼저 package-local event를 정의하고, adapter는 소유 패키지나 example에 둔다. |
| `bluetape-go/resilience` | Synchronous `OnEvent` hooks with stable event categories | 유지한다. 현재 first-party package observability에 맞는 모델이다. |

## 외부 근거

- Go의 `log/slog`는 standard library structured logging이며 `Logger`,
  `Handler`, `TextHandler`, `JSONHandler`, levels, groups, `LogAttrs`,
  context-aware methods를 제공한다.
- `slog` handler는 의도된 확장 지점이고 concurrent call을 스스로 관리해야
  한다. 따라서 구체 adapter 요구가 없으면 shared bluetape-go handler
  facade는 concurrency와 ownership 부담이 된다.
- 공식 Go blog는 `slog`를 애플리케이션의 competing logging dependency를
  줄이기 위한 shared structured logging framework로 설명한다.
- OpenTelemetry는 이미 `slog.Record` 값을 OpenTelemetry log record로
  변환하는 `otelslog` bridge를 제공하므로, bluetape-go는
  package-specific value가 생기기 전 같은 bridge를 감싸지 않는다.

## 기각

- bluetape-go package 안의 global logger registry 또는 default logger
  mutation.
- MDC-shaped context helper. ownership을 숨기고 implicit logging behavior를
  유도한다.
- core package의 first-party OpenTelemetry exporter 또는 handler dependency.
- package-local event contract 없는 generic metrics package.
- hook 내부 default logging. Hook은 caller-owned이고 예측 가능해야 한다.

## 향후 패키지 필수 패턴

Observability가 필요한 향후 package design은 다음을 문서화해야 한다.

1. 안정적인 event name과 low-cardinality label.
2. context propagation과 cancellation ownership.
3. hook이 protected call path에서 synchronous로 실행되는지 여부.
4. package가 async loop나 background worker를 소유한다면 panic 또는 slow
   hook에 대한 failure isolation.
5. runtime dependency를 추가하지 않고 event를 `log/slog`로 bridge하는 예시.

## 검증

- 현재 repository search에서 안정적인 event hook surface를 소유한 패키지는
  `resilience`뿐이다.
- `docs/superpowers/research/2026-06-24-issue-223-utility-parity.md`는 이미
  global `slog` wrapper를 기각하고 observability scope를 이 이슈로 라우팅했다.
- 현재 패키지에는 package-local hook 이상의 observability facade가 필요하지
  않으므로, 이 이슈는 follow-up implementation 없이 닫는다.
