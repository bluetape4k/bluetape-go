# Issue #223 교훈

- utility parity는 JVM module breadth가 아니라 반복되는 Go caller에서 시작해야 한다.
  기존 `core`, `measure`, `money` package는 이 milestone에서 찾은 좁은 repeated
  helper demand를 이미 덮는다.
- logging parity는 특히 over-port하기 쉽다. Go는 global logger state, MDC-shaped API,
  framework coupling을 추가하기보다 `log/slog`, `context.Context`, explicit hook과
  호환되어야 한다.
- broad math, geo, science helper는 pure value, dependency-heavy algorithm,
  provider-backed IO, domain-specific edge case를 섞으므로 별도 research issue가
  필요하다.
- durable outcome이 documented boundary와 oversized domain follow-up issue라면, 새
  code 없이 inventory issue를 닫는 것도 허용된다.
