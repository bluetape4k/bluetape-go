# Issue #438 Graph Neo4j/Memgraph Benchmark 교훈

Graph adapter benchmark는 pure mapping row와 database-backed row를 분리해야 한다.

- 기본 `go test -run '^$' -bench . ./graph/neo4j`는 Docker-free로 유지하고
  `dbtype.Node`, `dbtype.Relationship`, record adaptation cost를 측정한다.
- Neo4j/Memgraph Testcontainers row는 각 실행이 external database setup과 cleanup을
  소유하므로 명시적 opt-in env flag와 serial command가 필요하다.
- Container benchmark read/write/seed/error operation은 bounded context를 사용해
  opt-in benchmark가 stuck driver call에서 무기한 멈추지 않게 한다.
- Database-backed benchmark output은 image version을 표시하고 local-runtime caveat를
  보존해야 한다. local Testcontainers latency는 regression evidence이지 production
  database ranking이 아니다.
- 측정된 graph adapter row는 자동 abstraction mandate가 아니다. `Client`는
  caller-owned로 유지하고, 여러 adapter와 example이 shared contract를 증명할 때까지
  backend-neutral repository를 미룬다.
