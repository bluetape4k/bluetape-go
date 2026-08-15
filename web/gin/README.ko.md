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
router := gin.New()
router.Use(gin.Recovery())
router.Use(ginadapter.RequestContext(web.RequestContextOptions{}))
router.Use(rateLimit, authentication)
router.GET("/orders", ginadapter.WrapResilience(handler, ginadapter.ResilienceOptions{}))
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
   go test -run 'Example(Bootstrap|Migration)$' ./web/gin
   go test ./web/gin
   go test -race ./web/gin
   ```

   Request-context header-name 오류는 별도 startup validation API 없이 첫 요청의
   400으로 표시됩니다. deterministic readiness smoke test는 example이 담당합니다.
2. **Canary** — 일부 traffic만 Gin adapter 경로로 보냅니다. trusted/untrusted
   request-context header, valid/missing JWT, allow/reject rate-limit 요청을 각각
   probe합니다.
3. **Observe** — callback에서 status, request ID, `c.IsAborted()`,
   `c.Writer.Written()`을 기록합니다. `AuthenticationError.Kind`만 기록하고
   Authorization header, raw token, raw parser/backend error는 기록하지 않습니다.
4. **Rollback** — canary route에서 adapter middleware를 제거하고 이전
   `net/http` 경로로 복원합니다. response를 기록한 route는 retry하지 않습니다.
5. **Recovery** — `gin.Recovery()`는 adapter chain 바깥에 둡니다. 401/429/503
   Problem을 safe status와 kind field로 조사하고 traffic 복구 전 preflight와
   canary probe를 다시 실행합니다.

## 검증

```bash
go test -count=1 ./web/gin
go test -race -count=1 ./web/gin
go vet ./web/gin
```
