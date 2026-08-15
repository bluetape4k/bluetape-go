# Issue #543 Gin adapter 설계

## 문서 상태

- 상태: 승인된 설계 초안
- 대상: `bluetape4k/bluetape-go#543`
- 상위 Epic: `#540`
- 기준 브랜치: `develop`
- 작성일: 2026-08-16

## 배경과 목표

`web`의 framework-neutral helper와 `#542` HTTP middleware conformance가
완료되었으므로, 첫 번째 native web framework adapter로 Gin을 추가한다. 이
adapter는 기존 request context, rate limiter, JWT parser, resilience policy의
계약을 Gin의 `gin.HandlerFunc` 경계로 옮기되, core 패키지에 Gin 의존성을
전파하지 않는다.

공식 Gin middleware 계약은 `gin.HandlerFunc`이며, middleware가 보통
`c.Next()`로 downstream chain을 실행한다. 따라서 adapter는 Gin의 context와
response writer 의미를 유지하고, 기존 `net/http` helper의 동작을 얇게
연결해야 한다.

## 범위

다음 public API를 `web/gin`에 추가한다. 디렉터리 이름은 import 경로와
일치시키고, 패키지 이름은 표준 `gin` 패키지와의 충돌을 피하도록
`ginadapter`로 한다.

```go
package ginadapter

const DefaultJWTContextKey = "bluetape.web.gin.jwt"

func RequestContext(options web.RequestContextOptions) gin.HandlerFunc

type RateLimitOptions struct {
	Limiter      ratelimit.Limiter
	KeyFunc      RateLimitKeyFunc
	Tokens       int64
	ErrorHandler func(*gin.Context, ratelimit.Result, error)
}

type RateLimitKeyFunc func(*gin.Context) string

func NewRateLimit(options RateLimitOptions) (gin.HandlerFunc, error)

type ContextParser interface {
    ParseContext(context.Context, string, ...jwt.ParseOption) (*jwt.Reader, error)
}

type JWTOptions struct {
	Parser        jwt.Parser
	ContextParser ContextParser
	Header        string
	Scheme        string
	ContextKey    string
	ParseOptions  []jwt.ParseOption
	ErrorHandler  func(*gin.Context, error)
}

type JWTErrorKind string

const (
	JWTErrorMissing   JWTErrorKind = "missing"
	JWTErrorMalformed JWTErrorKind = "malformed"
	JWTErrorInvalid   JWTErrorKind = "invalid"
	JWTErrorExpired   JWTErrorKind = "expired"
	JWTErrorCanceled  JWTErrorKind = "canceled"
)

type AuthenticationError struct {
	Kind JWTErrorKind
}

func (e AuthenticationError) Error() string
func (e AuthenticationError) ProblemDetails() web.Problem

func NewJWT(options JWTOptions) (gin.HandlerFunc, error)

type ResilienceOptions struct {
	Policies     []resilience.Policy[struct{}]
	ErrorHandler func(*gin.Context, error)
}

func WrapResilience(next gin.HandlerFunc, options ResilienceOptions) gin.HandlerFunc

func AbortWithProblem(c *gin.Context, err error) error

func JWTReader(c *gin.Context, key string) (*jwt.Reader, bool)
```

구현 시 실제 선언은 Go formatter와 기존 public API naming에 맞춰 확정하되,
위 계약에서 벗어나는 새 추상화나 외부 dependency는 추가하지 않는다.

### Request context adapter

`RequestContext`는 다음 순서로 동작한다.

1. 현재 `c.Request`에 대해 `web.WithRequestContextOnRequest`를 호출한다.
2. 성공한 request 복사본만 `c.Request`에 임시 연결하고 `c.Next()`를 실행한다.
3. `defer`로 원래 request 포인터를 복구한다. downstream panic과 Gin
   `Recovery` 경로에서도 복구가 보장된다.
4. 추출 또는 검증 오류는 `AbortWithProblem`으로 RFC 9457 400을 기록하고
   `c.Abort()`한다.

request context에 이미 연결된 cancellation, deadline, 원본 header/body는
`http.Request.WithContext`와 Gin writer를 통해 보존한다. adapter는 goroutine을
생성하지 않는다. `TrustedProxy`가 nil이면 restricted header는 항상 신뢰하지
않으며, adapter는 Gin의 `ClientIP`, `X-Forwarded-For`, client가 보낸 임의
header를 신뢰 판정에 사용하지 않는다. caller가 제공하는 predicate는 peer IP,
mTLS 또는 server-established metadata처럼 client가 바꿀 수 없는 신호만
사용해야 한다.

### Rate-limit adapter

`NewRateLimit`은 Gin-native `RateLimitOptions`를 받아 limiter, Gin-native
`RateLimitKeyFunc`, token 수, 오류 handler를 설정한다. 내부에서는 기존
`ratelimit.HandlerOptions`로 변환해 framework-neutral `ratelimit.Handler`를
한 번 구성하고, request context에 현재 `*gin.Context`를 연결한 bridge
`http.Handler`를 downstream으로 사용한다. 따라서 caller는 `net/http`
writer/request callback을 직접 다루지 않는다.

- 허용: 기존 handler가 `c.Next()`를 한 번 실행한다.
- 거부: 기존 `Retry-After`, `X-RateLimit-Remaining`, 429 의미를 유지하고
  `c.Abort()`한다.
- limiter/key 오류: 기존 오류 handler가 기록한 상태와 body를 유지하고
  `c.Abort()`한다.
- request context가 `context.Canceled` 또는 `context.DeadlineExceeded`이면
  backend 오류로 뭉뚱그리지 않고 기존 `web.ProblemFromError`의 cancellation/
  deadline mapping을 사용한다. downstream은 호출하지 않고 bounded return을
  보장한다.
- backend 오류: 기본 adapter handler는 원인 문자열을 response에 복사하지 않고
  redacted 503 Problem을 기록한다. caller가 `RateLimitOptions.ErrorHandler`를
  직접 제공한 경우 그 공개 범위와 logging은 caller 책임이다.
- key function은 `*gin.Context`를 기준으로 호출하며, 기본값은
  `ratelimit.RemoteIPKey`를 adapter 내부에서 감싼 함수다.

이 bridge는 rate-limit 판단을 재구현하지 않으므로 core와 Gin의 결과가
달라지는 drift를 방지한다.

### JWT adapter

`NewJWT`는 다음 기본값을 사용한다.

- header: `Authorization`
- scheme: `Bearer` (대소문자 무시)
- context key: 패키지 상수로 제공하는 안정된 기본 키
- error handler: RFC 9457 Problem Details 401 응답

각 adapter 생성 시 `ParseOptions`와 resilience `Policies` slice는 방어적으로
복사한다. 생성 이후 caller가 원본 slice를 변경해도 serving goroutine의 설정과
data race가 변하지 않는다.

Bearer 값이 없거나 형식이 잘못되었거나 parser가 실패하면 token 원문을
response, log, Gin context에 노출하지 않고 redacted authentication error로
오류 handler를 호출한다. callback용 오류는 parser의 원문 오류나 `Unwrap`
경로를 노출하지 않으며, 안정된 분류만 보존한다. callback을 호출할 때는
Authorization header를 제거한 request 복사본을 임시 연결해 callback이 원문
token을 다시 읽을 수 없게 하고, 호출 뒤 원래 request 포인터를 복구한다.
성공한 `*jwt.Reader`만 `c.Set`으로 저장하며 `JWTReader` helper로 읽는다.
`JWTReader`의 빈 key는 `DefaultJWTContextKey`를 사용한다.

`AuthenticationError`는 token 원문이나 parser 원인 오류를 포함하지 않는
안정된 분류 API다. `Error()`는 `authentication failed: <kind>` 형태의 고정
문자열만 반환하고 `Unwrap()`을 제공하지 않는다. `ProblemDetails()`는
고정된 401 `web.Problem`만 반환한다. caller는 `errors.As`로 `Kind`를 읽어
metrics/audit를 문자열 파싱 없이 분류할 수 있다.

`Parser`와 `ContextParser`가 모두 nil이거나 동시에 설정되면 생성자는 오류를
반환한다. Header, scheme, context key가 비어 있으면 각각 기본값을 사용하고,
whitespace/control 문자가 포함되거나 Authorization header가 여러 개 또는
comma-joined 값으로 모호하면 요청을 401로 거부한다. 문법은 정확히 하나의
`Bearer <non-empty-token>` 값이며, token은 control 문자를 포함하지 않고 최대
8 KiB다. `ParseOptions` 안의 nil option도 생성자에서 거부한다.
Parser, ContextParser, limiter, policy interface의 typed-nil 값도 생성자에서
거부해 serving 중 nil receiver panic을 방지한다.

`ContextParser`가 설정된 경우 request context를 전달한다. 그렇지 않은 기존
`jwt.Parser`는 `Parse`를 사용하고, parse 전후로 request cancellation을
확인한다. 이 legacy 경로는 진행 중인 blocking parse를 중단할 수 없는
best-effort 계약이며, strict cancellation이 필요한 caller는 ContextParser를
사용해야 한다. 별도 field를 두어 기존 Parser 구현과 `DistributedProvider`
같은 context-only provider를 모두 source-compatible하게 수용한다.

### Resilience route wrapper

`WrapResilience`는 route-level wrapper로만 사용한다.

`next`가 nil이면 constructor는 request-time panic 대신 기존 core handler와
같은 404 not-found 동작을 사용한다. `JWTReader`는 nil context 또는 없는 key에
대해 `(nil, false)`를 반환한다.

1. wrapper가 받은 `gin.HandlerFunc`를 `resilience.Run`의 operation으로 감싼다.
2. 각 operation 시도 동안 `c.Request`를 `req.WithContext(policyCtx)`로
   임시 교체하고, `defer`로 원래 request를 복구한다.
3. operation 시작 시점의 Gin error 수를 기록하고, 해당 시도에서 새로
   추가된 `c.Error(err)` 또는 `c.AbortWithError` 중 가장 최근 error만 policy
   오류로 연결한다. retry 시도 사이에는 시도-local error를 제거해 이전
   시도의 오류가 다음 시도를 오염시키지 않게 한다.
4. operation이 반환되기 직전에 response가 이미 기록되었으면, 새로 수집한
   handler 오류를 `resilience.NonRetryable`로 감싸 Gin error chain에 남긴다.
   core retry/circuit-breaker가 이를 failure로 관찰하되 재시도하지 않으므로
   duplicate side effect와 성공 상태 은폐를 함께 막는다.
5. handler가 오류를 남기지 않아도 operation 반환 직후 `policyCtx.Err()`를
   확인한다. response가 아직 기록되지 않았다면 cancellation/deadline을
   policy 오류로 반환하고, 기록되었다면 위 non-retryable 경계를 적용한다.
6. 오류가 response 기록 전에 발생하면 설정된 오류 handler 또는 기본
   resilience 오류 handler로 응답하고 `c.Abort()`한다. 기본 handler는
   policy 거부/고갈을 503 Problem으로 매핑하고, context cancellation/deadline은
   기존 `web` 매핑(408/504)을 유지한다.
7. 이미 response가 기록된 뒤 발생한 오류는 writer를 덮어쓰지 않는다. 해당
   오류는 Gin error chain에 남기고 caller가 관찰할 수 있게 한다.

Gin handler는 반환값이 없고 response rollback도 제공하지 않으므로, 이
wrapper를 `router.Use`로 전체 chain에 걸어 자동 retry middleware처럼
사용하는 것은 계약으로 보장하지 않는다. retry policy를 사용할 때 route는
idempotent해야 하며, response를 기록하기 전에 context cancellation과 오류를
반환하는 애플리케이션 handler만 안전하게 재실행할 수 있다. adapter는 Gin의
`Recovery` 또는 logger를 대체하지 않는다.

response가 기록된 오류는 `resilience.NonRetryable` extension을 사용한다.
이 extension은 `resilience` core의 retry default와 circuit-breaker default가
failure로 관찰하되 재시도하지 않도록 하는 작은 공통 계약이며, 기존 policy와
source compatibility를 유지한다.

구현 계획에는 다음 최소 core extension을 포함한다.

```go
var ErrNonRetryable error

type NonRetryableError struct {
	Cause error
}

func NonRetryable(err error) error
func IsNonRetryable(err error) bool
```

`RetryPolicy`는 `IsNonRetryable(err)`를 custom `RetryIf`보다 먼저 확인해
재시도를 중단하고, circuit breaker는 이를 failure로 기록한다. 원인 error의
`errors.Is` 의미는 보존하되 오류 문자열은 adapter response에 직접 노출하지
않는다.

## 공통 오류와 response 계약

`AbortWithProblem`은 `web.WriteProblem(c.Writer, c.Request, err)`를 호출해
RFC 9457과 호환되는 `application/problem+json`을 유지하고, `c.Abort()` 뒤
writer 오류를 반환한다. `c`가 nil이면 `web.ErrInvalidProblem`을 반환하고
panic하지 않는다. pre-write marshal 오류로 writer가 아직 비어 있으면
안전한 generic 500 Problem fallback을 한 번 시도해 200 상태로 남지 않게 한다.
response가 이미 기록된 경우에는 writer를 건드리지 않고 `c.Abort()`만
수행하며 nil을 반환한다. 호출자는 반환 오류를 `c.Error` 또는 자체 observer로
전달할 수 있다.

기존 `web.ProblemFromError`는 일반 오류를 500으로 매핑하므로, adapter는
request-context 입력 오류를 400, JWT 입력/검증 오류를 401로 바꾸는 내부
`web.ProblemError` wrapper를 사용한다. 이 wrapper는 공개 API로 노출하지 않고
검증된 상태/제목만 제공하며, parser의 오류 문자열이나 token 원문을 detail에
복사하지 않는다. cancellation/deadline과 이미 공개된 `ProblemError`는 기존
core 매핑을 그대로 따른다.

| 상황 | 기본 상태 | body/header 계약 | chain |
| --- | ---: | --- | --- |
| request context header 오류 | 400 | RFC 9457, 내부 400 ProblemError | `Abort` |
| JWT 누락/형식/검증 오류 | 401 | RFC 9457, token 원문 비노출 | `Abort` |
| limiter 거부 | 429 | 기존 `Retry-After`, `X-RateLimit-Remaining` | `Abort` |
| limiter backend 오류 | 503 | 설정된 handler 결과 | `Abort` |
| resilience policy 거부/timeout | 503 또는 설정 handler | 기본 503 Problem, context 오류는 기존 mapping | `Abort` |
| downstream panic | adapter가 변환하지 않음 | Gin `Recovery` 책임 | Gin 설정 따름 |
| request cancellation/deadline | 기존 `web`/`resilience` 매핑 | writer가 아직 비어 있을 때만 problem 기록 | `Abort` |

응답이 이미 기록된 경우 status, header, body를 다시 쓰지 않는다. 따라서
각 adapter는 `c.Writer.Written()`을 확인한 뒤에만 기본 오류 응답을 기록한다.

## 테스트 설계

### Unit 및 Gin `httptest`

- request context: 생성된 request ID, trusted header, 잘못된 header, 원본
  request 복구, 정상/패닉 경로 request 복구, cancellation 전파
- rate limit: 허용, 429와 retry header, empty key, backend 오류, downstream
  미호출, `Allow` 중 request cancellation과 bounded return
- JWT: 유효 token, 누락/비 Bearer/잘못된 서명/만료, parser context 취소,
  context-capable parser의 cancellation, legacy parser의 best-effort 표시,
  parse 직후 cancellation에서 reader/downstream 미저장·미호출, malicious
  parser error redaction, duplicate/comma/oversize header, reader 저장과 token
  원문 비저장
- resilience: 성공, policy 거부, timeout/cancellation, `c.Error` 연결,
  response 기록 후 retry 차단/side-effect 1회, 오류의 non-overwrite, Gin
  `Recovery`와의 조합, response-committed policy failure event/state
- problem: RFC 9457 content type, 상태 매핑, 이미 기록된 writer 보호, nil
  context, invalid extension marshal 실패와 generic fallback
- 설정 slice 복사: 생성 후 caller mutation과 concurrent serving의 `-race` 보호
- failing/short-write writer에서 `AbortWithProblem` 반환 오류 관찰
- retry 시도별 `c.Errors`/`c.Keys` 상태와 response commit을 분리하고,
  shared rate-limit bridge의 동시 요청이 서로 다른 Gin context/writer/key를
  보존하는 bounded `-race` 시나리오

### Framework-neutral conformance

`webtest` production package에는 Gin dependency를 추가하지 않는다. 테스트에서
Gin engine을 `http.Handler`로 노출하고 `webtest.Adapter`로 감싸 기존
conformance scenario를 재사용한다. 이를 통해 `#542`의 response, cancellation,
error isolation 계약을 framework별로 동일하게 검증한다.

## Benchmark와 문서 산출물

Issue #560의 후속 요구에 맞춰 `web/gin` request path benchmark를 추가한다.
다음 경로를 같은 명령과 환경에서 측정한다.

- request context extraction
- rate-limit check
- JWT parser validation (JWKS network/provider는 #545 후속 범위)
- RFC 9457 problem response
- resilience policy hook

raw output, 실행 명령/환경/날짜, `-benchmem`·`ReportAllocs` 설정, baseline과
warm/cold 상태, 방향이 표시된 결과 표, Markdown으로 렌더링 가능한 chart,
use-case 해석, caveat와 제외한 해석을
`docs/research/outputs/issue-543/`에 보존한다. benchmark는 startup-only
비교나 보편적 framework 승자 선언을 하지 않는다.

재현 명령은 다음 형식을 고정한다.

```bash
go test -run '^$' -bench '^BenchmarkGinAdapter' -benchmem -count=5 ./web/gin
```

benchmark source는 `testing.B.ReportAllocs`를 호출하고 deterministic local
fixture를 사용한다. 환경 ledger에 Go/Gin 버전, OS/CPU, 실행일, Gin engine
설정, JWT parser warm/cold 상태와 fixture를 기록한다. CI는 benchmark를 기본
gate로 실행하지 않으며, `make bench-web-gin` opt-in target과 raw artifact
검증 명령으로 재현한다.

각 경로는 `gin` no-op, direct core, bridge-only, full adapter 행을 분리한다.
측정 구간은 `b.ResetTimer` 뒤의 요청 경로만 포함하고, `ns/op`, `B/op`,
`allocs/op`, derived ops/sec를 기록한다. serial과 `b.RunParallel`을
`-cpu 1,2,4`로 별도 실행하며, resilience는 zero/one policy와 retry-attempt,
problem은 normal/already-written/fallback 분기를 분리한다. `testing.B` 평균값과
latency percentile은 구분하고 p95/p99 주장은 별도 bounded latency harness가
있을 때만 허용한다. warm/cold와 concurrency profile을 한 표에서 섞지 않는다.

다음 문서를 함께 갱신한다.

- `web/gin/README.md`
- `web/gin/README.ko.md`
- root `README.md`, `README.ko.md` package table
- `web/README.md`, `web/README.ko.md` adapter 안내 링크
- Go doc comments와 examples

두 README에는 컴파일 가능한 bootstrap example을 같은 API로 수록한다.

```go
engine := gin.New()
engine.Use(gin.Recovery())
engine.Use(ginadapter.RequestContext(web.RequestContextOptions{
	TrustedProxy: trustedPeer,
}))
engine.Use(jwtMiddleware)
engine.Use(rateLimitMiddleware)
engine.GET("/orders/:id", ginadapter.WrapResilience(orderHandler, resilienceOptions))
```

예제에는 `net/http` middleware를 Gin에 임의로 재사용하지 않고, adapter의
Gin-native callback과 `JWTReader` helper를 사용하는 before/after migration
설명을 포함한다. production bootstrap은 Recovery → request context →
authentication/rate-limit → route resilience 순서를 고정하고, trusted-peer
predicate·readiness preflight·error observer 연결을 명시한다.

문서 parity 검증은 다음 matrix로 수행한다.

| 항목 | `README.md` | `README.ko.md` | 검증 |
| --- | --- | --- | --- |
| 설치/import 경계 | 동일 API/명령 | 동일 API/명령의 한국어 설명 | diff/read-back |
| bootstrap 순서 | 동일 code fence | 동일 code fence | example compile |
| 오류·cancellation·retry 주의 | 동일 의미 | 동일 의미 | checklist |
| benchmark 재현 | 동일 command/path | 동일 command/path | `make bench-web-gin` |

## 비목표와 제약

- Echo, Fiber, chi adapter는 추가하지 않는다.
- route registration, application lifecycle, global recovery/logger를 소유하지
  않는다.
- `net/http` helper를 제거하거나 대체하지 않는다.
- Gin dependency는 단일 root module의 module graph에는 존재할 수 있지만,
  source import는 `web/gin` 하위 패키지에만 둔다. `web`, root package, 기타
  core package가 Gin symbol을 import하지 않는 것을 CI/review에서 검증한다.
- adapter 내부에서 raw JWT token을 장기 보관하거나 로그에 기록하지 않는다.
- response buffering/rollback을 새 framework abstraction으로 만들지 않는다.

## 수용 기준

1. `go.mod`에서 Gin dependency가 `web/gin` 사용 경계에만 연결된다.
2. 위 네 capability가 기존 core API의 cancellation, error, header, status 의미를
   보존한다.
3. Gin `httptest`와 framework-neutral conformance가 통과한다.
4. `make fmt-check`, `make tidy-check`, `make vet`, `make lint`, `make test`,
   `make race` 및 repository CI가 통과한다. Testcontainers package 병렬성은
   repository Makefile의 `-p 1` 정책을 따른다.
5. `resilience.NonRetryable`의 retry 중단·circuit failure 관찰과 `AuthenticationError`
   분류 API가 unit test로 고정된다.
6. README 두 언어에는 production bootstrap 예제(`gin.Recovery` → request
   context → auth/rate-limit/resilience 순서), trusted-proxy 설정, 오류
   observer 연결, readiness/장애 확인 항목이 포함되고 benchmark evidence도
   동기화된다.
7. public API와 non-goal이 Go doc, 설계/계획 문서, PR body에서 일치한다.
8. P0/P1 blocker가 없는 7-tier review와 lesson 기록이 남는다.

## 조사 근거

- Gin 공식 middleware 문서: <https://gin-gonic.com/en/docs/middleware/>
- Gin v1.12.0 release: <https://github.com/gin-gonic/gin/releases/tag/v1.12.0>
- Issue #543: <https://github.com/bluetape4k/bluetape-go/issues/543>
- Issue #540: <https://github.com/bluetape4k/bluetape-go/issues/540>
- Issue #560 benchmark follow-up:
  <https://github.com/bluetape4k/bluetape-go/issues/560>
- Issue #545 JWKS provider follow-up:
  <https://github.com/bluetape4k/bluetape-go/issues/545>
- ecosystem parity research gate:
  <https://github.com/bluetape4k/bluetape4k-wiki/blob/develop/research/2026-07-09-bluetape-go-ecosystem-parity-research-gate.md>

## 설계 자체 점검 (SPW-01~05)

- SPW-01 audience/purpose/evidence: Go library maintainer와 adapter consumer를
  대상으로 목표, 범위, 공식 Gin 근거, Issue/benchmark 근거를 명시했다.
- SPW-02 artifact contract: public API, 오류/response, 테스트, benchmark,
  README 산출물과 수용 기준을 고정했다.
- SPW-03 Korean naturalness: 기술 용어와 API 이름을 제외한 설명·표·결정은
  한국어로 작성했고, 문장 주어와 상태 표현을 일관되게 유지했다.
- SPW-04 technical traceability: 각 API 결정은 기존 `web`, `ratelimit`,
  `jwt`, `resilience`, `webtest` 계약과 연결했으며 retry/side effect,
  panic 복구, legacy parser cancellation, writer 오류, token redaction,
  trusted-proxy fail-closed, RFC 9457 표기를 명시했다.
- SPW-05 final read-back: 구현 전에 파일 경로, import 경계, 테스트 명령,
  benchmark evidence, non-goal을 다시 읽어 계획 문서와 대조한다.
