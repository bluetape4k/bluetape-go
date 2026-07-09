# Issue #438 environment

- Date: 2026-07-09
- Base commit: abed706ba4d3f6dde6d88e1605f51fd65aa4a84a
- Dirty tree: yes, issue #438 benchmark/documentation diff
- Go: go version go1.26.5 darwin/arm64
- OS/arch: Darwin 25.5.0 arm64
- CPU: Apple M5
- Neo4j benchmark image: neo4j:5.26.0
- Memgraph benchmark image: memgraph/memgraph:3.5.0
- Container benchmark opt-in env: BLUETAPE_GRAPH_NEO4J_BENCH=1
- Container benchmark operation timeout: 10s per read/write/seed/error operation

## Docker

- Client: 29.6.1
- Server: 28.4.0

## Commands

- `go test -run '^$' -bench . -benchmem ./graph/neo4j`
- `BLUETAPE_GRAPH_NEO4J_BENCH=1 go test -p 1 -run '^$' -bench '^BenchmarkGraphNeo4jContainers' -benchtime=100x -benchmem ./graph/neo4j`
