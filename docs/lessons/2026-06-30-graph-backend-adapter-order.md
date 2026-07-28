# Graph backend adapter 순서

일자: 2026-06-30
이슈: #50
마일스톤: 0.10.0

## 교훈

Backend adapter 작업은 common repository/session abstraction이 아니라 driver로 검증된
하나의 package에서 시작해야 한다. Neo4j는 official Go driver, active release,
context-aware API shape, 전용 Testcontainers Go module을 모두 갖춘 유일한 평가 backend다.
Memgraph는 새 abstraction이 아니라 이 Neo4j-driver 경로의 compatibility coverage로 다음에
둔다.

AGE, FalkorDB, TinkerPop/TinkerGraph, Neptune은 local-test, driver, managed-service
형태 때문에 현재 graph API가 정당화할 수 있는 것보다 많은 infrastructure를 요구하므로
첫 Go adapter 범위에서는 보류하거나 제외한다.

## 적용된 후속 작업

- #365가 첫 Neo4j adapter proof를 소유한다.
- #366이 Neo4j-driver 경로에 대한 Memgraph compatibility를 소유한다.
- #51 example은 먼저 `graph`와 `graph/graphio`를 사용한다. Backend example은 #365가
  검증된 package boundary를 가진 뒤 진행한다.

## 가드레일

최소 하나의 backend adapter와 하나의 domain example이 Go에서 shared behavior를 증명하기
전까지 backend-independent graph repository, session, schema, transaction, query
interface를 추가하지 않는다.
