# Observability Graph 예제

[English](README.md) | [한국어](README.ko.md)

이 package는 `bluetape4k-graph/examples/observability-graph-examples`의
incident-response graph를 Go다운 예제로 옮깁니다. Neo4j나 Memgraph adapter가
아직 없어도 실행할 수 있도록 `graph` value와 `graph/graphio` record만 사용합니다.

![Observability incident graph topology](../../../docs/images/readme-diagrams/graph-observability-incident-topology.png)

## 증명하는 것

Seed fixture는 checkout 장애 상황을 모델링합니다.

- edge API에 의존하는 public API vertex 2개,
- edge API에서 checkout, payment, Postgres로 이어지는 service dependency,
- 영향받은 service에 연결된 latency/error alert,
- `payment-service`를 가리키는 incident root-cause edge,
- `payments-team`으로 이어지는 ownership edge.

Package는 graph가 실제로 쓸모 있는지 보여주는 caller 질문에 답합니다.

| 질문 | Go API |
|---|---|
| 장애 service에 의존하는 service는 무엇인가? | `UpstreamImpactedServices("payment-service", 3)` |
| 영향받는 public API는 무엇인가? | `AffectedAPIs("payment-service", 5)` |
| service가 의존하는 downstream은 무엇인가? | `DownstreamDependencies("checkout-service", 2)` |
| alert boundary에 들어가는 service는 무엇인가? | `AlertBoundary([]string{...}, 2)` |
| incident boundary를 소유한 team은 어디인가? | `OwningTeams("payment-service")` |

## Seed Data

Seed data는 `SeedIncidentGraph`에 있습니다. Source CSV fixture와 같은 10개
vertex, 10개 edge를 사용합니다. Query 결과는 `payment-service`,
`checkout-api`, `payments-team` 같은 domain ID를 반환하고, graph element ID는
`svc-payment` 같은 안정적인 transport ID로 유지합니다.

같은 graph는 `WriteNDJSON`과 `ReadIncidentGraphNDJSON`을 통해 `graph/graphio`
NDJSON으로 export/import할 수 있습니다.

## Test

```bash
go test -count=1 ./examples/graph/observability
go test -race -count=1 ./examples/graph/observability
```

## Production Omission

이 예제는 backend session, persistence, Cypher/Gremlin, online schema migration,
alert ingestion, incident lifecycle state machine, authorization, metrics,
traversal 성능 주장을 의도적으로 제외합니다. 이런 범위는 첫 Neo4j proof가 들어간
뒤 adapter-backed follow-up에서 다룹니다.

`0.10.0`에서는 이 observability 예제 하나만 port합니다. Code dependency, fraud,
knowledge, social, recommendation, Ktor 예제는 backend traversal contract, 더 큰
domain model, 또는 Go로 그대로 옮기면 안 되는 JVM/Ktor shape가 필요하므로
defer합니다. 다음으로 가치가 큰 IAM/access graph 예제는
[#368](https://github.com/bluetape4k/bluetape-go/issues/368)에서 추적합니다.
