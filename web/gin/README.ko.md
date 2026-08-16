# web/gin

[English](README.md) | [한국어](README.ko.md)

`web/gin`은 framework-neutral `web`, `ratelimit`, `jwt`, `resilience` 계약을 Gin에
연결하는 얇은 adapter입니다. Gin 의존성은 이 package에만 격리하고 core package는
`net/http`와 다른 adapter에서 계속 사용할 수 있게 합니다.

## 설치와 import

```go
import ginadapter "github.com/bluetape4k/bluetape-go/web/gin"
```

이 adapter는 Gin `v1.12.0`을 사용합니다. Core package에서는 Gin을 import하지
않습니다.

## Bootstrap

[`example_test.go`](example_test.go)의 `ExampleBootstrap`과 `ExampleMigration`은
compile-checked fixture입니다. 일반적인 조합 순서는 recovery, request context,
rate limit, authentication, route-level resilience wrapper입니다.

```go
import (
    "net/http"

    "github.com/bluetape4k/bluetape-go/jwt"
    "github.com/bluetape4k/bluetape-go/ratelimit"
    "github.com/bluetape4k/bluetape-go/web"
    ginadapter "github.com/bluetape4k/bluetape-go/web/gin"
    "github.com/gin-gonic/gin"
)

func buildRouter() (*gin.Engine, error) {
    provider, err := jwt.NewFixedHMACProvider(jwt.HS256, []byte("0123456789abcdef0123456789abcdef"))
    if err != nil { return nil, err }
    limiter, err := ratelimit.New(ratelimit.Options{RatePerSecond: 10, Burst: 10})
    if err != nil { return nil, err }
    rateLimit, err := ginadapter.NewRateLimit(ginadapter.RateLimitOptions{Limiter: limiter})
    if err != nil { return nil, err }
    authentication, err := ginadapter.NewJWT(ginadapter.JWTOptions{Parser: provider})
    if err != nil { return nil, err }
    router := gin.New()
    router.Use(gin.Recovery(), ginadapter.RequestContext(web.RequestContextOptions{}), rateLimit, authentication)
    handler := func(c *gin.Context) { c.Status(http.StatusNoContent) }
    router.GET("/orders", ginadapter.WrapResilience(handler, ginadapter.ResilienceOptions{}))
    return router, nil
}
```

`RequestContext`는 공유 `web.RequestContext`를 request 복사본에 저장하고 정상
반환과 panic 뒤에 원래 request pointer를 복원합니다. 제한 header는 caller가
제공한 `TrustedProxy` predicate가 true일 때만 읽습니다.

## Middleware 계약

- `AbortWithProblem`은 RFC 9457 `application/problem+json`을 기록하고 Gin chain을
  중단합니다. 알 수 없는 오류는 redaction하며 이미 기록된 writer를 덮어쓰지
  않습니다.
- `NewRateLimit`은 기존 limiter를 bridge합니다. 허용 요청은 다음 Gin handler를
  한 번 호출합니다. 거부 응답은 `Retry-After`와 `X-RateLimit-Remaining`을
  보존하고, 기본 backend 오류는 세부 정보 없는 503 Problem으로 응답합니다.
- `NewJWT`는 대소문자를 구분하지 않는 정확히 하나의 `Bearer <token>` header만
  허용합니다. 중복/comma-joined 값, control/whitespace, 8 KiB 초과 token은
  거부합니다. 검증된 `*jwt.Reader`만 저장하고 `JWTReader`로 읽습니다. 오류
  callback에는 Authorization header를 제거한 request 복사본과 parser 세부
  정보·raw token이 없는 `AuthenticationError`를 전달합니다.
- `WrapResilience`는 route-level wrapper입니다. 응답 기록 전이고 `GetBody`로
  request body를 재생할 수 있을 때만 retry가 안전합니다. 이미 기록된 응답과
  재생할 수 없는 body는 `resilience.NonRetryable`이 되며 writer를 rollback하거나
  덮어쓰지 않습니다.

## 운영 runbook

1. **Preflight** — rollout 전에 example smoke test와 package 검사를 실행합니다.

   ```bash
   git rev-parse --show-toplevel
   # 기대: bluetape-go repository root
   go test -run 'Example(Bootstrap|Migration)$' ./web/gin
   go test ./web/gin
   go test -race ./web/gin
   ```

   Request-context header-name 오류는 별도 startup validation API 없이 첫 요청의
   400으로 표시됩니다. deterministic readiness smoke test는 example이 담당합니다.
2. **Canary (5분)** — 일부 traffic만 Gin adapter 경로로 보냅니다. 전체 관찰
   기간 동안 readiness는 `200`, 정상 요청은 `2xx`를 유지해야 합니다.

   ```bash
   curl -i https://service.example/readyz
   # 기대: HTTP/2 200과 문서화된 readiness body
   curl -i -H 'X-Auth-Subject: subject-1' https://service.example/orders
   # trusted 요청 기대: 2xx, raw error/token field 없음
   curl -i -H 'X-Auth-Subject: spoofed' https://service.example/orders
   # untrusted peer 기대: subject 무시 또는 안전한 4xx
   curl -i -H 'Authorization: Bearer <test-token>' https://service.example/orders
   # 기대: valid token은 2xx, missing/invalid token은 redacted 401
   curl -i https://service.example/orders
   # 기대: token/parser detail 없는 401 application/problem+json
   curl -i --max-time 2 https://service.example/orders
   # server가 cancellation을 관찰하면 기대: 408 application/problem+json과
   # bounded "Request canceled" body; deadline은 504로 매핑됩니다.
   ```

3. **Observe** — callback은 `adapter`, `kind`, `status`, `committed`,
   `request_id`, `duration`만 emit합니다. 해당 middleware 결정 이후
   `c.IsAborted()`, `c.Writer.Written()`, `AuthenticationError.Kind`를
   기록합니다. Authorization header, raw token, raw parser/backend error는
   기록하지 않습니다. JWT error handler에는 sanitized request 복사본이
   전달됩니다. Rate-limit과 resilience callback에는 caller-owned Gin
   context/request가 전달되고 custom handler에는 원래 error도 전달되므로,
   logging 전에 해당 값을 sanitize해야 합니다.

   | callback | 시점 | 필드 |
   | --- | --- | --- |
   | rate-limit error handler | abort 이후, response 반환 전; caller-owned request/error를 sanitize | adapter, kind, status, committed, request_id, duration |
   | JWT error handler | abort 및 sanitized request copy 이후 | adapter, kind, status, committed, request_id, duration |
   | resilience error handler | policy 결과 이후, uncommitted Problem 기록 전; caller-owned context/error를 sanitize | adapter, kind, status, committed, request_id, duration |

4. **Rollback** — raw token/parser error가 관찰되거나 canary error budget을
   넘으면 adapter middleware를 제거하고 이전 `net/http` 경로로 복원합니다.
   response를 기록한 route는 retry하지 않습니다.
5. **Recovery** — `gin.Recovery()`는 adapter chain 바깥에 둡니다. readiness가
   `200`, 정상 요청이 `2xx`이고 5xx/429/401 비율이 canary 이전 baseline으로
   돌아온 뒤에만 traffic을 복구합니다. 복구 전에 preflight와 모든 probe를
   다시 실행합니다.

## Migration 형태

이전에는 기존 `http.Handler`를 `net/http` 경로에서 serve하고, Gin 뒤에 유지해야
하면 `gin.WrapH(handler)`(또는 동등한 framework bridge)로 연결합니다. 이후에는
handler 본문을 유지한 채 새 Gin route를 `WrapResilience`로 감싸고, 위 bootstrap
순서에 `RequestContext`, `NewJWT`, `NewRateLimit`을 추가합니다.
`ExampleMigration`이 `gin.WrapH` bridge와 adapter 경로를 모두 compile-checked
형태로 보여 줍니다.

## 검증

```bash
go test -count=1 ./web/gin
go test -race -count=1 ./web/gin
go vet ./web/gin
```
