# Issue #366 Memgraph Compatibility Design

> 한국어 요구사항 경계: 이 spec/design/test-spec 문서는 한국어 독자가 요구사항을 추적할 수 있도록 목적과 검증 경계를 한국어로 보강한다. API 이름, command, code identifier, issue/PR 번호, compatibility matrix, acceptance keyword, DoD/test evidence는 요구사항 약화를 막기 위해 원문 그대로 보존한다. 변경자는 아래 literal contract를 삭제하거나 의미를 약하게 바꾸지 않아야 한다.
> 추가 한국어 검증 메모: 영어로 남은 항목은 대부분 code/API/evidence literal이다. 구현 전에는 한국어 경계 문장과 원문 acceptance checklist를 함께 읽고, 검증 gate가 줄어들지 않았는지 확인한다.\n

## 목표

Prove Memgraph compatibility around the selected `graph/neo4j` adapter path. The
work should reuse the official Neo4j Go driver surface and avoid creating a
standalone `graph/memgraph` abstraction unless compatibility fails.

## Fixture Decision

Use a generic Testcontainers container with `memgraph/memgraph:3.5.0`.

Evidence:

- `go list -m -json github.com/testcontainers/testcontainers-go/modules/memgraph@latest`
  returns no matching versions, so there is no dedicated Testcontainers Go
  Memgraph module to adopt.
- Docker manifest inspection confirms `memgraph/memgraph:3.5.0` is available.

Rejected alternatives:

- `graph/memgraph`: would create a second backend abstraction before the Neo4j
  driver path has shown an incompatibility.
- Lowest-common-denominator query API: #365 deliberately kept broad Cypher DSL
  and backend-neutral repository/session contracts out of the first proof.
- Manual local Memgraph dependency: not repeatable in CI.

## Compatibility Matrix

| Runtime | Fixture | Covered behavior |
|---|---|---|
| Neo4j | `neo4j:5.26.0` via Testcontainers Neo4j module | create/read node and relationship, graph mapping, bad query, cancellation, driver cleanup |
| Memgraph | `memgraph/memgraph:3.5.0` via generic Testcontainers container | same `graph/neo4j` client surface and mapping behavior |

## Guardrails

- Keep compatibility tests on the shared Bolt/Cypher subset.
- Require Bolt-returned `ElementId` values. Deprecated numeric Neo4j IDs remain
  outside the adapter contract.
- Record future runtime-specific behavior as a guardrail or follow-up issue
  before adding any `graph/memgraph` package.

## Test Requirements

- Add a Testcontainers-backed Memgraph test in `graph/neo4j`.
- Cover create/read relationship queries, result mapping into `graph.Vertex`
  and `graph.Edge`, query error behavior, context cancellation, and cleanup.
- Update README/README.ko to explain that Memgraph is compatibility coverage on
  the Neo4j-driver path.
