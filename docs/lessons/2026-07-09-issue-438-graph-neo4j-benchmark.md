# Issue #438 Graph Neo4j/Memgraph Benchmark Lesson

Graph adapter benchmarks should keep pure mapping and database-backed rows
separate.

- Default `go test -run '^$' -bench . ./graph/neo4j` should stay Docker-free
  and measure `dbtype.Node`, `dbtype.Relationship`, and record adaptation
  costs.
- Neo4j/Memgraph Testcontainers rows need an explicit opt-in env flag and
  serial command, because each run owns external database setup and cleanup.
- Container benchmark read/write/seed/error operations should use bounded
  contexts so an opt-in benchmark does not hang indefinitely on a stuck driver
  call.
- Database-backed benchmark output must label image versions and preserve the
  local-runtime caveat; local Testcontainers latency is regression evidence, not
  a production database ranking.
- A measured graph adapter row is not an automatic abstraction mandate. Keep
  `Client` caller-owned and defer backend-neutral repositories until multiple
  adapters and examples prove a shared contract.
