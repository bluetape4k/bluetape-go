# Issue #439 Audit Repository And SQL Outbox Benchmark

Issue: [#439](https://github.com/bluetape4k/bluetape-go/issues/439)  
Milestone: Backlog  
Date: 2026-07-09  
Scope: `audit` in-memory repository 및 `audit/sqloutbox` PostgreSQL relay path

## Snapshot Boundary

이 report는 local benchmark snapshot이다. production ranking이 아니며 audit delivery semantics를 그 자체로 바꾸지 않는다.
낮은 `ns/op`, `ms/op`, `B/op`, `allocs/op`이 더 좋다.

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
| `go test -run '^$' -bench 'Benchmark(MemoryRepository\\|AuditEntryJSONRoundTrip)' -benchmem ./audit` | `docs/research/outputs/issue-439/audit-memory-bench.txt` | in-memory repository 및 JSON row. |
| `BLUETAPE_AUDIT_SQL_OUTBOX_BENCH=1 go test -p 1 -run '^BenchmarkAuditSQLOutboxPostgres' -benchtime=100x -benchmem ./audit/sqloutbox` | `docs/research/outputs/issue-439/audit-sqloutbox-postgres-bench.txt` | serial opt-in PostgreSQL/Testcontainers run. |

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

- in-memory single-aggregate lookup과 small history operation은 test 및 local adapter에 충분히 저렴하다. medium row는 expected
  clone 및 validation cost를 보여 준다. 256-entry `LoadHistory`는 약 0.94 ms/op이며 약 2.0 MiB/op를 allocate한다. repository read가
  defensive copy를 반환하고 contiguous `History`를 재구성하기 때문이다.
- `AggregateType` 기준 `Find`는 aggregate-key lookup보다 의도적으로 넓으며 64x16 in-memory corpus를 scan한 뒤 최신 32 row로
  제한한다. 이는 repository shape visibility row이지 production query design recommendation이 아니다.
- PostgreSQL outbox row는 local `postgres:16-alpine` Testcontainers batch 10개에서 low single-digit millisecond range다.
  `Claim`은 batch 하나를 update/decode하므로 가장 작고, `RunOnce`는 claim과 publish/failure marking을 포함한다.
- dead-letter marking은 bounded failure text와 state transition field를 저장하므로 publish path보다 약간 느리고 allocation이
  조금 더 많다. 현재 correctness-first relay boundary에서는 수용 가능하다.

## Not Proven By This Snapshot

- 이 row는 PostgreSQL을 Redis Streams, Kafka, NATS, RabbitMQ, Redpanda, Pulsar 또는 future publisher adapter와 ranking하지 않는다.
- network latency, WAL setting, connection pool, migration, package DDL 밖 schema index, concurrent relay worker 아래 production
  throughput을 증명하지 않는다.
- #439에서 delivery semantics, claim ownership, idempotency, retry, dead-letter behavior 변경을 정당화하지 않는다.

## Follow-Up Use

- future publisher adapter issue는 throughput 또는 low-latency delivery claim 전에 이 report를 link해야 한다.
- SQL relay throughput이 target이 되면 다음 benchmark는 pooled concurrent relay worker, transaction shape, connection-pool size,
  representative payload redaction/encryption overhead를 추가해야 한다.

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
