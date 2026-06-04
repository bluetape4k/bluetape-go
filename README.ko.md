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
Redis 기반 leader election, resilience policy, 첫 cache contract가 들어
있습니다. 남은 `0.3.0` 작업은 Redis near-cache invalidation, distributed lock,
token-bucket rate limiting, cache benchmark에 집중합니다.

## 패키지

| 패키지 | 상태 | 목적 |
|---|---:|---|
| `core` | active | 작은 공용 validation, zero/default, pointer, string, number helper. |
| `collections` | active | chunking, grouping, distinct, error-aware transform용 작은 generic slice/map helper. |
| `concurrency` | active | context-aware goroutine group, worker pool, bounded parallel helper. |
| `codec` | active | Base58, Base62, Base64, hex, URL-safe encoding helper. |
| `compression` | active | gzip, deflate, zstd, lz4, snappy, registry 기반 compression helper. |
| `serialization` | active | 안전한 기본값을 가진 JSON/binary serializer interface. |
| `testing` | active | eventual consistency 테스트용 공용 helper. |
| `testcontainers/redis` | active | Testcontainers for Go 기반 Redis fixture. |
| `testcontainers/postgres` | active | Testcontainers for Go 기반 PostgreSQL fixture. |
| `testcontainers/mysql` | active | Testcontainers for Go 기반 MySQL 8.4 fixture. |
| `testcontainers/nats` | active | Testcontainers for Go 기반 NATS fixture. |
| `testcontainers/kafka` | active | Testcontainers for Go 기반 Kafka fixture. |
| `leader` | active | Leader election API. |
| `leader/redis` | active | TTL renewal과 ZSET slot token 기반 Redis 단일/group leader election 구현. |
| `resilience` | active | service call을 위한 자체 composable retry, timeout, circuit breaker, bulkhead policy, synchronous observability hook, `net/http` adapter. |
| `cache` | initial | context-aware loader와 same-key stampede protection을 제공하는 generic in-process TTL cache interface. |

다음 계획 패키지군은 `workflow`, `batch`, `id`, `jwt`, `graph`, `text`,
`audit`, AWS helper/example 패키지입니다.

## 설치

```bash
go get github.com/bluetape4k/bluetape-go
```

## Leader Election

```go
client := redis.NewClient(&redis.Options{Addr: "localhost:6379"})

elector, err := redisleader.New(client, leader.Options{
    Group:    "billing-workers",
    MemberID: "worker-1",
})
if err != nil {
    return err
}

if err := elector.Campaign(ctx); err != nil {
    return err
}
defer elector.Resign(context.Background())
```

Kotlin/JVM `bluetape4k-leader` repository는 별도로 계속 유지합니다. Go 구현과
Kotlin 구현을 같은 Redis leader 참가자로 섞는 방식은 `0.1.0`에서 지원하지
않습니다. Go Redis backend는 `bluetape:leader:<group>` key에
`memberID:random` token을 TTL과 함께 저장하는 자체 key 형식을 사용합니다.
Kotlin/JVM `bluetape4k-leader`의 Lettuce backend는 lock name을 직접 key로 쓰고,
Redisson backend는 Redisson `RLock` 내부 구조를 사용합니다. 명시적인 interop
adapter를 추가하기 전까지 Kotlin과 Go leader group은 분리해서 운영합니다.

Redis leader 예제는 backend replica 중 하나만 실행해야 하는 조정 문제를 다룹니다.

| 예제 | 문제 | Smoke test |
|---|---|---|
| Batch scheduler | 모든 scheduler replica가 같은 nightly job을 실행하지 않게 합니다. | `go test -count=1 ./leader/redis -run TestBatchSchedulerExample` |
| Migration gate | 배포 중 하나의 service instance만 migration을 적용하게 합니다. | `go test -count=1 ./leader/redis -run TestMigrationGateExample` |

동시에 제한된 수의 replica가 같은 worker lane을 실행해도 된다면 `NewGroup`을
사용합니다.

```go
group, err := redisleader.NewGroup(client, leader.GroupOptions{
    Options: leader.Options{
        Group:    "batch-workers",
        MemberID: "worker-1",
    },
    MaxLeaders: 3,
})
if err != nil {
    return err
}

if err := group.Campaign(ctx); err != nil {
    return err
}
defer group.Resign(context.Background())
```

Redis group backend는 살아 있는 slot을 `bluetape:leader-group:<group>` ZSET에
저장합니다. 각 member는 Redis server time 기준 만료 score를 가진
`memberID:random` token입니다. 만료된 slot은 acquire와 status check 중 정리되므로,
process crash로 누수된 slot도 별도 reaper 없이 회수됩니다.

## Resilience Policy

Resilience policy는 service call 주변에 조합할 수 있는 retry, timeout, circuit
breaker, bulkhead primitive를 제공합니다. 각 policy는 `OnEvent` hook을 받을 수
있으며, 이 hook은 보호 대상 call path에서 동기적으로 호출됩니다. Hook payload인
`resilience.Event`에는 안정적인 policy type, event kind, event category,
attempt/state data, 낮은 cardinality의 error category label이 들어갑니다.

`OnEvent`는 service가 이미 사용하는 logging, metrics, tracing 도구로 policy
결정을 전달하는 얇은 bridge로 사용합니다.

```go
retry, err := resilience.NewRetry[string](resilience.RetryOptions{
    Name:        "catalog",
    MaxAttempts: 3,
    Backoff:     resilience.ConstantBackoff(50 * time.Millisecond),
    OnEvent: func(ctx context.Context, event resilience.Event) {
        logger.InfoContext(ctx, "resilience event",
            "policy", event.PolicyName,
            "type", event.PolicyType,
            "kind", event.Kind,
            "category", event.Category,
            "error_category", event.ErrorCategory,
            "attempt", event.Attempt,
        )
    },
})
```

Package는 OpenTelemetry exporter를 내장하지 않습니다. Event handler는 보호 대상
call을 지연시키지 않도록 빠르고 non-blocking하게 작성해야 합니다.

HTTP client는 `net/http` transport adapter로 같은 policy를 사용할 수 있습니다.
Adapter는 retry 가능한 response status를 `StatusError`로 바꾸고, 다음 시도 전에
해당 response body를 닫으며, observability는 같은 `OnEvent` hook contract를
사용합니다.

```go
retry, err := resilience.NewRetry[*http.Response](resilience.RetryOptions{
    Name:        "catalog-http",
    MaxAttempts: 3,
    Backoff:     resilience.ConstantBackoff(50 * time.Millisecond),
    OnEvent:     onResilienceEvent,
})
if err != nil {
    return err
}
timeout, err := resilience.NewTimeout[*http.Response](resilience.TimeoutOptions{
    Name:    "catalog-http",
    Timeout: 500 * time.Millisecond,
    OnEvent: onResilienceEvent,
})
if err != nil {
    return err
}
breaker, err := resilience.NewCircuitBreaker[*http.Response](resilience.CircuitBreakerOptions{
    Name:             "catalog-http",
    FailureThreshold: 5,
    OpenTimeout:      30 * time.Second,
    OnEvent:          onResilienceEvent,
})
if err != nil {
    return err
}

client := http.Client{
    Transport: resilience.NewRoundTripper(resilience.RoundTripperOptions{
        Transport:       http.DefaultTransport,
        Policies:        []resilience.Policy[*http.Response]{retry, timeout, breaker},
        RetryableStatus: resilience.RetryableServerError,
    }),
}
```

Server handler는 `NewHandler`로 admission 또는 timeout policy를 적용할 수 있습니다.
Response를 이미 쓴 server handler를 retry하지 말고, request body를 replay할 수
있는 outbound client call 쪽에 retry를 적용하는 방식을 우선합니다.

## Cache

`cache` 패키지는 framework-neutral cache 계약과 process-local memory 구현을
제공합니다. `ErrCacheMiss`는 없는 값이나 만료된 값을 구분하고, TTL `0`은 만료
없는 저장을 의미합니다. `GetOrLoad`는 한 cache instance 안에서 같은 key의
동시 loader 호출을 하나의 in-flight 실행으로 합칩니다.

```go
localCache := cache.NewMemory[string, string]()

value, err := localCache.GetOrLoad(ctx, "catalog", time.Minute,
    func(ctx context.Context, key string) (string, error) {
        return loadCatalogValue(ctx, key)
    },
)
if err != nil {
    return err
}
fmt.Println(value)
```

`Delete`와 `Clear`는 동시 호출에 안전하지만 이미 실행 중인 loader를 취소하지는
않습니다. 실행 중인 loader가 나중에 성공하면 일반적인 cache-aside 순서에 따라
cache를 다시 채울 수 있습니다. Redis near-cache invalidation과 cross-process
stampede protection은 이후 0.3.0 작업에서 다룹니다.

## Roadmap

| Milestone | 주제 |
|---|---|
| `0.1.0` | Core support, collections, goroutine helper, codec, compression, Redis leader election, Testcontainers. |
| `0.2.0` | Resilience primitive: retry, timeout, circuit breaker, bulkhead, HTTP middleware. |
| `0.3.0` | Cache/coordination: near cache, Redis lock, token-bucket rate limiting. |
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
| `make ci` | 로컬 CI gate를 실행합니다. |

Redis integration test는 Testcontainers를 사용하므로 Docker가 필요합니다. 일반
CI와 Nightly workflow 모두 실제 container를 사용해 테스트합니다.

비동기 테스트 assertion은 Gomega 기반 `testing` helper를 사용합니다.

```go
bttesting.Eventually(t, time.Second, func() bool {
    return elector.IsLeader()
})

bttesting.Consistently(t, 200*time.Millisecond, elector.IsLeader)
```

Testcontainers fixture는 작은 `Start(ctx, t)` helper로 제공되며, `t.Cleanup`에
정리를 등록하고 service connection 정보를 반환합니다.

```go
redisAddr := redistestcontainer.Start(ctx, t)
postgresURL := postgrestestcontainer.Start(ctx, t)
mysqlDSN := mysqltestcontainer.Start(ctx, t)
natsURL := natstestcontainer.Start(ctx, t)
kafkaBrokers := kafkatestcontainer.Start(ctx, t)
```

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
