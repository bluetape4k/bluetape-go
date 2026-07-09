# Issue #439 Audit Repository And SQL Outbox Benchmark

Issue: [#439](https://github.com/bluetape4k/bluetape-go/issues/439)  
Milestone: Backlog  
Date: 2026-07-09  
Scope: `audit` in-memory repository and `audit/sqloutbox` PostgreSQL relay paths

## Snapshot Boundary

This report is a local benchmark snapshot. It is not a production ranking and
does not change audit delivery semantics by itself. Lower `ns/op`, `ms/op`,
`B/op`, and `allocs/op` are better.

![audit + sqloutbox benchmark summary](../images/readme-charts/audit-outbox-benchmark-summary.png)

## Environment

| Field | Value |
|---|---|
| OS/Arch | darwin/arm64 |
| CPU | Apple M5 |
| Logical CPUs | 10 |
| Go version | go1.26.3 |
| Git SHA | cbc5b0af8c44f70d8a6d42572b797d31e994b13a |
| Dirty tree | yes, benchmark branch under development |
| PostgreSQL fixture | `postgres:16-alpine` via `testcontainers/postgres` |

## Commands And Outputs

| Command | Raw output file | Notes |
|---|---|---|
| `go test -run '^$' -bench 'Benchmark(MemoryRepository\|AuditEntryJSONRoundTrip)' -benchmem ./audit` | `docs/research/outputs/issue-439/audit-memory-bench.txt` | In-memory repository and JSON rows. |
| `BLUETAPE_AUDIT_SQL_OUTBOX_BENCH=1 go test -p 1 -run '^$' -bench '^BenchmarkAuditSQLOutboxPostgres' -benchtime=100x -benchmem ./audit/sqloutbox` | `docs/research/outputs/issue-439/audit-sqloutbox-postgres-bench.txt` | Serial opt-in PostgreSQL/Testcontainers run. |

## In-Memory Results

| Benchmark | Shape | ns/op | B/op | allocs/op |
|---|---|---:|---:|---:|
| `BenchmarkMemoryRepositoryFind/SingleAggregateHistory16-10` | 1 aggregate, 16 entries, payload 256 B | 2,770 | 17,440 | 54 |
| `BenchmarkAuditEntryJSONRoundTrip/Payload256-10` | entry JSON encode/decode, payload 256 B | 10,054 | 4,974 | 56 |
| `BenchmarkMemoryRepositoryLoadHistory/Small16/Payload256-10` | 1 aggregate history load, 16 entries | 14,296 | 47,084 | 162 |
| `BenchmarkMemoryRepositoryAppend/History16/Batch1/Payload256-10` | append 1 entry after 16-entry history | 17,189 | 45,226 | 125 |
| `BenchmarkAuditEntryJSONRoundTrip/Payload2048-10` | entry JSON encode/decode, payload 2 KiB | 34,231 | 12,345 | 56 |
| `BenchmarkMemoryRepositoryFind/TypeScan64AggregatesLimit32-10` | 64 aggregates x 16 entries, newest 32 | 283,617 | 1,400,594 | 3,084 |
| `BenchmarkMemoryRepositoryAppend/History256/Batch8/Payload2048-10` | append 8 entries after 256-entry history | 857,813 | 1,522,878 | 1,612 |
| `BenchmarkMemoryRepositoryLoadHistory/Medium256/Payload2048-10` | 1 aggregate history load, 256 entries | 942,250 | 2,126,793 | 2,329 |

## PostgreSQL Outbox Results

| Benchmark | Shape | ms/op | B/op | allocs/op |
|---|---|---:|---:|---:|
| `BenchmarkAuditSQLOutboxPostgres/Claim/Limit10/Pending100/Payload512-10` | claim 10 rows from 100 pending | 1.70 | 96,471 | 819 |
| `BenchmarkAuditSQLOutboxPostgres/Enqueue/Batch10/Payload512-10` | enqueue 10 entries | 2.64 | 86,278 | 366 |
| `BenchmarkAuditSQLOutboxPostgres/RunOnce/Publish10/Payload512-10` | claim and mark 10 rows published | 2.72 | 104,983 | 957 |
| `BenchmarkAuditSQLOutboxPostgres/RunOnce/DeadLetter10/Payload512-10` | claim and mark 10 rows dead-lettered | 2.93 | 111,107 | 987 |

## Interpretation

- In-memory single-aggregate lookup and small history operations are cheap
  enough for tests and local adapters. The medium rows show the expected clone
  and validation cost: 256-entry `LoadHistory` is about 0.94 ms/op and allocates
  about 2.0 MiB/op because repository reads return defensive copies and then
  reconstruct a contiguous `History`.
- `Find` over `AggregateType` is intentionally broader than aggregate-key lookup
  and scans a 64x16 in-memory corpus before limiting to 32 newest rows. Treat
  this as a visibility row for repository shape, not as a recommendation for
  production query design.
- PostgreSQL outbox rows are in the low single-digit millisecond range for
  local `postgres:16-alpine` Testcontainers batches of 10. `Claim` is the
  smallest row because it updates and decodes one batch, while `RunOnce`
  includes claim plus publish/failure marking.
- Dead-letter marking is slightly slower and allocates slightly more than the
  publish path because it persists bounded failure text and state transition
  fields. This is acceptable for the current correctness-first relay boundary.

## Not Proven By This Snapshot

- These rows do not rank PostgreSQL against Redis Streams, Kafka, NATS, RabbitMQ,
  Redpanda, Pulsar, or any future publisher adapter.
- They do not prove production throughput under network latency, WAL settings,
  connection pools, migrations, schema indexes beyond the package DDL, or
  concurrent relay workers.
- They do not justify changing delivery semantics, claim ownership, idempotency,
  retry, or dead-letter behavior in #439.

## Follow-Up Use

- Future publisher adapter issues should link this report before making
  throughput or low-latency delivery claims.
- If SQL relay throughput becomes a target, the next benchmark should add
  pooled concurrent relay workers, transaction shape, connection-pool size, and
  representative payload redaction/encryption overhead.

## Artifacts

| Artifact | Path |
|---|---|
| Chart PNG | `docs/images/readme-charts/audit-outbox-benchmark-summary.png` |
| Chart SVG | `docs/images/readme-charts/audit-outbox-benchmark-summary.svg` |
| Chart data | `docs/images/readme-charts/audit-outbox-benchmark-summary.vl.json` |
| Chart generator | `docs/images/readme-charts/generate-audit-outbox-benchmark-summary.mjs` |
| Environment | `docs/research/outputs/issue-439/environment.md` |
| In-memory raw output | `docs/research/outputs/issue-439/audit-memory-bench.txt` |
| PostgreSQL raw output | `docs/research/outputs/issue-439/audit-sqloutbox-postgres-bench.txt` |
