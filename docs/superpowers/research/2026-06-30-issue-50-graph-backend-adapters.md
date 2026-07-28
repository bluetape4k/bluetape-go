# Issue #50 Graph Backend Adapter Feasibility

> 한국어 연구 요약: 이 문서는 사용자 협업용 조사/결정 기록이다. 아래 표와 목록의 URL, package name, command, issue number, version, source path는 evidence이므로 그대로 보존한다. 의사결정, 선택/보류/거절 사유, 후속 이슈 경계는 한국어 독자가 바로 이해할 수 있도록 이 요약을 우선 적용한다.
> 추가 한국어 해석: 이 문서에서 영어로 남은 표의 값은 원문 근거이며, 실제 채택 여부는 한국어 결정 문장을 따른다. 후속 작업자는 보류와 거절 항목을 새 구현 범위로 착각하지 않아야 한다.\n

Date: 2026-06-30
Milestone: 0.10.0
Issue: #50

## 결정

Rank the Go graph backend path as:

1. Neo4j adapter proof through the official Go driver.
2. Memgraph compatibility matrix that reuses the Neo4j-driver path.
3. Apache AGE deferred until its Go driver and local-test story lose the source
   install and AGType ergonomics risks.
4. FalkorDB deferred until Redis-module container testing and query semantics are
   proven in Go.
5. TinkerPop/TinkerGraph rejected as a local Go adapter target; the Go GLV is a
   remote Gremlin client, not an in-process TinkerGraph equivalent.
6. Amazon Neptune remains research-only because it depends on managed-service
   or remote Gremlin/SDK access rather than a cheap local backend proof.

Created implementation follow-ups only for selected paths:

- #365: Implement Neo4j graph adapter proof.
- #366: Prove Memgraph compatibility for the Neo4j graph adapter.

## Source-Parity Baseline

`bluetape4k-graph` provides backend modules for Neo4j, Memgraph, Apache AGE,
TinkerPop/TinkerGraph, and FalkorDB, plus future Neptune tracking. The JVM
project also has a broad `graph-core` repository/session/schema/query contract.

`bluetape-go` should not port that contract yet. Issues #48 and #49 deliberately
kept the Go surface to backend-neutral values plus bounded NDJSON/CSV record
I/O. Backend work now needs one concrete driver proof before any common
repository or session abstraction is credible.

## 근거 Matrix

| Backend | Go driver maturity | License / maintenance | Query model | Transactions / batch / schema | Local test story | Decision |
|---|---|---|---|---|---|---|
| Neo4j | Official `neo4j/neo4j-go-driver`, latest release `v6.1.0` on 2026-05-15, repo active on 2026-06-28 | Apache-2.0, vendor-owned | Cypher over Bolt | Driver exposes context-aware query APIs; schema/batch details can be proven against real Neo4j | Testcontainers for Go has a `neo4j` module with Bolt URL helper | Select first |
| Memgraph | Official docs direct Go users to the Neo4j Golang driver | Memgraph server active; driver path is Neo4j driver | Cypher over Bolt-compatible protocol | Must prove compatibility differences instead of assuming Neo4j parity | No dedicated Testcontainers Go module found; generic container likely needed | Select as compatibility matrix |
| Apache AGE | Apache repo includes `drivers/golang` but README requires source install, Java, ANTLR, DSN setup, and AGE-loaded PostgreSQL | Apache-2.0; server active | Cypher through PostgreSQL/AGE SQL functions and AGType | AGType/query ergonomics need proof before API design | Could build on Postgres Testcontainers, but AGE image/setup must be proven separately | Defer |
| TinkerPop / TinkerGraph | Apache Gremlin-Go exists and is active | Apache-2.0 | Gremlin remote traversal | It connects to a Gremlin Server or remote provider; not an embedded TinkerGraph replacement | Requires remote Gremlin service, not cheap in-process Go local tests | Reject local adapter; keep remote Gremlin/Neptune research boundary |
| FalkorDB | Official `FalkorDB/falkordb-go/v2`, latest `v2.1.0` on 2026-01-15, repo active but small | BSD-3-Clause; smaller Go client footprint | openCypher subset over Redis module | Query API exists, but result and timeout semantics need proof | README tests expect server at `localhost:6379`; no dedicated Testcontainers Go module found | Defer |
| Neptune | AWS Go SDK modules exist for Neptune Data and Neptune Graph; AWS docs cover Go Gremlin access | Managed AWS service | Gremlin/openCypher/RDF depending service | Not a local backend contract; authentication and service setup dominate | No local Testcontainers-equivalent proof | Research-only |

## Selected Follow-Ups

### #365 Neo4j Adapter Proof

Use the official Neo4j Go driver and the Testcontainers Neo4j module to prove:

- `neo4j.Node` and `neo4j.Relationship` to `graph.Vertex` / `graph.Edge`
  adaptation.
- Context cancellation and driver close behavior.
- Basic create/read/query failure mapping.
- README boundaries that avoid a premature repository/session/schema DSL.

### #366 Memgraph Compatibility

Treat Memgraph as a Neo4j-driver compatibility matrix:

- Reuse the selected Neo4j adapter surface.
- Add a generic Memgraph container only if no dedicated Testcontainers Go module
  exists when the issue starts.
- Record Cypher/result behavior differences as guardrails or follow-up issues.

## Rejected Or Deferred Paths

- Do not create a separate Memgraph abstraction before compatibility evidence.
- Do not create an AGE adapter until `drivers/golang` source-install, AGType, and
  local AGE container setup are proven by a small spike.
- Do not create FalkorDB work until Redis-module container setup and result
  semantics are proven in Go.
- Do not port JVM TinkerGraph. Gremlin-Go is a remote GLV and should only be
  revisited for Neptune/remote-Gremlin examples.
- Do not start Neptune until a managed-service integration policy and cost/test
  boundary exists.

## Source Evidence

- `bluetape4k-graph/README.md`: supported backend matrix and JVM module shape.
- `docs/lessons/2026-06-25-graph-research-order.md`: first-pass graph backend
  ordering from #38.
- `docs/lessons/2026-06-29-graph-model-api-boundaries.md`: #48 model-only
  boundary.
- `docs/lessons/2026-06-30-graph-io-boundaries.md`: #49 record-stream boundary.
- `neo4j/neo4j-go-driver`: official Go driver, `v6.1.0`, Apache-2.0.
- `testcontainers/testcontainers-go`: `modules/neo4j` exposes `Run` and
  `BoltUrl`.
- Memgraph Go client docs: Memgraph currently depends on the Neo4j Golang
  driver because both support Bolt and Cypher.
- `apache/age/drivers/golang/README.md`: AGType parser/driver support, source
  install, Java/ANTLR and DSN prerequisites.
- `apache/tinkerpop/gremlin-go/README.md`: Go GLV connects to Gremlin Server or
  remote providers.
- `FalkorDB/falkordb-go`: official Go client; tests expect a FalkorDB server at
  localhost.
- AWS Neptune docs and Go modules: Go access exists, but the proof is remote or
  managed-service oriented.

## 검증

- `gh issue view 50 --json ...`: issue scope and acceptance criteria confirmed.
- `gh issue list --search "Neo4j graph adapter ..."` and Memgraph equivalent:
  no duplicate implementation issues before creating #365 and #366.
- `gh repo view` for Neo4j, Testcontainers Go, Memgraph, Apache AGE, TinkerPop,
  and FalkorDB upstream metadata.
- `go list -m -versions` for Neo4j, Testcontainers Go, FalkorDB, AWS Neptune
  modules, and Apache/TinkerPop module availability.
