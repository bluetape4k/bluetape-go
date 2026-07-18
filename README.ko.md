# bluetape-go

[English](README.md) | [한국어](README.ko.md)

![bluetape-go 대표 이미지](docs/assets/bluetape-go-hero.png)

bluetape 생태계를 위한 Go다운 백엔드 유틸리티와 분산 인프라 패키지입니다.

`bluetape-go`는 Kotlin/JVM 기반 bluetape4k 라이브러리를 대체하거나 API 형태를
그대로 옮기는 프로젝트가 아닙니다. Go를 쓰는 팀이 서비스 인프라, 분산 조정,
테스트 fixture, resilience, cache, workflow, batch 처리, portable value, Redis
adapter를 작은 패키지로 가져다 쓸 수 있게 만든 별도 구현입니다.

## 아키텍처

![bluetape-go Architecture Overview](docs/assets/bluetape-go-architecture-overview.png)

## 로깅

`bluetape-go` 예제는 structured logging에 standard-library `log/slog` 계약을
사용합니다. Application이 handler와 level을 설정하고, library package는 global
logging default를 바꾸거나 bluetape-go logger registry를 설치하지 않습니다.
대신 `resilience.OnEvent` 같은 caller-owned hook을 노출합니다. 비용이 큰 debug
attribute는 계산 전에 `logger.Enabled(ctx, slog.LevelDebug)`로 guard하세요.

## 현재 상태

`bluetape-go`는 `v0.17.0` 릴리스 선을 배포했습니다. 현재 repository에는
foundation helper, codec, compression, context-aware concurrency, serializer
contract, Redis 기반 leader election과 lock, resilience policy, cache
coordination, token-bucket rate limiting, finite state machine, workflow report,
lightweight workflow runner, checkpoint 기반 batch job, portable service value,
SQL helper, text search primitive, audit/event package, graph helper, bounded
image helper, encryption helper, first-party rule primitive가 들어 있습니다.
`0.18.0` 개발 선은 MongoDB group/strategic leader elector, bounded GraphML
graph I/O, Redis Streams sqloutbox publisher provider까지 release-prep 상태로
정리되었습니다. `v0.18.0` tag는 release PR, main promotion, tag, GitHub
Release gate가 끝난 뒤 생성합니다.

`v0.6.x` portable utility 범위에는 UUID, ULID, KSUID, Snowflake ID 생성,
명시적 algorithm 기반 JWT signing/parsing/validation, 인메모리/Redis/MongoDB
KeyChain repository 기반 distributed key rotation, typed unit과 measured value,
ISO currency와 decimal-backed money 연산, 인메모리 Bloom 및 Redis-backed Bloom과
HyperLogLog helper가 포함됩니다. RedisBloom `CF*` Cuckoo 지원은 현재 public
API가 아니라 module-gated future scope입니다.

## 패키지

| 패키지 | 상태 | 목적 |
|---|---:|---|
| [`core`](core/README.ko.md) | active | 작은 공용 validation, zero/default, pointer, string, number helper. |
| [`collections`](collections/README.ko.md) | active | chunking, grouping, distinct, error-aware transform용 작은 generic slice/map helper. |
| [`concurrency`](concurrency/README.ko.md) | active | context-aware goroutine group, worker pool, bounded parallel helper. |
| [`codec`](codec/README.ko.md) | active | Base58, Base62, Base64, hex, URL-safe encoding helper. |
| [`encrypt`](encrypt/README.ko.md) | active | Versioned envelope와 associated data를 지원하는 stdlib AES-GCM byte/string facade. |
| [`compression`](compression/README.ko.md) | active | gzip, deflate, zstd, lz4, snappy, registry 기반 compression helper. |
| [`imagekit`](imagekit/README.ko.md) | active | 서비스 입력을 위한 bounded pure-Go thumbnail, resize, JPEG/PNG conversion helper. |
| [`serialization`](serialization/README.ko.md) | active | 안전한 기본값을 가진 JSON/binary serializer interface. |
| [`testing`](testing/README.ko.md) | active | eventual consistency 테스트용 공용 helper. |
| [`testing/concurrency`](testing/concurrency/README.ko.md) | active | concurrent test를 위한 stress/async job helper. |
| [`testcontainers/redis`](testcontainers/redis/README.ko.md) | active | Testcontainers for Go 기반 Redis fixture. |
| [`testcontainers/postgres`](testcontainers/postgres/README.ko.md) | active | Testcontainers for Go 기반 PostgreSQL fixture. |
| [`testcontainers/mysql`](testcontainers/mysql/README.ko.md) | active | Testcontainers for Go 기반 MySQL 8.4 fixture. |
| [`testcontainers/mongodb`](testcontainers/mongodb/README.ko.md) | active | Testcontainers for Go 기반 MongoDB fixture. |
| [`testcontainers/nats`](testcontainers/nats/README.ko.md) | active | Testcontainers for Go 기반 NATS fixture. |
| [`testcontainers/kafka`](testcontainers/kafka/README.ko.md) | active | Testcontainers for Go 기반 Kafka fixture. |
| [`dynamodb/batchwrite`](dynamodb/batchwrite/README.ko.md) | active | AWS SDK for Go v2 BatchWriteItem 25개 chunking과 unprocessed-item retry helper. |
| [`examples/integration`](examples/integration/README.ko.md) | example | 수정된 `0.6.x` package를 묶는 compile-checked end-to-end recipe. |
| [`examples/audit`](examples/audit/README.ko.md) | example | Repository history와 outbox replay boundary를 보여주는 runnable audit-backed order service. |
| [`examples/graph/observability`](examples/graph/observability/README.ko.md) | example | Blast radius, alert boundary, ownership, NDJSON graph I/O 경계를 보여주는 runnable observability incident graph. |
| [`examples/graph/iamaccess`](examples/graph/iamaccess/README.ko.md) | example | Effective access, deny path, risky privilege chain, least-privilege drift, NDJSON graph I/O 경계를 보여주는 runnable IAM access graph. |
| [`examples/s3`](examples/s3/README.ko.md) | example | Floci fixture 기반 compile-checked AWS SDK for Go v2 S3 예제. |
| [`examples/sqs-sns`](examples/sqs-sns/README.ko.md) | example | Floci fixture 기반 compile-checked AWS SDK for Go v2 SQS/SNS 예제. |
| [`leader`](leader/README.ko.md) | active | 단일, group, strategy 기반 계약을 포함한 leader election API. |
| [`leader/redis`](leader/redis/README.ko.md) | active | TTL renewal, ZSET slot token, candidate registry 기반 Redis 단일/group/strategic leader election 구현. |
| [`leader/mongo`](leader/mongo/README.ko.md) | active | Owner-token lease, bounded slot, candidate registry, TTL cleanup index를 사용하는 MongoDB 단일/group/strategic leader election 구현. |
| [`leader/sql`](leader/sql/README.ko.md) | active | Caller-owned row lease와 caller-owned `*sql.DB`를 사용하는 PostgreSQL 전용 단일 leader election 구현. |
| [`resilience`](resilience/README.ko.md) | active | service call을 위한 자체 composable retry, timeout, circuit breaker, bulkhead policy, synchronous observability hook, `net/http` adapter. |
| [`cache`](cache/README.ko.md) | active | context-aware loader와 same-key stampede protection을 제공하는 generic in-process TTL cache interface. |
| [`cache/redisnear`](cache/redisnear/README.ko.md) | active | process-local loading cache를 위한 Redis Pub/Sub near-cache invalidation. |
| [`cache/rediscoord`](cache/rediscoord/README.ko.md) | active | cold burst 동안 하나의 loader 결과를 process-local cache 사이에서 공유하는 opt-in Redis coordination wrapper. |
| [`cache/redisfory`](cache/redisfory/README.ko.md) | active | 명시적인 schema generation으로 Redis에 직접 저장하는 bounded Go-native Apache Fory binary value cache. |
| [`cache/redisvalue`](cache/redisvalue/README.ko.md) | active | Reference를 보존하는 process-local tiered decorator와 bounded serialized Redis L2 value cache. |
| [`redis`](redis/README.ko.md) | active | Redis key, owner-token, lease script, TTL, redacted operation error를 위한 공유 primitive. |
| [`lock/redis`](lock/redis/README.ko.md) | active | TTL acquire와 owner-safe Lua unlock을 제공하는 Redis 단일 인스턴스 owner-token lock. |
| [`ratelimit`](ratelimit/README.ko.md) | active | process-local keyed token-bucket limiter와 `net/http` middleware. |
| [`ratelimit/redis`](ratelimit/redis/README.ko.md) | active | atomic Lua consume/refill과 idle key expiration을 쓰는 Redis-backed token-bucket limiter. |
| [`ratelimit/sql`](ratelimit/sql/README.ko.md) | active | Caller-owned schema와 cleanup을 사용하는 moderate-QPS, database-only 배포용 PostgreSQL atomic token bucket. |
| [`state`](state/README.ko.md) | active | typed transition, guard, final state, sentinel error를 제공하는 작은 finite state machine primitive. |
| [`workreport`](workreport/README.ko.md) | active | lightweight workflow code를 위한 status, failure-policy, report-tree value. |
| [`workflow`](workflow/README.ko.md) | active | `context.Context`와 `workreport` 기반 sequential, conditional, all-branches parallel runner. |
| [`batch`](batch/README.ko.md) | active | Chunk-oriented batch step, sequential job, retry/skip policy, report, checkpoint. |
| [`batch/sqlcheckpoint`](batch/sqlcheckpoint/README.ko.md) | active | Batch callback과 consumed-input checkpoint를 revision CAS로 함께 commit하는 PostgreSQL durable checkpoint provider. |
| [`id`](id/README.ko.md) | active | UUID v4/v7, random/monotonic ULID, standard KSUID, Kotlin-compatible KSUID millis, Snowflake ID generator. |
| [`jwt`](jwt/README.ko.md) | active | 명시적 algorithm을 사용하는 JWT signing, parsing, validation, typed claim reading, in-memory/distributed `kid` key rotation, optional provider cache adapter. |
| [`jwt/redis`](jwt/redis/README.ko.md) | active | Distributed JWT key-chain repository 생성을 위한 Redis 전용 facade. |
| [`jwt/mongo`](jwt/mongo/README.ko.md) | active | Distributed JWT key-chain repository 생성을 위한 MongoDB 전용 facade. |
| [`measure`](measure/README.ko.md) | active | Typed unit, measured value, compound unit, parsing, formatting, affine temperature helper. |
| [`money`](money/README.ko.md) | active | ISO 4217 통화 값, CLDR-backed locale currency lookup, decimal-backed 금액, 합산, 직렬화, caller-supplied 환율 변환, ECB-backed provider 변환. |
| [`rules`](rules/README.ko.md) | active | Dependency-free facts, functional rule, deterministic rule set, composite group, bounded inference, result detail, context cancellation. |
| [`sqlkit`](sqlkit/README.ko.md) | active | Runtime-first `database/sql` transaction helper, 명시적 row mapping/cardinality helper, PostgreSQL 우선 inspectable SQL builder. |
| [`audit`](audit/README.ko.md) | active | validated JSON entry, pending event recording, history reconstruction을 제공하는 storage-neutral aggregate event/audit model. |
| [`audit/sqloutbox`](audit/sqloutbox/README.ko.md) | active | Caller-owned transaction choreography를 유지하는 PostgreSQL-backed audit outbox store와 relay. |
| [`audit/sqloutbox/redisstreams`](audit/sqloutbox/redisstreams/README.ko.md) | active | 안정적인 event/idempotency metadata를 보존하는 Redis Streams sqloutbox publisher. |
| [`audit/sqloutbox/sqloutboxtest`](audit/sqloutbox/sqloutboxtest/README.ko.md) | active | sqloutbox test, example, retry, duplicate-delivery assertion을 위한 deterministic publisher helper. |
| [`graph`](graph/README.ko.md) | active | Vertex, edge, path, label, ID, shallow property, validated JSON을 제공하는 model-only graph value. |
| [`graph/graphio`](graph/graphio/README.ko.md) | active | Graph vertex/edge를 위한 bounded NDJSON 및 paired CSV import/export helper. |
| [`graph/neo4j`](graph/neo4j/README.ko.md) | active | 공식 Neo4j Go driver 결과를 graph vertex/edge로 변환하는 proof adapter. |
| [`probabilistic`](probabilistic/README.ko.md) | active | deterministic config, merge compatibility check, stress/race coverage를 갖춘 goroutine-safe 인메모리 Bloom filter. |
| [`probabilistic/redis`](probabilistic/redis/README.ko.md) | active | Static Lua Bloom script, immutable config metadata, operator runbook 경계를 갖춘 Redis-backed shared Bloom filter와 HyperLogLog estimate. |

다음 계획 패키지군은 추가 durable audit transport publisher adapter와 example
service입니다. Redis-backed Cuckoo 지원은 Redis Bloom과 HyperLogLog 범위 이후
별도로 추적합니다.

## SerDe Baseline Guidance

0.14.0 cross-repo SerDe matrix는 default를 보수적으로 유지합니다. Go
serialization은 JSON, raw bytes/strings, Go-local `BTGS` envelope를 사용하고,
`compression.Default()`는 zstd로 유지합니다. lz4/snappy는 throughput 후보,
gzip/deflate는 compatibility 선택지이며, Base58/Base62/URL62는 large binary
transport codec이 아니라 ID/key surface입니다.
[`docs/research/2026-07-07-issue-402-cross-repo-serde-recommendation.md`](docs/research/2026-07-07-issue-402-cross-repo-serde-recommendation.md)를 참고하세요.

## 설치

```bash
go get github.com/bluetape4k/bluetape-go
```

## 패키지 문서

각 package README에는 실제 사용 예제, 운영 경계, benchmark 메모, root 개요에
넣기에는 너무 구체적인 제약을 정리합니다.

- Foundation: [`core`](core/README.ko.md), [`collections`](collections/README.ko.md),
  [`concurrency`](concurrency/README.ko.md), [`codec`](codec/README.ko.md),
  [`encrypt`](encrypt/README.ko.md), [`compression`](compression/README.ko.md),
  [`serialization`](serialization/README.ko.md).
- Test support: [`testing`](testing/README.ko.md),
  [`testing/concurrency`](testing/concurrency/README.ko.md), 위 표의 Testcontainers
  fixture package README. `testing`의 focused example은 assertion DSL 추가 없이
  table-driven test, package-local builder, golden file, deterministic random data,
  cancellation assertion을 다룹니다.
- AWS/Floci: [`dynamodb/batchwrite`](dynamodb/batchwrite/README.ko.md),
  [`examples/integration`](examples/integration/README.ko.md),
  [`examples/s3`](examples/s3/README.ko.md),
  [`examples/sqs-sns`](examples/sqs-sns/README.ko.md).
- Text: deterministic multi-pattern search, tokenizer core interface,
  blockword detection/masking, severity metadata, normalization,
  boundary-aware matching을 위한
  [`textsearch`](textsearch/README.ko.md) 및 optional Kagome adapter인
  [`textsearch/japanese`](textsearch/japanese/README.ko.md), Lingua-Go detector인
  [`textsearch/language`](textsearch/language/README.ko.md).
- Image: 명시적 format과 memory boundary를 가진 bounded pure-Go resize,
  thumbnail, JPEG/PNG conversion helper인 [`imagekit`](imagekit/README.ko.md).
- Coordination: [`leader`](leader/README.ko.md),
  [`leader/redis`](leader/redis/README.ko.md),
  [`leader/mongo`](leader/mongo/README.ko.md),
  [`leader/sql`](leader/sql/README.ko.md), [`redis`](redis/README.ko.md),
  [`redis/stream`](redis/stream/README.ko.md), [`lock/redis`](lock/redis/README.ko.md).
- Runtime policy/cache/state/workflow: [`resilience`](resilience/README.ko.md),
  [`cache`](cache/README.ko.md), [`cache/redisnear`](cache/redisnear/README.ko.md),
  [`cache/rediscoord`](cache/rediscoord/README.ko.md), [`cache/redisfory`](cache/redisfory/README.ko.md),
  [`cache/redisvalue`](cache/redisvalue/README.ko.md),
  [`ratelimit`](ratelimit/README.ko.md),
  [`state`](state/README.ko.md), [`workreport`](workreport/README.ko.md),
  [`workflow`](workflow/README.ko.md), [`batch`](batch/README.ko.md).
- Portable utility: [`id`](id/README.ko.md), [`jwt`](jwt/README.ko.md),
  [`jwt/redis`](jwt/redis/README.ko.md), [`jwt/mongo`](jwt/mongo/README.ko.md),
  [`measure`](measure/README.ko.md), [`money`](money/README.ko.md),
  [`rules`](rules/README.ko.md),
  [`probabilistic`](probabilistic/README.ko.md) 및
  [`probabilistic/redis`](probabilistic/redis/README.ko.md).
- Data access: [`sqlkit`](sqlkit/README.ko.md) 및 optional
  [SQL generator/migration guide](docs/sql-generator-migration-guidance.ko.md).
- Audit: storage-neutral aggregate event value, pending event handoff, validated
  audit entry JSON, history reconstruction을 위한 [`audit`](audit/README.ko.md),
  PostgreSQL-backed at-least-once outbox delivery를 위한
  [`audit/sqloutbox`](audit/sqloutbox/README.ko.md), Redis Streams publish
  attempt를 위한
  [`audit/sqloutbox/redisstreams`](audit/sqloutbox/redisstreams/README.ko.md),
  deterministic publisher helper인 [`audit/sqloutbox/sqloutboxtest`](audit/sqloutbox/sqloutboxtest/README.ko.md),
  runnable audit-backed order service인 [`examples/audit`](examples/audit/README.ko.md).
- Graph: model-only vertex, edge, path, label, ID, shallow property, validated
  JSON value를 제공하는 [`graph`](graph/README.ko.md), bounded NDJSON/paired CSV
  import/export helper를 제공하는 [`graph/graphio`](graph/graphio/README.ko.md),
  첫 Neo4j backend proof인 [`graph/neo4j`](graph/neo4j/README.ko.md),
  runnable incident-response graph 예제인
  [`examples/graph/observability`](examples/graph/observability/README.ko.md),
  IAM access-path review 예제인
  [`examples/graph/iamaccess`](examples/graph/iamaccess/README.ko.md).

## Workshop Adoption

동반
[`bluetape-go-workshop`](https://github.com/bluetape4k/bluetape-go-workshop)
repository가 runnable scenario tutorial을 소유합니다. 이 library README는 tutorial
본문을 중복하지 않고 matching workshop example과 active backlog만 연결합니다.
Source-checked adoption matrix는
[`docs/research/2026-07-08-issue-415-workshop-adoption-matrix.md`](docs/research/2026-07-08-issue-415-workshop-adoption-matrix.md)
및 issue [#415](https://github.com/bluetape4k/bluetape-go/issues/415)에
기록했습니다.

- SQL adoption: [`sql-access-strategy-decision`](https://github.com/bluetape4k/bluetape-go-workshop/tree/develop/examples/sql-access-strategy-decision),
  [`sql-order-repository`](https://github.com/bluetape4k/bluetape-go-workshop/tree/develop/examples/sql-order-repository),
  [`sql-transaction-boundary`](https://github.com/bluetape4k/bluetape-go-workshop/tree/develop/examples/sql-transaction-boundary),
  [`gin-sql-crud-api`](https://github.com/bluetape4k/bluetape-go-workshop/tree/develop/examples/gin-sql-crud-api),
  [`gin-sql-order-service`](https://github.com/bluetape4k/bluetape-go-workshop/tree/develop/examples/gin-sql-order-service).
- AWS/Floci adoption: [`s3-floci-storage`](https://github.com/bluetape4k/bluetape-go-workshop/tree/develop/examples/s3-floci-storage),
  [`sqs-floci-worker`](https://github.com/bluetape4k/bluetape-go-workshop/tree/develop/examples/sqs-floci-worker),
  [`dynamodb-batchwrite-materializer`](https://github.com/bluetape4k/bluetape-go-workshop/tree/develop/examples/dynamodb-batchwrite-materializer),
  [`dynamodb-conditional-repository`](https://github.com/bluetape4k/bluetape-go-workshop/tree/develop/examples/dynamodb-conditional-repository),
  [`s3-sqs-dynamodb-document-workflow`](https://github.com/bluetape4k/bluetape-go-workshop/tree/develop/examples/s3-sqs-dynamodb-document-workflow).
- Probabilistic adoption: [`probabilistic-dedupe-admission`](https://github.com/bluetape4k/bluetape-go-workshop/tree/develop/examples/probabilistic-dedupe-admission),
  [`shared-redis-bloom-admission`](https://github.com/bluetape4k/bluetape-go-workshop/tree/develop/examples/shared-redis-bloom-admission), planned
  [Redis HyperLogLog workflow](https://github.com/bluetape4k/bluetape-go-workshop/issues/151).
- Text adoption: [`text-moderation-masking`](https://github.com/bluetape4k/bluetape-go-workshop/tree/develop/examples/text-moderation-masking),
  [`gin-text-search-service`](https://github.com/bluetape4k/bluetape-go-workshop/tree/develop/examples/gin-text-search-service),
  follow-up issue
  [#34](https://github.com/bluetape4k/bluetape-go-workshop/issues/34),
  [#55](https://github.com/bluetape4k/bluetape-go-workshop/issues/55),
  [#67](https://github.com/bluetape4k/bluetape-go-workshop/issues/67),
  [#118](https://github.com/bluetape4k/bluetape-go-workshop/issues/118),
  [#119](https://github.com/bluetape4k/bluetape-go-workshop/issues/119).
- Audit, graph, logging adoption은 issue-tracked workshop scope로 남깁니다.
  Audit [#35](https://github.com/bluetape4k/bluetape-go-workshop/issues/35),
  [#56](https://github.com/bluetape4k/bluetape-go-workshop/issues/56),
  [#57](https://github.com/bluetape4k/bluetape-go-workshop/issues/57),
  [#58](https://github.com/bluetape4k/bluetape-go-workshop/issues/58),
  [#68](https://github.com/bluetape4k/bluetape-go-workshop/issues/68),
  [#150](https://github.com/bluetape4k/bluetape-go-workshop/issues/150);
  graph [#36](https://github.com/bluetape4k/bluetape-go-workshop/issues/36),
  [#50](https://github.com/bluetape4k/bluetape-go-workshop/issues/50),
  [#51](https://github.com/bluetape4k/bluetape-go-workshop/issues/51),
  [#52](https://github.com/bluetape4k/bluetape-go-workshop/issues/52),
  [#69](https://github.com/bluetape4k/bluetape-go-workshop/issues/69);
  slog [#139](https://github.com/bluetape4k/bluetape-go-workshop/issues/139).
  Workshop backlog은 이 library release line을 block하지 않습니다.

### Graph I/O 한눈에 보기

![Graph I/O Record Flow](docs/images/readme-diagrams/graph-io-record-flow.png)

`graph/graphio`는 import/export를 record-stream boundary에 고정합니다. Reader는
`graph.Vertex`와 `graph.Edge`를 반환하기 전에 byte, column, record count,
duplicate vertex, missing endpoint를 검사합니다. Writer는 deterministic NDJSON
또는 paired CSV record를 내보냅니다. Optional `graph/graphio/graphml`은 bounded
directed GraphML subset을 담당하며 broad yEd/yFiles, filesystem, backend
ownership은 주장하지 않습니다.

### Observability Graph 예제

![Observability Incident Graph](docs/images/readme-diagrams/graph-observability-incident-topology.png)

Observability 예제는 checkout API, service dependency, alert, incident root
cause, owning team을 seed로 구성합니다. Backend adapter는 follow-up으로 남겨두고,
upstream impact, affected API, alert boundary, ownership, NDJSON round-trip을
compile-checked query로 증명합니다.

### IAM Access Graph 예제

![IAM Access Graph Paths](docs/images/readme-diagrams/graph-iam-access-paths.png)

IAM 예제는 user, group, role, policy, permission, resource, break-glass grant를
seed로 구성합니다. Backend adapter 없이 effective access, explicit deny path,
risky nested admin inheritance, least-privilege drift, temporary access, NDJSON
round-trip을 compile-checked query로 증명합니다.

### Audit Example 한눈에 보기

![Audit Example Service Flow](docs/images/readme-diagrams/audit-example-service-flow.png)

Audit 예제는 일부러 작게 만들었습니다. 현재 상태를 가진 source model과 변경 이력을
담는 audit history를 분리합니다. Command는 `audit.Repository`를 통해
`audit.Entry`를 append하고, append가 성공한 뒤에만 예제 order state를 바꿉니다.
History query도 같은 repository boundary를 읽습니다. Outbox replay는 최소
`EntrySink`만 사용하므로 운영 code에서는 in-memory fixture 대신 `audit/sqloutbox`를
연결하면 됩니다. Adoption path도 명시적으로 유지합니다. `Store.Enqueue`가 durable
row를 쓰고, `Relay.RunOnce` 또는 `Relay.Run`이 claim하며,
`sqloutbox.Publisher` adapter가 duplicate-safe consumer를 위해 `Record.EventID` /
`Record.IdempotencyKey`를 보존합니다. 예제를 framework로 키우지 않고 boundary만
드러내려는 의도입니다.

## Roadmap

| Milestone | 주제 |
|---|---|
| `0.1.0` | Core support, collections, goroutine helper, codec, compression, Redis leader election, Testcontainers. |
| `0.2.0` | Resilience primitive: retry, timeout, circuit breaker, bulkhead, HTTP middleware. |
| `0.3.0` | Cache/coordination: near cache, Redis lock, token-bucket rate limiting, strategic leader election. |
| `0.4.0` | State machine과 lightweight workflow primitive. |
| `0.5.0` | Checkpoint 기반 batch processing과 leader-guarded example. |
| `0.6.0` | ID generation, JWT, measured value, money, probabilistic structure. |
| `0.6.1` | Redis Bloom filter, provider cache, exchange-rate provider, locale currency mapping, compatibility evidence를 포함한 portable utility hardening. |
| `0.6.2` | Core, testing, Testcontainers corrective source-parity matrix와 hardening plan. |
| `0.6.3` | Core foundation parity와 hardening. |
| `0.6.4` | JUnit5-inspired Go testing helper parity. |
| `0.6.5` | Testcontainers contract hardening과 service coverage expansion. |
| `0.6.6` | Developer-experience parity, integration example, corrective-series closure. |
| `0.7.0` | Relational SQL DSL과 repository helper. |
| `0.8.0` | Text search, blockword masking, tokenizer adapter. |
| `0.9.0` | bluetape4k-javers 패턴 기반 audit/event package. |
| `0.10.0` | Graph package와 example. |
| `0.11.0` | Image, encryption, rule-engine research, utility follow-up. |
| `0.12.0` | Core foundation parity: core, collections, codec, concurrency, logging convention, first-party rules primitive를 source-backed replacement로 보강. |
| `0.13.0` | Retrospective hardening: 7-tier review, stress/race coverage, P0/P1 fix, cumulative lesson cleanup, MongoDB Testcontainers fixture, feature-gap triage, release-readiness audit. |
| `0.14.0` | Cross-repo SerDe/compression benchmark evidence, raw artifact retention, evidence-scoped recommendation matrix. |
| `0.15.0` | Audit sqloutbox publisher adoption helper와 focused JSON/zstd allocation reduction. |
| `0.16.0` | Redis probabilistic follow-up: HyperLogLog support, Testcontainers/stress/race coverage, 명시적인 RedisBloom Cuckoo deferral. |
| `0.17.0` | Workshop adoption sync: library-side pointer, cross-repo issue link, library readiness와 workshop backlog를 분리한 release-readiness note. |
| `0.18.0` | Ecosystem follow-up: MongoDB group/strategic leader elector, bounded GraphML graph I/O, Redis Streams sqloutbox publisher provider. |

닫힌 `0.7.0 Research Gate` milestone은 큰 도메인 범위 결정을 기록한
research milestone이며 release tag를 만들지 않았습니다.

현재 계획은 [GitHub milestones](https://github.com/bluetape4k/bluetape-go/milestones)
와 [`docs/research`](docs/research/)에서 확인할 수 있습니다.

## 개발

```bash
make test
make coverage
make ci
```

주요 명령:

| 명령 | 목적 |
|---|---|
| `make fmt` | `gofmt`로 Go source를 format합니다. |
| `make fmt-check` | format되지 않은 Go source가 있으면 실패합니다. |
| `make tidy` | `go mod tidy`를 실행합니다. |
| `make tidy-check` | `go mod tidy` 후 `go.mod`/`go.sum` 변경이 있으면 실패합니다. |
| `make vet` | `go vet ./...`를 실행합니다. |
| `make lint` | `golangci-lint run ./...`를 실행합니다. |
| `make test` | Testcontainers 테스트가 package 단위로 직렬 실행되도록 `go test -p 1 -count=1 ./...`를 실행합니다. |
| `make race` | Testcontainers 테스트가 race detector에서도 package 단위로 직렬 실행되도록 `go test -race -p 1 -count=1 ./...`를 실행합니다. |
| `make coverage` | `coverage/` 아래에 Go coverage profile, package 소계 table, text summary, HTML report를 생성합니다. |
| `make bench-cache` | opt-in cache, Redis NearCache, Redis coordinator benchmark를 실행합니다. |
| `make bench-ratelimit` | opt-in local rate limiter benchmark를 실행합니다. |
| `make bench-id` | opt-in id generator benchmark를 실행합니다. |
| `make ci` | 로컬 CI gate를 실행합니다. |

Redis integration test는 Testcontainers를 사용하므로 Docker가 필요합니다. 일반
CI와 Nightly workflow 모두 실제 container를 사용해 테스트하고 Go coverage
report artifact를 게시합니다.

Fixture 사용법은 [`testing`](testing/README.ko.md),
[`testing/concurrency`](testing/concurrency/README.ko.md), 각 Testcontainers package
README를 참고하세요.

## 프로젝트 관리

- [Changelog](CHANGELOG.md)
- [Current WIP](WIP.md)
- [Research index](docs/research/README.ko.md)
- [Package layout policy](docs/package-layout.md)
- [Release guide](docs/release.md)

## 프로젝트 원칙

- Go에 자연스러운 API를 우선합니다. Kotlin extension 형태를 기계적으로 옮기지
  않습니다.
- catch-all utility package보다, 서비스 코드에서 의미가 분명한 작은 package를
  선호합니다.
- 위험을 낮출 수 있으면 검증된 Go dependency를 사용합니다. 다만 성숙한 SDK를
  bluetape 고유 가치 없이 감싸지 않습니다.
- 인프라 패키지는 Testcontainers 기반 smoke test를 추가합니다.
