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

## 현재 상태

`bluetape-go`는 `v0.8.0` 릴리스 선을 배포했습니다. 현재 repository에는
foundation helper, codec, compression, context-aware concurrency, serializer
contract, Redis 기반 leader election과 lock, resilience policy, cache
coordination, token-bucket rate limiting, finite state machine, workflow report,
lightweight workflow runner, checkpoint 기반 batch job, portable service value,
SQL helper, text search primitive가 들어 있습니다.

`v0.6.x` portable utility 범위에는 UUID, ULID, KSUID, Snowflake ID 생성,
명시적 algorithm 기반 JWT signing/parsing/validation, 인메모리/Redis/MongoDB
KeyChain repository 기반 distributed key rotation, typed unit과 measured value,
ISO currency와 decimal-backed money 연산, 인메모리 Bloom 또는 Redis-backed Bloom
filter가 포함됩니다. 현재 `0.6.7` 선에는 corrective `0.6.3`부터 `0.6.6` 구현
series와 MongoDB-backed JWT KeyChain storage가 포함됩니다.

## 패키지

| 패키지 | 상태 | 목적 |
|---|---:|---|
| [`core`](core/README.ko.md) | active | 작은 공용 validation, zero/default, pointer, string, number helper. |
| [`collections`](collections/README.ko.md) | active | chunking, grouping, distinct, error-aware transform용 작은 generic slice/map helper. |
| [`concurrency`](concurrency/README.ko.md) | active | context-aware goroutine group, worker pool, bounded parallel helper. |
| [`codec`](codec/README.ko.md) | active | Base58, Base62, Base64, hex, URL-safe encoding helper. |
| [`compression`](compression/README.ko.md) | active | gzip, deflate, zstd, lz4, snappy, registry 기반 compression helper. |
| [`serialization`](serialization/README.ko.md) | active | 안전한 기본값을 가진 JSON/binary serializer interface. |
| [`testing`](testing/README.ko.md) | active | eventual consistency 테스트용 공용 helper. |
| [`testing/concurrency`](testing/concurrency/README.ko.md) | active | concurrent test를 위한 stress/async job helper. |
| [`testcontainers/redis`](testcontainers/redis/README.ko.md) | active | Testcontainers for Go 기반 Redis fixture. |
| [`testcontainers/postgres`](testcontainers/postgres/README.ko.md) | active | Testcontainers for Go 기반 PostgreSQL fixture. |
| [`testcontainers/mysql`](testcontainers/mysql/README.ko.md) | active | Testcontainers for Go 기반 MySQL 8.4 fixture. |
| [`testcontainers/nats`](testcontainers/nats/README.ko.md) | active | Testcontainers for Go 기반 NATS fixture. |
| [`testcontainers/kafka`](testcontainers/kafka/README.ko.md) | active | Testcontainers for Go 기반 Kafka fixture. |
| [`dynamodb/batchwrite`](dynamodb/batchwrite/README.ko.md) | active | AWS SDK for Go v2 BatchWriteItem 25개 chunking과 unprocessed-item retry helper. |
| [`examples/integration`](examples/integration/README.ko.md) | example | 수정된 `0.6.x` package를 묶는 compile-checked end-to-end recipe. |
| [`examples/audit`](examples/audit/README.ko.md) | example | Repository history와 outbox replay boundary를 보여주는 runnable audit-backed order service. |
| [`examples/s3`](examples/s3/README.ko.md) | example | Floci fixture 기반 compile-checked AWS SDK for Go v2 S3 예제. |
| [`examples/sqs-sns`](examples/sqs-sns/README.ko.md) | example | Floci fixture 기반 compile-checked AWS SDK for Go v2 SQS/SNS 예제. |
| [`leader`](leader/README.ko.md) | active | 단일, group, strategy 기반 계약을 포함한 leader election API. |
| [`leader/redis`](leader/redis/README.ko.md) | active | TTL renewal, ZSET slot token, candidate registry 기반 Redis 단일/group/strategic leader election 구현. |
| [`resilience`](resilience/README.ko.md) | active | service call을 위한 자체 composable retry, timeout, circuit breaker, bulkhead policy, synchronous observability hook, `net/http` adapter. |
| [`cache`](cache/README.ko.md) | active | context-aware loader와 same-key stampede protection을 제공하는 generic in-process TTL cache interface. |
| [`cache/redisnear`](cache/redisnear/README.ko.md) | active | process-local loading cache를 위한 Redis Pub/Sub near-cache invalidation. |
| [`cache/rediscoord`](cache/rediscoord/README.ko.md) | active | cold burst 동안 하나의 loader 결과를 process-local cache 사이에서 공유하는 opt-in Redis coordination wrapper. |
| [`lock/redis`](lock/redis/README.ko.md) | active | TTL acquire와 owner-safe Lua unlock을 제공하는 Redis 단일 인스턴스 owner-token lock. |
| [`ratelimit`](ratelimit/README.ko.md) | active | process-local keyed token-bucket limiter와 `net/http` middleware. |
| [`ratelimit/redis`](ratelimit/redis/README.ko.md) | active | atomic Lua consume/refill과 idle key expiration을 쓰는 Redis-backed token-bucket limiter. |
| [`state`](state/README.ko.md) | active | typed transition, guard, final state, sentinel error를 제공하는 작은 finite state machine primitive. |
| [`workreport`](workreport/README.ko.md) | active | lightweight workflow code를 위한 status, failure-policy, report-tree value. |
| [`workflow`](workflow/README.ko.md) | active | `context.Context`와 `workreport` 기반 sequential, conditional, all-branches parallel runner. |
| [`batch`](batch/README.ko.md) | active | Chunk-oriented batch step, sequential job, retry/skip policy, report, checkpoint. |
| [`id`](id/README.ko.md) | active | UUID v4/v7, random/monotonic ULID, standard KSUID, Kotlin-compatible KSUID millis, Snowflake ID generator. |
| [`jwt`](jwt/README.ko.md) | active | 명시적 algorithm을 사용하는 JWT signing, parsing, validation, typed claim reading, in-memory/distributed `kid` key rotation, optional provider cache adapter. |
| [`jwt/redis`](jwt/redis/README.ko.md) | active | Distributed JWT key-chain repository 생성을 위한 Redis 전용 facade. |
| [`jwt/mongo`](jwt/mongo/README.ko.md) | active | Distributed JWT key-chain repository 생성을 위한 MongoDB 전용 facade. |
| [`measure`](measure/README.ko.md) | active | Typed unit, measured value, compound unit, parsing, formatting, affine temperature helper. |
| [`money`](money/README.ko.md) | active | ISO 4217 통화 값, CLDR-backed locale currency lookup, decimal-backed 금액, 합산, 직렬화, caller-supplied 환율 변환, ECB-backed provider 변환. |
| [`sqlkit`](sqlkit/README.ko.md) | active | Runtime-first `database/sql` transaction helper, 명시적 row mapping/cardinality helper, PostgreSQL 우선 inspectable SQL builder. |
| [`audit`](audit/README.ko.md) | active | validated JSON entry, pending event recording, history reconstruction을 제공하는 storage-neutral aggregate event/audit model. |
| [`audit/sqloutbox`](audit/sqloutbox/README.ko.md) | active | Caller-owned transaction choreography를 유지하는 PostgreSQL-backed audit outbox store와 relay. |
| [`probabilistic`](probabilistic/README.ko.md) | active | deterministic config, merge compatibility check, stress/race coverage를 갖춘 goroutine-safe 인메모리 Bloom filter. |
| [`probabilistic/redis`](probabilistic/redis/README.ko.md) | active | Static Lua script, immutable config metadata, operator runbook 경계를 갖춘 Redis-backed shared Bloom filter. |

다음 계획 패키지군은 audit publisher adapter, example service, graph package입니다.
Redis-backed Cuckoo와 HyperLogLog/HLL 지원은 Redis Bloom 범위 이후 별도로
추적합니다.

## 설치

```bash
go get github.com/bluetape4k/bluetape-go
```

## 패키지 문서

각 package README에는 실제 사용 예제, 운영 경계, benchmark 메모, root 개요에
넣기에는 너무 구체적인 제약을 정리합니다.

- Foundation: [`core`](core/README.ko.md), [`collections`](collections/README.ko.md),
  [`concurrency`](concurrency/README.ko.md), [`codec`](codec/README.ko.md),
  [`compression`](compression/README.ko.md), [`serialization`](serialization/README.ko.md).
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
- Coordination: [`leader`](leader/README.ko.md),
  [`leader/redis`](leader/redis/README.ko.md), [`lock/redis`](lock/redis/README.ko.md).
- Runtime policy/cache/state/workflow: [`resilience`](resilience/README.ko.md),
  [`cache`](cache/README.ko.md), [`cache/redisnear`](cache/redisnear/README.ko.md),
  [`cache/rediscoord`](cache/rediscoord/README.ko.md), [`ratelimit`](ratelimit/README.ko.md),
  [`state`](state/README.ko.md), [`workreport`](workreport/README.ko.md),
  [`workflow`](workflow/README.ko.md), [`batch`](batch/README.ko.md).
- Portable utility: [`id`](id/README.ko.md), [`jwt`](jwt/README.ko.md),
  [`jwt/redis`](jwt/redis/README.ko.md), [`jwt/mongo`](jwt/mongo/README.ko.md),
  [`measure`](measure/README.ko.md), [`money`](money/README.ko.md),
  [`probabilistic`](probabilistic/README.ko.md) 및
  [`probabilistic/redis`](probabilistic/redis/README.ko.md).
- Data access: [`sqlkit`](sqlkit/README.ko.md) 및 optional
  [SQL generator/migration guide](docs/sql-generator-migration-guidance.ko.md).
- Audit: storage-neutral aggregate event value, pending event handoff, validated
  audit entry JSON, history reconstruction을 위한 [`audit`](audit/README.ko.md)와
  PostgreSQL-backed at-least-once outbox delivery를 위한
  [`audit/sqloutbox`](audit/sqloutbox/README.ko.md), runnable audit-backed order
  service인 [`examples/audit`](examples/audit/README.ko.md).

### Audit Example 한눈에 보기

![Audit Example Service Flow](docs/images/readme-diagrams/audit-example-service-flow.png)

Audit 예제는 일부러 작게 만들었습니다. 현재 상태를 가진 source model과 변경 이력을
담는 audit history를 분리합니다. Command는 `audit.Repository`를 통해
`audit.Entry`를 append하고, append가 성공한 뒤에만 예제 order state를 바꿉니다.
History query도 같은 repository boundary를 읽습니다. Outbox replay는 최소
`EntrySink`만 사용하므로 운영 code에서는 in-memory fixture 대신 `audit/sqloutbox`를
연결하면 됩니다. 예제를 framework로 키우지 않고 boundary만 드러내려는 의도입니다.

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
| `0.11.0` | Image, encryption, utility follow-up. |

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
