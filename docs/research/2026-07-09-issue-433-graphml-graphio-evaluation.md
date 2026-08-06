# Issue #433 GraphML Import/Export Evaluation

Issue: [#433](https://github.com/bluetape4k/bluetape-go/issues/433)  
Milestone: Backlog  
Date: 2026-07-09  
Decision: **`graph/graphio`의 GraphML implementation을 defer한다**

## 결정

지금 `graph/graphio`에 GraphML import/export를 추가하지 않는다. NDJSON과 paired CSV를 supported record-stream format으로
유지하고, XML GraphML compatibility가 올바른 interchange boundary라는 점을 Go caller 또는 workshop example이 증명한 뒤 다시
검토한다.

GraphML은 valid future candidate지만, 현재 core `graphio` record boundary의 조용한 확장이 아니라 별도 optional slice 또는
subpackage여야 한다.

## Current Repo Evidence

| Evidence | Result |
|---|---|
| `graph/graphio/doc.go` | GraphML, compression, encryption, path ownership, atomic replacement, backend integration은 의도적으로 deferred다. |
| `graph/graphio/README.md` / `.ko.md` | unsupported-capability table은 GraphML이 NDJSON/CSV adoption evidence를 따른다고 말한다. |
| `graph/README.md` / `.ko.md` | capability matrix는 GraphML을 deferred follow-up으로 유지한다. |
| `docs/research/2026-06-25-issue-38-graph-scope.md` | 이전 graph scope는 GraphML을 medium fit이지만 medium/high maintenance cost로 평가했다. |
| `docs/lessons/2026-06-30-graph-io-boundaries.md` | accepted boundary는 filesystem, backend, XML compatibility ownership이 아니라 bounded record stream이다. |
| `bluetape-go-workshop` issue #52 | downstream Go workshop request는 CSV, NDJSON, GraphML-style fixture를 허용하지만 GraphML을 특정해 요구하지 않는다. |

## External Compatibility Evidence

| Source | Useful signal for `graphio` |
|---|---|
| NetworkX GraphML docs | NetworkX는 GraphML read/write를 지원하지만 mixed directed/undirected graph, hypergraph, nested graph, port는 unsupported라고 명시한다. reader는 Python XML parsing을 trusted file로 제한하라고 경고한다. |
| NetworkX `read_graphml` / `write_graphml` docs | edge ID, multigraph behavior, yEd extension, default, compression, numeric type inference가 simple node/edge 이상의 compatibility expectation에 영향을 준다. |
| Gephi GraphML docs | Gephi는 limited GraphML subset만 지원하고 subgraph 및 hyperedge를 제외한다. `boolean`, integer, floating, string attribute type과 default를 지원한다. |
| Neo4j APOC GraphML export/import docs | APOC는 interoperability용으로 GraphML을 쓰지만 mixed property value type과 unsupported value type은 string이 되는 등 property-graph fidelity 일부를 잃는다. import도 label, relationship-type, node-ID, batch-size configuration을 노출한다. |
| yWorks/yFiles GraphML docs | yFiles는 standard data/key mechanism을 arbitrary complex data로 확장한다. 따라서 yEd visual GraphML compatibility는 structural GraphML subset만의 문제가 아니다. |
| `bluetape4k-graph` PR #272 / issue #235 | Kotlin line도 GraphML compatibility를 주장하기 전에 explicit fixture, skip/fail behavior, unsupported-construct documentation이 필요했다. |
| `bluetape4k-graph` PR #349 | typed GraphML value에는 dedicated error-reporting fix가 필요했다. GraphML이 low-cost parser addition이 아님을 강화한다. |

## Classification

| Question | Answer |
|---|---|
| 지금 구현? | No. 현재 GraphML을 NDJSON/CSV보다 필요로 하는 bluetape-go caller가 없다. |
| Defer or reject? | Defer. GraphML은 ecosystem value가 있지만 constrained compatibility slice로만 적합하다. |
| 되살릴 때 placement | `graph/graphio/graphml` 또는 유사 optional package를 선호한다. XML-specific option, fixture, limitation이 core NDJSON/CSV API를 확장하지 않게 한다. |
| 첫 supported subset | directed property graph subset: graph, node, edge, key/data attribute, scalar value, edge ID, duplicate ID check, missing endpoint check, explicit input limit. |
| 첫 slice explicit non-goals | nested graph, 한 document 안의 mixed directed/undirected graph, hyperedge, port, yFiles visual styling, arbitrary XML extension payload, path ownership, compression/encryption wrapper, graph database import/export semantics. |

## Risk Matrix

| Risk | Severity | Reason |
|---|---|---|
| XML parser safety 및 untrusted input limit | High | GraphML은 XML이므로 unsafe construct를 거부하고 input을 bound하며 caller-owned reader/deadline behavior를 보존해야 한다. |
| compatibility overclaim | High | common producer가 서로 다른 subset을 지원한다. simple file 하나를 accept한다고 yEd, Gephi, NetworkX, APOC compatibility가 증명되지 않는다. |
| type conversion drift | Medium/high | GraphML scalar declaration, default, mixed value type, Neo4j property coercion은 property value를 조용히 바꿀 수 있다. |
| API creep | Medium | complete GraphML surface는 현재 simple record로 scoped된 package에 schema/key/default/extension semantics를 끌어들인다. |
| fixture maintenance | Medium | hand-written minimal XML만으로 compatibility claim을 만들지 않으려면 real producer sample이 필요하다. |

## Follow-Up Gate

다음 중 하나가 true일 때만 implementation issue를 연다.

- `bluetape-go-workshop` issue #52가 scenario-shaped example에 NDJSON 또는 CSV가 아니라 GraphML을 선택한다.
- `graph/neo4j` 또는 migration workflow가 Gephi, NetworkX, Neo4j APOC, yEd 같은 named producer와 GraphML interchange를
  필요로 한다.
- downstream repository가 NDJSON/CSV conversion으로 합리적으로 처리할 수 없는 representative GraphML fixture를 제공한다.

follow-up issue는 다음을 정의해야 한다.

- package placement 및 dependency policy.
- accepted GraphML subset.
- XML decoder safety limit 및 rejected construct.
- typed property conversion 및 default.
- duplicate ID, missing endpoint, unknown key, malformed value error.
- accepted producer의 fixture corpus.
- round-trip 및 fail-closed test.
- verification command:
  - `go test -count=1 ./graph ./graph/graphio`
  - `go test -race -count=1 ./graph/graphio`

## Outcome

README wording은 deferred follow-up으로 유지하고 이 note를 현재 decision으로 link한다. #433에는 production Go API 또는 behavior
change가 필요하지 않다.
