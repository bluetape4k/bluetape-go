# bluetape-go

[English](README.md) | [한국어](README.ko.md)

![bluetape-go 대표 이미지](docs/assets/bluetape-go-hero.png)

bluetape 생태계를 위한 Go 백엔드 유틸리티와 분산 인프라 패키지입니다.

`bluetape-go`는 Kotlin/JVM 기반 bluetape4k 라이브러리를 대체하는 프로젝트가
아닙니다. Go를 선호하는 백엔드 팀을 위해 서비스 인프라, 분산 조정, 테스트
fixture, resilience, cache, workflow, batch, graph, text, audit, AWS 관련
반복 코드를 Go답게 제공하는 별도 구현입니다.

## 현재 상태

`bluetape-go`는 초기 `0.1.0` 개발 단계입니다. 현재 repository에는 첫 foundation
패키지와 Redis 기반 leader election 구현이 들어 있으며, 이후 범위는 milestone과
research 문서로 추적합니다.

## 패키지

| 패키지 | 상태 | 목적 |
|---|---:|---|
| `core` | initial | 작은 공용 validation/support helper. |
| `testing` | initial | eventual consistency 테스트용 공용 helper. |
| `testcontainers/redis` | initial | Testcontainers for Go 기반 Redis fixture. |
| `leader` | initial | Leader election API. |
| `leader/redis` | initial | Redis `SET NX PX`와 TTL renewal 기반 leader election 구현. |

계획 중인 패키지군은 `collections`, `concurrency`, `serialization`,
`resilience`, `cache`, `workflow`, `batch`, `id`, `jwt`, `graph`, `text`,
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
Kotlin 구현의 Redis key 호환 여부는 첫 stable tag 전에 명시적으로 결정합니다.

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
go test ./...
```

Redis integration test는 Testcontainers를 사용하므로 Docker가 필요합니다.

## 프로젝트 원칙

- Go에 자연스러운 API를 우선합니다. Kotlin extension 형태를 기계적으로 옮기지
  않습니다.
- catch-all utility package보다, 서비스 코드에서 의미가 분명한 작은 package를
  선호합니다.
- 위험을 낮출 수 있으면 검증된 Go dependency를 사용합니다. 다만 성숙한 SDK를
  bluetape 고유 가치 없이 감싸지 않습니다.
- 인프라 패키지는 Testcontainers 기반 smoke test를 추가합니다.

