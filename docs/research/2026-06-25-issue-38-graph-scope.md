# Issue 38 Graph 범위 연구

## 맥락

Issue #38은 milestone 0.11.0 구현을 시작하기 전에 `bluetape4k-graph` ecosystem을
Go package로 만들지, example로 남길지, defer할지를 결정하는 0.7.0 research gate다.

이 노트는 #44, #48, #49, #50, #51에 대한 source-backed scope decision으로
넓은 June 1 graph placeholder를 대체한다.

## 소스 인벤토리

현재 `bluetape4k-graph` 증거:

- Root README describes a unified Kotlin API over Apache AGE, Neo4j, Memgraph,
  TinkerPop/TinkerGraph, and FalkorDB, plus graph I/O, Spring Boot/Ktor
  integrations, examples, benchmarks, and a BOM.
- `graph/graph-core` owns vertices, edges, paths, element IDs, labels,
  properties, repository contracts, traversal APIs, schema DSL, batch writes,
  optional schema/merge/transaction capabilities, weighted path hooks, and graph
  algorithm contracts.
- `graph-io` owns shared import/export reports and failure models, CSV paired
  files, NDJSON envelopes, GraphML XML/StAX subset, and OkIO streaming with
  compression/encryption adapters.
- Backend modules are `graph-neo4j`, `graph-memgraph`, `graph-age`,
  `graph-tinkerpop`, and `graph-falkordb`.
- Example modules cover code dependency, fraud detection, IAM access,
  knowledge graph, LinkedIn/social graph, observability incidents,
  recommendations, supply-chain impact, data lineage, network topology,
  security attack path, and Ktor integration.
- Benchmark docs contain already useful Go-facing signals: Neo4j is the safest
  production default, Memgraph is the fastest persistent backend in local
  latency/write rows, AGE timed out or underperformed in larger graph adoption
  runs, FalkorDB is lightweight but weak for edge-heavy repeated writes, and
  TinkerGraph remains valuable only as an in-memory contract/example baseline.

## 현재 Go Ecosystem 증거

External source는 2026-06-25에 확인했다.

- Neo4j는 `v6`용 official Go driver와 current manual을 제공한다
  (`github.com/neo4j/neo4j-go-driver/v6`).
- Memgraph의 Go quick start는 Neo4j Go driver를 명시적으로 사용한다. 두 제품 모두
  Bolt와 Cypher를 사용하기 때문이다. Memgraph는 별도 first-party Go driver surface가
  아니라 compatibility를 adoption path로 둔다.
- Apache AGE repository에는 Go driver/parser가 있지만 surface가 Neo4j driver보다
  작고 PostgreSQL AGE AGType/Cypher-over-SQL semantic에 묶여 있다.
- FalkorDB에는 `github.com/FalkorDB/falkordb-go/v2`와 Go client/OGM support를
  언급하는 FalkorDB docs가 있다.
- Testcontainers for Go는 Neo4j와 PostgreSQL official module을 제공한다. 현재
  증거로는 Memgraph, Apache AGE, FalkorDB용 first-party Testcontainers Go module이
  보이지 않으므로, 구현 전 repo-local `GenericContainer` launcher 또는 custom module이
  필요하다.

## 순위

| Area | Go fit | Maintenance cost | Recommendation |
| --- | --- | --- | --- |
| Graph domain examples | High | Low/medium | Implement first as examples using a selected backend and in-memory fixtures. |
| Graph I/O NDJSON/CSV | High | Medium | Implement as backend-neutral helpers after core models settle. |
| Minimal graph models | Medium/high | Medium | Implement a small model package only if I/O/examples need it. |
| Neo4j adapter/examples | High | Medium | Adopt official driver directly; build thin examples/adapters only around repeated bluetape-go contracts. |
| Memgraph examples | Medium/high | Medium | Use Neo4j driver compatibility; keep as example/test matrix, not a distinct API first. |
| GraphML | Medium | Medium/high | Defer until CSV/NDJSON contracts prove value; XML edge cases raise maintenance cost. |
| Backend-independent repository API | Medium | High | Narrow heavily; avoid a lowest-common-denominator query abstraction. Use optional interfaces only after two adapters prove the same contract. |
| Apache AGE adapter | Low/medium | High | Defer; PostgreSQL-native recursive CTE or SQL examples may be better for Go services. |
| FalkorDB adapter | Low/medium | Medium/high | Defer implementation; keep as research/example-only until edge-heavy behavior and local test launcher are acceptable. |
| TinkerPop/TinkerGraph Go port | Low | High | Do not port directly; use pure in-memory test fixtures instead. |
| Spring/Ktor integrations | N/A for Go | High | Do not port; replace with workshop HTTP examples when needed. |
| Benchmarks | Medium | Medium | Add only after implementation candidates exist; use Go benchmarks and measured command output, not copied JMH claims. |

## 결정

### 구현

- #49 또는 #51이 요구할 때만 작은 `graph` model surface를 둔다.
  `Vertex`, `Edge`, `Path`, element IDs, labels, properties, and typed import
  errors가 대상이다. 첫 PR에서는 data-oriented로 유지하고 repository/session
  abstraction을 피한다.
- NDJSON graph envelope helper를 먼저 둔다. NDJSON은 streaming-friendly,
  line-oriented, `encoding/json`으로 테스트하기 쉽고 XML이나 paired-file
  coordination이 필요 없어 Go 적합성이 가장 높다.
- NDJSON이 stable model을 드러내고 명확한 bulk import/export example이 생긴 뒤에
  CSV paired-file helper를 두 번째로 검토한다.
- 첫 domain example은 observability incident 또는 IAM access graph로 둔다. 둘 다
  Go service concern에 매핑되고 broad graph abstraction 없이 테스트할 수 있다.

### 채택

- real graph database example과 향후 adapter experiment에는 Neo4j official Go driver를
  사용한다.
- Memgraph compatibility experiment에는 Neo4j Go driver를 사용한다. 별도 interface를
  만들지 말고 Memgraph compatibility quirk를 따로 기록한다.
- Neo4j integration test에는 Testcontainers Go Neo4j module을 사용한다.
- PostgreSQL/AGE exploratory test에는 Testcontainers Go PostgreSQL module만 사용한다.
  dedicated local launcher가 reliability를 증명하기 전에는 AGE를 selected로 취급하지 않는다.

### Example-only

- Memgraph는 별도 public package가 아니라 Neo4j-driver compatibility example 또는
  test matrix row로 시작한다.
- 두 backend가 동일한 Go-shaped contract를 증명하기 전까지 backend comparison은
  docs/workshop example에 둔다.

### Defer

- Broad backend-independent repository/session abstractions.
- Schema/index DSL, merge/upsert, transaction DSL, and algorithm interfaces in
  base packages.
- AGE adapter, FalkorDB adapter, TinkerPop/TinkerGraph equivalent, Neptune, and
  GraphML until the first Go graph examples show user value.
- Spring Boot/Ktor integration analogues; use ordinary Go HTTP examples if a
  service integration is needed.

## 필요한 Issue 업데이트

- #44: Make 0.11.0 graph epic implementation order explicit: examples and
  NDJSON/CSV before adapters, adapter work only after #50 proves local test
  maturity, and no Spring/Ktor parity port.
- #48: Narrow base abstraction to models plus optional capability follow-ups.
  Defer repository/session interfaces unless two selected adapters prove a
  shared contract.
- #49: Start with NDJSON, then CSV. Defer GraphML and compression/encryption
  chaining until the core import/export reports and streaming ownership policy
  are stable.
- #50: Rank backend feasibility as Neo4j first, Memgraph compatibility second,
  AGE/FalkorDB deferred, TinkerGraph not ported, Neptune research-only.
- #51: Select observability incident or IAM access graph as the first Go domain
  example; defer the large source example matrix until backend/I/O decisions are
  proven.

## 향후 구현 검증 계획

- Public Go API는 success, malformed input, cancellation, zero-value test가 필요하다.
- Streaming import/export는 `context.Context`를 전파하고 ownership flag에 따라 모든
  `io.Reader`/`io.Writer` resource를 닫으며 large-record test를 포함해야 한다.
- Adapter 또는 graph server example은 Testcontainers-backed package를 serial로 실행하고
  local cleanup evidence를 포함해야 한다.
- Shared reader/writer, batched importer, cache, goroutine-owned adapter에는
  concurrency/race evidence가 필요하다.

## 후속 권고

#48을 full Kotlin repository contract로 시작하지 않는다. #49/#51은 작은 shared model과
NDJSON/example proof로 시작한다. 그 proof에서 반복 backend operation이 드러나면,
그 operation만 대상으로 하는 더 좁은 #48 implementation issue를 만든다.
