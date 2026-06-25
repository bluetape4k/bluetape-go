# Issue #218 Database and Storage Server Matrix

Issue: [#218](https://github.com/bluetape4k/bluetape-go/issues/218)  
Parent Epic: [#215](https://github.com/bluetape4k/bluetape-go/issues/215)  
Date: 2026-06-23

## Current Baseline

- Existing wrappers: PostgreSQL, MySQL, Redis, Kafka, NATS.
- Shared server contract from #217 is available in `testcontainers/server`.
- `docs/research/2026-06-21-issue-202-source-parity-matrix.md` routes
  DB/storage gaps to #218 but excludes low-value service coverage without a
  package consumer.

## Roadmap Evidence

| Roadmap | Evidence | Fixture Implication |
|---|---|---|
| #100 / #101 SQL DSL and repositories | PostgreSQL, MySQL, MariaDB, SQLite, CockroachDB, ClickHouse, Trino, StarRocks, DuckDB, and real database tests are candidate anchors. | Add only fixtures that can back near-term repository tests. MariaDB is current-adjacent to MySQL and cheap to validate. |
| #47 / #60-#64 AWS helpers | Floci, S3, SQS/SNS, and DynamoDB local testing are AWS-track decisions. | Route DynamoDB Local and S3 emulator work to #220/#61-#64 rather than this first DB slice. |
| #46 / #56-#59 audit/outbox | Audit repository adapters may need SQL, Redis, Kafka/NATS later. | Existing PostgreSQL/Redis/Kafka/NATS wrappers plus MariaDB cover the likely early SQL/storage needs. |
| #44 / #48-#51 graph | AGE/Neo4j/Memgraph/FalkorDB selection depends on #50. | Route graph database fixtures to #220/#50 until backend choice is reviewed. |
| #198 MongoDB JWT KeyChain repository | MongoDB backend explicitly waits for MongoDB driver/testcontainer boundaries. | Do not add MongoDB helper in the first #218 PR; link to #198 and revisit when the backend issue starts. |

## Testcontainers-Go Module Availability

Checked with:

```bash
go list -m -versions github.com/testcontainers/testcontainers-go/modules/<name>
```

| Candidate | Module Availability | Roadmap Need | Decision |
|---|---|---|---|
| MariaDB | `github.com/testcontainers/testcontainers-go/modules/mariadb` v0.42.0 available for current repo version. | #100/#101 relational SQL/repository parity and MySQL-adjacent coverage. | Implement first slice. |
| MongoDB | module available, but Go MongoDB package boundary is tracked by #198. | JWT distributed backend and possible document storage. | Defer to #198 or a child issue when MongoDB package work starts. |
| MinIO | module available. | S3-compatible examples and AWS helper work. | Defer to #220/#61-#64 because emulator/service choice belongs to AWS track. |
| DynamoDB Local | module available. | DynamoDB repository/helper evaluation. | Defer to #220/#64. |
| CockroachDB | module available. | SQL dialect candidate for #100/#101. | Defer until SQL DSL chooses dialect breadth. |
| ClickHouse | module available. | Analytics dialect candidate for #100/#101. | Defer until SQL DSL chooses dialect breadth. |
| Trino | no stable version surfaced by `go list -m -versions` in this check. | Query engine dialect candidate. | Defer until #100/#101 proves need and module story. |
| PostGIS / pgvector | No separate Testcontainers-Go module needed; likely PostgreSQL image/extension helpers. | PostgreSQL extension support for repository/search features. | Defer to a focused PostgreSQL extension issue after first wrapper slice. |
| PostgreSQL AGE | PostgreSQL/graph hybrid fixture. | Graph #50 decision. | Defer to #220/#50. |

## First Slice

Implement `testcontainers/mariadb` only:

- mirrors existing `testcontainers/mysql` ergonomics;
- uses `testcontainers/server.Started` from #217;
- adds `DSNKey = "mariadb.dsn"`;
- uses image `mariadb:11.0.3`, matching Testcontainers-Go module default for
  v0.42.0;
- verifies with `database/sql` and the existing MySQL driver;
- updates README and README.ko.md with dynamic port and env export guidance.

## Deferred Follow-Ups

- MongoDB: #198 owns backend timing; add a MongoDB fixture when that package
  boundary starts.
- MinIO, DynamoDB Local, Floci/S3/SQS/SNS: #220 and #61-#64 own AWS/emulator
  selection.
- CockroachDB, ClickHouse, Trino: #100/#101 must first decide SQL dialect
  breadth.
- AGE/Neo4j/Memgraph/FalkorDB: #220/#50 owns graph backend selection.
- PostGIS/pgvector: add a focused PostgreSQL extension issue after the base
  relational fixture slice lands.

