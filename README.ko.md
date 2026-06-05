# bluetape-go

[English](README.md) | [한국어](README.ko.md)

![bluetape-go 대표 이미지](docs/assets/bluetape-go-hero.png)

bluetape 생태계를 위한 Go 백엔드 유틸리티와 분산 인프라 패키지입니다.

`bluetape-go`는 Kotlin/JVM 기반 bluetape4k 라이브러리를 대체하는 프로젝트가
아닙니다. Go를 선호하는 백엔드 팀을 위해 서비스 인프라, 분산 조정, 테스트
fixture, resilience, cache, workflow, batch, graph, text, audit, AWS 관련
반복 코드를 Go답게 제공하는 별도 구현입니다.

## 아키텍처

![bluetape-go Architecture Overview](docs/assets/bluetape-go-architecture-overview.png)

## 현재 상태

`bluetape-go`는 현재 `0.3.0` 개발선에 있습니다. Repository에는 foundation
utility, codec, compression, concurrency helper, serialization contract,
Redis 기반 leader election, resilience policy, cache/Redis coordination package,
token-bucket rate limiting이 들어 있습니다. 이 개발선은 candidate registry 기반
pluggable leader election strategy도 포함합니다.

## 패키지

| 패키지 | 상태 | 목적 |
|---|---:|---|
| [`core`](core/README.md) | active | 작은 공용 validation, zero/default, pointer, string, number helper. |
| [`collections`](collections/README.md) | active | chunking, grouping, distinct, error-aware transform용 작은 generic slice/map helper. |
| [`concurrency`](concurrency/README.md) | active | context-aware goroutine group, worker pool, bounded parallel helper. |
| [`codec`](codec/README.md) | active | Base58, Base62, Base64, hex, URL-safe encoding helper. |
| [`compression`](compression/README.md) | active | gzip, deflate, zstd, lz4, snappy, registry 기반 compression helper. |
| [`serialization`](serialization/README.md) | active | 안전한 기본값을 가진 JSON/binary serializer interface. |
| [`testing`](testing/README.md) | active | eventual consistency 테스트용 공용 helper. |
| [`testing/concurrency`](testing/concurrency/README.md) | active | concurrent test를 위한 stress/async job helper. |
| [`testcontainers/redis`](testcontainers/redis/README.md) | active | Testcontainers for Go 기반 Redis fixture. |
| [`testcontainers/postgres`](testcontainers/postgres/README.md) | active | Testcontainers for Go 기반 PostgreSQL fixture. |
| [`testcontainers/mysql`](testcontainers/mysql/README.md) | active | Testcontainers for Go 기반 MySQL 8.4 fixture. |
| [`testcontainers/nats`](testcontainers/nats/README.md) | active | Testcontainers for Go 기반 NATS fixture. |
| [`testcontainers/kafka`](testcontainers/kafka/README.md) | active | Testcontainers for Go 기반 Kafka fixture. |
| [`leader`](leader/README.md) | active | 단일, group, strategy 기반 계약을 포함한 leader election API. |
| [`leader/redis`](leader/redis/README.md) | active | TTL renewal, ZSET slot token, candidate registry 기반 Redis 단일/group/strategic leader election 구현. |
| [`resilience`](resilience/README.md) | active | service call을 위한 자체 composable retry, timeout, circuit breaker, bulkhead policy, synchronous observability hook, `net/http` adapter. |
| [`cache`](cache/README.md) | active | context-aware loader와 same-key stampede protection을 제공하는 generic in-process TTL cache interface. |
| [`cache/redisnear`](cache/redisnear/README.md) | active | process-local loading cache를 위한 Redis Pub/Sub near-cache invalidation. |
| [`cache/rediscoord`](cache/rediscoord/README.md) | active | cold burst 동안 하나의 loader 결과를 process-local cache 사이에서 공유하는 opt-in Redis coordination wrapper. |
| [`lock/redis`](lock/redis/README.md) | active | TTL acquire와 owner-safe Lua unlock을 제공하는 Redis 단일 인스턴스 owner-token lock. |
| [`ratelimit`](ratelimit/README.md) | active | process-local keyed token-bucket limiter와 `net/http` middleware. |
| [`ratelimit/redis`](ratelimit/redis/README.md) | active | atomic Lua consume/refill과 idle key expiration을 쓰는 Redis-backed token-bucket limiter. |

다음 계획 패키지군은 `workflow`, `batch`, `id`, `jwt`, `graph`, `text`,
`audit`, AWS helper/example 패키지입니다.

## 설치

```bash
go get github.com/bluetape4k/bluetape-go
```

## 패키지 문서

상세 사용법, 운영 경계, package별 benchmark는 각 package README에 둡니다.

- Foundation: [`core`](core/README.md), [`collections`](collections/README.md),
  [`concurrency`](concurrency/README.md), [`codec`](codec/README.md),
  [`compression`](compression/README.md), [`serialization`](serialization/README.md).
- Test support: [`testing`](testing/README.md),
  [`testing/concurrency`](testing/concurrency/README.md), 위 표의 Testcontainers
  fixture package README.
- Coordination: [`leader`](leader/README.md),
  [`leader/redis`](leader/redis/README.md), [`lock/redis`](lock/redis/README.md).
- Runtime policy/cache: [`resilience`](resilience/README.md),
  [`cache`](cache/README.md), [`cache/redisnear`](cache/redisnear/README.md),
  [`cache/rediscoord`](cache/rediscoord/README.md),
  [`ratelimit`](ratelimit/README.md).

## Roadmap

| Milestone | 주제 |
|---|---|
| `0.1.0` | Core support, collections, goroutine helper, codec, compression, Redis leader election, Testcontainers. |
| `0.2.0` | Resilience primitive: retry, timeout, circuit breaker, bulkhead, HTTP middleware. |
| `0.3.0` | Cache/coordination: near cache, Redis lock, token-bucket rate limiting, strategic leader election. |
| `0.4.0` | State machine과 lightweight workflow primitive. |
| `0.5.0` | Checkpoint 기반 batch processing과 leader-guarded example. |
| `0.6.0` | ID generation, JWT, measured value, money, probabilistic structure, rule engine. |
| `0.7.0` | 큰 도메인에 대한 research gate. |
| `0.8.0` | Graph package와 example. |
| `0.9.0` | Text search, blockword masking, tokenizer research. |
| `0.10.0` | bluetape4k-javers 패턴 기반 audit/event package. |
| `0.11.0` | AWS helper package와 LocalStack example. |

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
| `make test` | Testcontainers 테스트가 실제 실행되도록 `go test -count=1 ./...`를 실행합니다. |
| `make race` | Testcontainers 테스트가 race detector에서도 실제 실행되도록 `go test -race -count=1 ./...`를 실행합니다. |
| `make coverage` | `coverage/` 아래에 Go coverage profile, package 소계 table, text summary, HTML report를 생성합니다. |
| `make bench-cache` | opt-in cache, Redis NearCache, Redis coordinator benchmark를 실행합니다. |
| `make bench-ratelimit` | opt-in local rate limiter benchmark를 실행합니다. |
| `make ci` | 로컬 CI gate를 실행합니다. |

Redis integration test는 Testcontainers를 사용하므로 Docker가 필요합니다. 일반
CI와 Nightly workflow 모두 실제 container를 사용해 테스트하고 Go coverage
report artifact를 게시합니다.

Fixture 사용법은 [`testing`](testing/README.md),
[`testing/concurrency`](testing/concurrency/README.md), 각 Testcontainers package
README를 참고하세요.

## 프로젝트 관리

- [Changelog](CHANGELOG.md)
- [Current WIP](WIP.md)
- [Research index](docs/research/README.md)
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
