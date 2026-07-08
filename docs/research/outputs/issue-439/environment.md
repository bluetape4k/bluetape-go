# Issue #439 Benchmark Environment

| Field | Value |
|---|---|
| Date | 2026-07-09 |
| Git SHA | cbc5b0af8c44f70d8a6d42572b797d31e994b13a |
| Dirty tree | yes |
| Go version | go1.26.5 |
| GOOS/GOARCH | darwin/arm64 |
| CPU | Apple M5 |
| Logical CPUs | 10 |
| PostgreSQL fixture | postgres:16-alpine via testcontainers/postgres |
| In-memory command | `go test -run '^$' -bench 'Benchmark(MemoryRepository|AuditEntryJSONRoundTrip)' -benchmem ./audit` |
| PostgreSQL command | `BLUETAPE_AUDIT_SQL_OUTBOX_BENCH=1 go test -p 1 -run '^$' -bench '^BenchmarkAuditSQLOutboxPostgres' -benchtime=100x -benchmem ./audit/sqloutbox` |
