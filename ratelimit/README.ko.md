# ratelimit

[English](README.md) | [한국어](README.ko.md)

`ratelimit`는 process-local keyed token-bucket limiter와 standard-library HTTP
middleware를 제공합니다. In-process request guard, tenant throttle, deterministic
rejection diagnostic이 필요한 test에 적합합니다.

여러 process가 하나의 bucket을 공유해야 하면 [`ratelimit/redis`](redis/README.ko.md)를 사용하세요.

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
context cancellation, `ratelimit/redis` 같은 backend implementation failure에
사용합니다.

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
