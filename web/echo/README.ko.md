# web/echo

[English](README.md) | [한국어](README.ko.md)

`web/echo`는 framework-neutral `web`, `ratelimit`, `jwt`, `resilience` 계약을
Echo에 연결하는 얇은 adapter입니다. Echo 의존성은 이 package에만 격리하며 core
package와 Gin adapter는 Echo를 import하지 않습니다.

## 설치와 bootstrap

```go
import (
    "net/http"

    "github.com/bluetape4k/bluetape-go/jwt"
    "github.com/bluetape4k/bluetape-go/ratelimit"
    "github.com/bluetape4k/bluetape-go/web"
    echoadapter "github.com/bluetape4k/bluetape-go/web/echo"
    "github.com/labstack/echo/v4"
)

func buildServer() (*echo.Echo, error) {
    provider, err := jwt.NewFixedHMACProvider(jwt.HS256, []byte("0123456789abcdef0123456789abcdef"))
    if err != nil { return nil, err }
    limiter, err := ratelimit.New(ratelimit.Options{RatePerSecond: 10, Burst: 10})
    if err != nil { return nil, err }
    rateLimit, err := echoadapter.NewRateLimit(echoadapter.RateLimitOptions{Limiter: limiter})
    if err != nil { return nil, err }
    authentication, err := echoadapter.NewJWT(echoadapter.JWTOptions{Parser: provider})
    if err != nil { return nil, err }

    server := echo.New()
    server.Use(echoadapter.RequestContext(web.RequestContextOptions{}), rateLimit, authentication)
    server.GET("/orders", echoadapter.WrapResilience(func(c echo.Context) error {
        return c.NoContent(http.StatusNoContent)
    }, echoadapter.ResilienceOptions{}))
    return server, nil
}
```

compile-checked [`example_test.go`](example_test.go)에 동일한 bootstrap과
`echo.WrapHandler`를 사용하는 migration 예제가 있습니다.

## Middleware 계약

- `AbortWithProblem`은 `application/problem+json`을 기록하고 이미 commit된
  Echo 응답을 덮어쓰지 않습니다. 알 수 없는 오류의 detail은 framework-neutral
  `web` mapper가 redaction합니다.
- `RequestContext`는 검증된 request context를 request 복사본에 저장하고
  middleware가 반환하거나 panic한 뒤 원래 request pointer를 복원합니다. 제한
  header는 caller의 `TrustedProxy` predicate가 true일 때만 읽습니다.
- `NewRateLimit`은 limiter가 허용한 경우에만 다음 Echo handler를 한 번 호출합니다.
  거부 응답은 `Retry-After`와 `X-RateLimit-Remaining`을 보존하고 backend 오류는
  안전한 503 Problem으로 반환합니다.
- `NewJWT`는 대소문자를 구분하지 않는 정확히 하나의 `Bearer <token>` header만
  허용합니다. 중복/comma-joined 값, control/whitespace, 8 KiB 초과 token을
  거부합니다. `JWTReader`는 context에 저장한 검증된 `*jwt.Reader`만 반환합니다.
  기본 실패 응답은 redacted 401이며 custom callback에는 설정한 인증 header와
  Authorization을 제거한 request 복사본을 전달합니다.
- `WrapResilience`는 route-level wrapper입니다. 각 attempt에서 request context와
  replayable body를 복제합니다. 이미 commit된 응답, 재생할 수 없는 body,
  cancellation은 retry하지 않습니다. Echo context에는 key 열거 API가 없으므로
  adapter가 변경한 key만 복원하며, 임의 store 변경을 retry해야 하는 handler는
  해당 변경을 non-retryable 경계 밖에 두어야 합니다.

## Migration

기존 `http.Handler`는 `echo.WrapHandler(handler)`로 Echo 뒤에 유지합니다. 새
route에는 `WrapResilience`를 적용하고 `RequestContext`, `NewRateLimit`, `NewJWT`는
필요한 middleware만 선택합니다. 이 adapter는 global logger, framework abstraction,
Fiber 지원을 추가하지 않습니다.

## Framework 경계 차이

`net/http` core는 `http.ResponseWriter`에 직접 쓰고 handler 오류를 호출자에게
반환합니다. Echo handler는 `error`를 반환하고 `echo.Context` store와
`Response().Committed`를 노출하므로, 이 adapter는 middleware가 거부한 요청을
Echo chain에서 중단하고 이미 commit된 응답을 덮어쓰지 않습니다. Gin adapter는
Gin의 abort/index와 writer 상태를 사용하므로 Gin 전용 abort 동작을 Echo에서
암묵적으로 재현하지 않습니다. 바깥 Echo 경계에서 framework-native 오류 처리를
유지하고 기존 `http.Handler` route에는 `echo.WrapHandler`를 사용합니다.

request-path benchmark 근거는
[`docs/research/outputs/issue-544`](../../docs/research/outputs/issue-544/README.md)에
기록했습니다.

## 검증

```bash
go test -run '^Example$|^Example_migration$' -count=1 ./web/echo
go test -count=1 ./web/echo
go test -race -count=1 ./web/echo
go vet ./web/echo
```
