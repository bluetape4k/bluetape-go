# Issue #218 Database and Storage Server Matrix

Issue: [#218](https://github.com/bluetape4k/bluetape-go/issues/218)  
Parent Epic: [#215](https://github.com/bluetape4k/bluetape-go/issues/215)  
Date: 2026-06-23

## 현재 기준선

- 기존 래퍼는 PostgreSQL, MySQL, Redis, Kafka, NATS다.
- #217의 shared server 계약은 `testcontainers/server`에서 사용할 수 있다.
- `docs/research/2026-06-21-issue-202-source-parity-matrix.md`는
  DB/storage gap을 #218로 라우팅하지만, package consumer가 없는 저가치
  service coverage는 제외한다.

## 로드맵 증거

| Roadmap | Evidence | Fixture Implication |
|---|---|---|
| #100 / #101 SQL DSL and repositories | PostgreSQL, MySQL, MariaDB, SQLite, CockroachDB, ClickHouse, Trino, StarRocks, DuckDB, real database tests가 candidate anchor다. | 가까운 시점의 repository test를 뒷받침할 fixture만 추가한다. MariaDB는 현재 MySQL과 인접하고 검증 비용이 낮다. |
| #47 / #60-#64 AWS helpers | Floci, S3, SQS/SNS, DynamoDB local testing은 AWS track 결정이다. | DynamoDB Local과 S3 emulator 작업은 첫 DB slice가 아니라 #220/#61-#64로 보낸다. |
| #46 / #56-#59 audit/outbox | Audit repository adapter는 이후 SQL, Redis, Kafka/NATS가 필요할 수 있다. | 기존 PostgreSQL/Redis/Kafka/NATS 래퍼와 MariaDB가 초기 SQL/storage 필요를 대부분 충족한다. |
| #44 / #48-#51 graph | AGE/Neo4j/Memgraph/FalkorDB 선택은 #50에 달려 있다. | Backend 선택이 검토될 때까지 graph database fixture는 #220/#50으로 보낸다. |
| #198 MongoDB JWT KeyChain repository | MongoDB backend는 MongoDB driver/testcontainer 경계를 명시적으로 기다린다. | 첫 #218 PR에는 MongoDB helper를 추가하지 않는다. #198에 연결하고 backend issue가 시작될 때 재검토한다. |

## Testcontainers-Go 모듈 가용성

다음 명령으로 확인했다.

```bash
go list -m -versions github.com/testcontainers/testcontainers-go/modules/<name>
```

| Candidate | Module Availability | Roadmap Need | Decision |
|---|---|---|---|
| MariaDB | 현재 repo 버전에서 `github.com/testcontainers/testcontainers-go/modules/mariadb` v0.42.0 사용 가능. | #100/#101 relational SQL/repository parity와 MySQL 인접 coverage. | 첫 slice로 구현한다. |
| MongoDB | module은 사용 가능하지만 Go MongoDB package boundary는 #198에서 추적한다. | JWT distributed backend와 가능한 document storage. | MongoDB package 작업이 시작될 때 #198 또는 child issue로 연기한다. |
| MinIO | module 사용 가능. | S3-compatible examples와 AWS helper work. | Emulator/service 선택은 AWS track이 소유하므로 #220/#61-#64로 연기한다. |
| DynamoDB Local | module 사용 가능. | DynamoDB repository/helper evaluation. | #220/#64로 연기한다. |
| CockroachDB | module 사용 가능. | #100/#101의 SQL dialect candidate. | SQL DSL이 dialect 폭을 결정할 때까지 연기한다. |
| ClickHouse | module 사용 가능. | #100/#101의 analytics dialect candidate. | SQL DSL이 dialect 폭을 결정할 때까지 연기한다. |
| Trino | 이번 확인에서 `go list -m -versions`로 안정 버전이 드러나지 않았다. | Query engine dialect candidate. | #100/#101이 필요성과 module story를 증명할 때까지 연기한다. |
| PostGIS / pgvector | 별도 Testcontainers-Go module은 필요 없고 PostgreSQL image/extension helper일 가능성이 높다. | Repository/search feature를 위한 PostgreSQL extension support. | 기본 relational fixture slice가 들어간 뒤 focused PostgreSQL extension issue로 연기한다. |
| PostgreSQL AGE | PostgreSQL/graph hybrid fixture. | Graph #50 decision. | #220/#50으로 연기한다. |

## 첫 Slice

`testcontainers/mariadb`만 구현한다.

- 기존 `testcontainers/mysql` 사용성을 따른다.
- #217의 `testcontainers/server.Started`를 사용한다.
- `DSNKey = "mariadb.dsn"`을 추가한다.
- v0.42.0의 Testcontainers-Go module 기본값과 맞춰 image
  `mariadb:11.0.3`을 사용한다.
- `database/sql`과 기존 MySQL driver로 검증한다.
- README와 README.ko.md에 dynamic port와 env export 안내를 갱신한다.

## 연기된 후속 작업

- MongoDB: #198이 backend timing을 소유한다. 해당 package boundary가
  시작될 때 MongoDB fixture를 추가한다.
- MinIO, DynamoDB Local, Floci/S3/SQS/SNS: #220과 #61-#64가 AWS/emulator
  선택을 소유한다.
- CockroachDB, ClickHouse, Trino: #100/#101이 먼저 SQL dialect 폭을
  결정해야 한다.
- AGE/Neo4j/Memgraph/FalkorDB: #220/#50이 graph backend 선택을 소유한다.
- PostGIS/pgvector: 기본 relational fixture slice가 들어간 뒤 focused
  PostgreSQL extension issue를 추가한다.
