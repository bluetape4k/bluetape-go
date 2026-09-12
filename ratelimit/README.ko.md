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

## 벤치마크 측정 결과

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

### 전환·롤백 runbook

1. 이전·신규 provider, 정책, namespace, cohort와 롤백 대상을 기록합니다.
   namespace가 달라도 사용자는 겹칠 수 있으므로 각 cohort를 정확히 하나의
   provider로 라우팅합니다. local 객체를 새로 만들어도 full bucket이 생깁니다.
2. 이전 경로의 신규 요청을 차단하고 대기 요청을 취소합니다. 진행 중인 모든
   `Allow` 호출과 이미 허용한 애플리케이션 작업이 종료될 때까지 기다립니다.
   취소만으로 이미 전송한 debit의 중단을 증명할 수는 없습니다.
3. `ErrCommitUnknown`이면 요청 token이 차감됐을 수 있다고 계산합니다.
   자동 replay, 다른 provider로의 fallback, 환불을 하지 않습니다. Redis Lua
   debit은 멱등적이지 않습니다. 이 limiter에는 호출자가 소유한 전용 Redis
   standalone client를 사용하고 `MaxRetries: -1`을 설정해야 합니다.
   go-redis 기본 설정은 transport 오류를 재시도하므로 adapter에 반환되기 전에
   다시 차감할 수 있습니다. `MaxRetries: 0`은 비활성화가 아니라 기본 설정을
   선택합니다. `Allow`를 재시도하는 middleware도 두지 않습니다.
   애플리케이션 요청 ID만으로 debit 중복을 제거할 수는 없습니다.
   Cluster client에는 별도의 `MaxRedirects` 재시도 반복이 있어 `MaxRetries`만
   비활성화해서는 충분하지 않습니다. `MaxRedirects: -1`도 설정하고 라우팅 오류가
   나도 무조건 debit을 재실행하지 않습니다. 아래 standalone fixture는 cluster
   transport 동작을 검증하지 않습니다.
4. 추가 burst 예산을 승인하지 않았다면 drain 이후 마지막으로 차감됐을 수 있는
   시점부터 이전·신규 full-refill 시간(`Burst / RatePerSecond`) 중 큰 값 이상을
   올림해서 기다리고 운영 여유 시간을 더합니다. key 삭제, TTL 만료, 프로세스
   재시작으로 quota를 초기화하지 않습니다. 이 대기는 운영 전환 규칙이며 quota
   이전이나 provider 전체를 묶는 전역 제한을 구현하지는 않습니다.
5. 해당 cohort의 신규 경로를 활성화합니다. 겹치는 운영을 명시적으로 승인했다면
   종료 시각, cohort, 최대 추가 burst, 지속 refill 허용량, 관측 지표와 중단 조건을
   기록합니다. 새 bucket 두 개는 burst의 합만큼 허용할 수 있고, 종료 시각이 없는
   중복 운영은 고정된 추가 burst 상한으로 통제할 수 없습니다.
6. 롤백에도 같은 drain·대기 절차를 적용합니다. 가능하면 이전 namespace와 local
   객체를 보존합니다. 새로 생성하면 quota가 다시 생깁니다. 롤백 전 commit 결과가
   불명확했던 요청을 다시 실행하지 않습니다.

허용·거절 합계, `ErrCommitUnknown`, 진행 중인 호출 수, drain 시간, 활성 라우팅
세대와 중복 운영 예산 소비량을 관측합니다. 고정된 provider·operation label과
명시적인 observer를 사용하고 사용자 key, namespace, backend 진단 원문을 메트릭
label이나 로그에 넣지 않습니다. 같은 cohort가 의도치 않게 두 경로로 유입되거나
불명확한 결과가 늘거나 승인된 예산이 소진되면 전환을 중단합니다.

실제 provider 테스트는 이전·신규 조합 9개, 사전 취소, 별도 cohort, 이전 상태
보존과 Lua debit 성공 후 Redis 응답 유실을 검증합니다. 실제 응답을 읽은 뒤
`io.EOF`를 주입하는 transport fixture에서 재시도 비활성화 시 1회 차감, 기본
client 설정 대조군에서 2회 차감을 확인합니다. 라우팅 controller를 배포하거나
호출자의 drain 절차를 증명하는 테스트는 아닙니다. 실제 전환 전에는 drain 완료,
계산한 refill 대기 시간과 실제 경과 시간, 중복 운영 예산 만료, 같은 cohort의
롤백 예행연습 결과를 수동 수용 기준으로 기록해야 합니다. Docker 테스트는
패키지별로 직렬 실행합니다.

```bash
go test -p 1 -count=1 -run '^TestProviderCutover' ./ratelimit/sql
go test -race -p 1 -count=1 -run '^TestProviderCutover' ./ratelimit/sql
```
