# Issue #544 Echo adapter 설계

## 문제와 목표

Issue #544는 framework-neutral `web`, `ratelimit`, `jwt`, `resilience` 계약을
Echo 애플리케이션에서 사용할 수 있는 얇은 adapter로 연결한다. 현재
`web/gin`이 증명한 package 경계를 유지하되 Echo를 core package에 노출하지
않고, Echo-specific middleware 계약과 RFC 9457 응답·redaction·재시도 경계를
테스트로 고정한다.

## 근거와 범위

- 상위 Epic: #540
- 현재 기준 adapter: `web/gin/*.go` 및 `web/gin/conformance_test.go`
- 수용 기준: Echo `httptest`, middleware conformance, README와 README.ko
- 의존성 확인: `github.com/labstack/echo/v4` 최신 확인 버전 `v4.15.4`,
  module `go 1.25.0`; repository는 `go 1.26.3`이므로 호환된다.
- 포함: Echo adapter package, 직접 의존성, 단위/conformance/race/example 테스트,
  request-path benchmark, package README 두 locale와 web README 연결, benchmark
  raw output·요약표·chart·use-case 분석 artifact.
- 제외: Fiber adapter, 일반 framework abstraction, JWKS provider(#545),
  core `web`/`jwt`/`ratelimit`/`resilience` API 변경.

## 대안과 선택

1. **Echo native middleware를 직접 구현**
   - Echo의 `Context`, `MiddlewareFunc`, `HandlerFunc`를 그대로 노출한다.
   - 공통 정책은 기존 package를 호출하므로 중복 정책 로직이 생기지 않는다.
   - 선택안이다. 호출자가 Echo 오류 처리와 commit 상태를 예측할 수 있다.
2. `echo.WrapMiddleware`로 모든 것을 `net/http` adapter로 우회
   - 기존 handler 재사용은 쉽지만 `echo.Context`의 `Set/Get`, `HandlerFunc` 오류
     반환, abort/commit 상태를 잃어 오류와 retry 경계를 흐린다.
3. 공통 framework abstraction 추가
   - Gin과 Echo를 한 API로 묶을 수 있지만 Epic의 비목표이며, 새 추상화와
     compatibility 결정이 범위를 키운다.

## 공개 계약

`web/echo` package는 다음을 제공한다.

- `AbortWithProblem(echo.Context, error) error`: RFC 9457 응답을 기록하고
  `Response().Committed` 이후에는 writer를 덮어쓰지 않는다.
- `RequestContext(web.RequestContextOptions) echo.MiddlewareFunc`: request 복사본에
  검증된 `web.RequestContext`를 연결하고 middleware 반환/패닉 뒤 원래 request
  pointer를 복원한다.
- `RateLimitOptions`, `RateLimitKeyFunc`, `NewRateLimit`: 공통 limiter handler를
  Echo chain으로 bridge한다. 허용 요청은 다음 handler를 한 번 호출하고, 거부/
  backend/cancellation은 status와 header를 보존한 Problem으로 처리한다.
- `JWTOptions`, `ContextParser`, `JWTErrorKind`, `AuthenticationError`, `NewJWT`,
  `JWTReader`: 정확히 하나의 case-insensitive `Bearer` header만 허용하고 검증된
  `*jwt.Reader`만 Echo context에 저장한다. 실패 callback에는 Authorization이 제거된
  request 복사본과 redacted 오류만 전달한다.
- `ResilienceOptions`, `WrapResilience`: Echo route handler를 공통 policy로
  감싼다. request context와 replayable body를 attempt마다 복원하며, replay
  불가능 body는 첫 시도만 허용하고 이후 retry를 fail-closed한다. commit된
  response와 cancellation도 retry하지 않는다. 실패 시 redacted observer를
  `DefaultResilienceErrorContextKey`에 기록하고 `ResilienceError`로 읽을 수 있다.

Echo의 context store는 public enumeration API가 없으므로 adapter가 임의의
`Set` 키를 snapshot한다고 약속하지 않는다. 따라서 retry attempt 사이에
복원할 수 있는 request/path/params와 adapter-owned 상태만 복원하고, handler가
store mutation을 수행한 뒤 오류를 반환하는 경우 commit 또는 non-retryable
경계로 fail-closed한다. 이 제한은 README와 테스트에 명시한다.

## 오류와 보안 경계

- 원인 오류, raw token, Authorization 값, provider payload를 Problem body나
  기본 callback에 노출하지 않는다.
- `context.Canceled`/`context.DeadlineExceeded`는 기존 `web.WriteProblem` 매핑을
  사용한다.
- `ContextParser`가 설정되면 request context를 parser I/O에 전달해 strict
  cancellation을 지원한다. 기존 `Parser`는 parse 전후만 확인하는 synchronous
  best-effort 경로이며, blocking `Parse`를 중단해야 하는 caller는
  `ContextParser`를 구현해야 한다.
- JWT failure callback request는 인증 header를 제거하고 body를 `http.NoBody`로
  격리해 callback이 원본 body를 소비하지 못하게 한다.
- custom error callback은 caller-owned Echo context와 원인 오류를 받을 수 있으므로
  로그 redaction 책임은 callback caller에게 있음을 문서화한다.
- 기본 resilience 오류 경로도 redacted observer를 Echo context에 남겨 outer
  logger/error handler가 low-cardinality 분류를 기록할 수 있게 하며, Problem body에는
  원인 오류를 노출하지 않는다.
- 이미 `Response().Committed`이면 adapter가 두 번째 status/body를 쓰지 않는다.

## 테스트와 DoD

- `webtest.Run`으로 problem, trusted/untrusted request context, rate-limit,
  JWT success/failure, resilience success/failure conformance를 검증한다.
- Echo-specific 테스트로 `Committed`, handler once-only, `JWTReader`, path/params,
  error callback request redaction, outer Echo HTTP error handler 경계를 검증한다.
- compile-checked `Example`/`Example_migration`을 제공한다.
- request-path benchmark는 context extraction, rate-limit, JWT, RFC 9457 Problem,
  resilience hook 조합의 local baseline을 serial/parallel CPU matrix로 기록하고,
  startup 시간이나 서로 다른 호스트의 수치를 framework 승패로 해석하지 않는다.
- `go test -count=1 ./web/echo`, `go test -race -count=1 ./web/echo`,
  `go vet ./web/echo`, formatter/lint, `git diff --check`, README locale parity,
  `make ci`와 PR CI를 통과한다.
- P0/P1 findings가 없고 Issue #544의 milestone/label/assignee와 PR metadata가
  live read-back으로 일치해야 한다.

## 롤백

Echo adapter 변경은 새 package와 dependency를 함께 revert할 수 있다. 공통
`web/context.go`의 duplicate trusted-header fail-closed hardening을 제외한
`web`, Gin adapter, `ratelimit`, `jwt`, `resilience` API는 수정하지 않으며,
실패 시 feature branch와 PR만 닫고 integration branch는 건드리지 않는다.
