# Issue #275 교훈

- 기본 Go logging 계약은 `log/slog`로 둔다. library package는 default를 변경하거나
  global logger state를 소유하거나 JVM MDC helper 형태를 복제하지 않는다.
- Observability는 먼저 event source를 소유한 package에 속한다. 그 package에서
  stable event와 label을 정의한 뒤, 호출자가 logging, metrics, tracing으로
  bridge하게 한다.
- Generic telemetry facade는 package-specific 가치가 증명되기 전에 dependency와
  lifecycle 소유권을 만든다. 구체적인 adapter issue가 필요성을 증명하기 전까지
  exporter dependency는 core package 밖에 둔다.
