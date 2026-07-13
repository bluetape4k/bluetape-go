# ratelimit

[English](README.md) | [한국어](README.ko.md)

`ratelimit`는 process-local keyed token-bucket limiter와 standard-library HTTP
middleware를 제공합니다. In-process request guard, tenant throttle, deterministic
rejection diagnostic이 필요한 test에 적합합니다.

소유권과 traffic 특성에 맞춰 provider를 선택합니다.

| Provider | 적합한 경우 | 운영 경계 |
|---|---|---|
| Local `ratelimit` | 한 process가 quota를 소유합니다. | 빠른 in-memory state이며 process 사이에 공유되지 않습니다. |
| [`ratelimit/redis`](redis/README.ko.md) | 여러 process가 낮은 latency의 quota를 공유해야 합니다. | Caller-owned Redis와 atomic Lua operation을 사용합니다. |
| [`ratelimit/sql`](sql/README.ko.md) | PostgreSQL을 이미 공유하는 moderate-QPS, database-only 배포입니다. | Caller-owned PostgreSQL schema, pool, cleanup을 사용하며 high-QPS에서 Redis를 대체하지 않습니다. |

## 다이어그램

![ratelimit local runtime flow](../docs/images/readme-diagrams/ratelimit-local-runtime-flow.png)

## 설치

```go
import "github.com/bluetape4k/bluetape-go/ratelimit"
```

## Local Token Bucket

```go
limiter, err := ratelimit.New(ratelimit.Options{
    RatePerSecond: 10,
    Burst:         20,
})
if err != nil {
    return err
}

result, err := limiter.Allow(ctx, "tenant:blue", 1)
if err != nil {
    return err
}
if !result.Allowed {
    return fmt.Errorf("retry after %s", result.RetryAfter)
}
```

Rejected attempt는 error가 아니라 정상 result입니다. Error는 invalid input,
context cancellation, `ratelimit/redis`나 `ratelimit/sql` 같은 backend
implementation failure에 사용합니다.

## HTTP Middleware

```go
handler, err := ratelimit.NewHandler(next, ratelimit.HandlerOptions{
    Limiter: limiter,
    KeyFunc: func(r *http.Request) string {
        return authenticatedTenantID(r)
    },
})
```

Default key function은 `Request.RemoteAddr`만 사용합니다. `X-Forwarded-For`,
`Forwarded` 또는 다른 proxy header를 신뢰하지 않습니다. Trusted proxy 뒤의
service는 authenticated tenant, user, API-key identity 기반의 명시적 `KeyFunc`를
제공해야 합니다.

Default middleware behavior:

- allowed attempt는 wrapped handler로 위임합니다.
- rejected attempt는 `429 Too Many Requests`를 반환합니다.
- retry delay를 알 수 있으면 rejected attempt에 `Retry-After`를 설정합니다.
- backend/key error는 `503 Service Unavailable`을 반환합니다.
- `ErrorHandler`는 response policy를 교체할 수 있습니다.

## 운영 경계

- State는 process-local이며 memory에 보관됩니다.
- `IdleTTL`은 inactive key state를 제거합니다. 기본값은 최소 1분이며 최소 두 번의
  full refill window 이상입니다.
- `Burst`보다 많은 token 요청은 bucket이 절대 만족할 수 없으므로 validation error입니다.
- Limiter는 concurrency-safe이지만 FIFO fairness를 제공하지 않습니다.

## 테스트

```bash
go test -count=1 ./ratelimit
go test -race -count=1 ./ratelimit
```

Stress와 cancellation coverage는 `testing/concurrency.GoroutineStressTester`와
`testing/concurrency.AsyncJobTester`를 사용합니다.

## 벤치마크

```bash
make bench-ratelimit
```

측정 경로:

- local allowed path;
- local rejected path;
- HTTP middleware allowed path.

낮은 `ns/op`와 낮은 allocation이 더 좋습니다. Redis-backed benchmark scope는
external Redis latency와 deployment topology에 의존하므로 별도로 유지합니다.

## 벤치마크 스냅샷

아래 수치는 local smoke number이며 production capacity ranking이 아닙니다. 실행
환경은 macOS arm64 Apple M4 Pro입니다. 낮은 `ns/op`, `B/op`, `allocs/op`가 더
좋습니다.

![ratelimit benchmark latency](../docs/images/readme-charts/ratelimit-benchmark-latency.png)

| Benchmark | ns/op | B/op | allocs/op |
|---|---:|---:|---:|
| `BenchmarkTokenBucketAllowAllowed` | 116.4 | 0 | 0 |
| `BenchmarkTokenBucketAllowRejected` | 76.76 | 0 | 0 |
| `BenchmarkHandlerAllowed` | 51.26 | 160 | 3 |

## Provider Conformance

`ratelimit/ratelimittest.Run`은 local, Redis, SQL provider에 같은 burst, refill,
cancellation, exact-admission contract를 적용합니다. Distributed provider의 redacted
failure는 `ratelimit.OperationError`로 확인합니다.
`errors.Is(err, ratelimit.ErrCommitUnknown)`이면 한 번 debit됐을 수 있으므로 zero result를
버리고 자동 replay하지 않습니다.

Local, Redis, SQL 사이에 quota state is not shared라는 경계가 있습니다. Provider를
동시에 섞으면 각각이 full burst를 허용해 multiple full bursts가 생기므로 금지합니다.
안전한 canary는 independent namespace와 independent cohort를 사용합니다. Cutover 또는
rollback에서는 old provider를 quiesce하고 보수적인 full-refill window를 기다린 뒤
정확히 하나의 새 provider를 활성화합니다. 겹치는 구간이 필요하면
approved extra-burst budget을 사전에 기록합니다.
