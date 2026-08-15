# #541 web context 및 problem details 설계

## 목표

`bluetape-go`에 framework-neutral `web` package를 추가해 `net/http` 호출자가
요청 식별자와 제한된 전달 컨텍스트를 안전하게 추출하고, 공개해도 되는 오류를
RFC 9457 problem details JSON으로 결정론적으로 응답하도록 한다. 이슈의
`RFC7807` 명칭은 현재 표준인 RFC 9457이 RFC 7807을 대체한다는 사실을 반영해
문서와 media type에서는 최신 표준을 따른다.

## 현재 근거

- GitHub #541은 request ID, correlation ID, auth subject, trace context를 위한
  framework-neutral helper와 명시적인 trusted-header/proxy 규칙을 요구한다.
- `resilience`와 `ratelimit`은 이미 `net/http` adapter를 제공하지만, 공통 오류
  payload나 request context 저장소를 소유하지 않는다.
- `jwt`는 token 검증만 제공하고 HTTP 인증 정책을 소유하지 않는다.
- `docs/package-layout.md`는 명확한 domain과 package docs, example, test가 있는
  독립적인 top-level public package를 허용하고 catch-all utility package를
  금지한다.
- RFC 9457은 RFC 7807을 대체하며 `type`, `title`, `status`, `detail`, `instance`
  표준 멤버와 problem-type 범위의 extension member를 정의한다.

## 선택한 경계

새 top-level `web` package는 다음 두 책임만 가진다.

1. `Problem`과 `ProblemError`를 사용해 오류를 status와 problem details로
   변환하고 `application/problem+json`으로 쓴다.
2. `net/http` request에서 검증된 request/correlation ID와 trusted proxy가
   승인한 auth subject 및 W3C trace context를 읽어 `context.Context`에 저장한다.

Gin/Echo/Fiber adapter, middleware conformance harness, JWT 검증, logger/MDC,
인증·인가 정책, background refresh는 이 slice에 포함하지 않는다. 후속 train은
`#542`에서 conformance를 먼저 추가하고 `#543`/`#544`에서 framework-specific
package를 각각 만든다. `#545` JWKS provider는 독립 train으로 유지한다.

## API 계약

### Problem details

```go
type Problem struct {
    Type       string
    Title      string
    Status     int
    Detail     string
    Instance   string
    Extensions map[string]any
}

type ProblemError interface {
    error
    ProblemDetails() Problem
}

func NewProblem(status int, title, detail string) (Problem, error)
func ProblemFromError(err error) Problem
func WriteProblem(w http.ResponseWriter, req *http.Request, err error) error
```

`Status`는 100~599 범위만 허용한다. 비어 있는 `Type`은 `about:blank`으로,
비어 있는 `Title`은 `http.StatusText(Status)`로 채운다. 일반 오류의 detail은
내부 오류를 노출하지 않도록 `Internal Server Error`로 제한하고, `ProblemError`
가 명시한 detail만 caller-owned 공개 오류로 사용한다. `errors.Is`/`errors.As`를
깨뜨리지 않도록 오류 원인은 `ProblemError` 구현체 밖에서 감싼다.

표준 멤버와 충돌하는 extension key, 빈 key, control character를 거부한다.
extension 값은 `encoding/json`이 지원하는 값만 허용하고, JSON 직렬화 실패는
응답을 시작하기 전에 반환한다. `WriteProblem`은 nil request를 허용하되
`instance`를 채우지 않고, request가 있으면 `RequestURI`에서 fragment를
제거하지 않은 원문을 그대로 사용하지 않고 `URL.RequestURI()`를 사용한다.
zero-value `Problem`이나 `ProblemError`가 반환한 잘못된 status는 writer가
응답을 시작하기 전에 `ErrInvalidProblem`으로 거부한다. `WriteProblem`에 nil
error를 전달하는 것도 동일한 입력 오류로 처리한다.

`ProblemFromError`의 기본 status 매핑은 다음과 같다.

| 원인 | status | 공개 detail |
|---|---:|---|
| `context.DeadlineExceeded` | 504 | `Request deadline exceeded` |
| `context.Canceled` | 408 | `Request canceled` |
| 그 외 | 500 | `Internal Server Error` |

`WriteProblem`은 body를 메모리에서 먼저 직렬화한다. 직렬화나 writer 검증이
실패하면 status/header를 쓰기 전에 오류를 반환한다. 성공하면 응답 header를
`Content-Type: application/problem+json`으로 설정하고 problem의 status를 쓴 뒤
한 번만 body를 쓴다. 이미 응답이 시작된 여부는 `http.ResponseWriter` 계약상
판단하지 않으며, middleware/conformance layer가 이 경계를 후속 이슈에서
검증한다.

### Request context

```go
type RequestContext struct {
    RequestID     string
    CorrelationID string
    AuthSubject   string
    TraceParent   string
    TraceState    string
}

type RequestContextOptions struct {
    TrustedProxy   func(*http.Request) bool
    GenerateID     func() (string, error)
    RequestIDHeader string
    CorrelationIDHeader string
    AuthSubjectHeader string
    TraceParentHeader string
    TraceStateHeader string
}

func ExtractRequestContext(req *http.Request, options RequestContextOptions) (RequestContext, error)
func WithRequestContext(ctx context.Context, value RequestContext) context.Context
func RequestContextFromContext(ctx context.Context) (RequestContext, bool)
func WithRequestContextOnRequest(req *http.Request, options RequestContextOptions) (*http.Request, RequestContext, error)
```

기본 header는 `X-Request-ID`, `X-Correlation-ID`, `X-Auth-Subject`,
`traceparent`, `tracestate`다. ID와 subject는 trim 후 단일 line, visible ASCII,
최대 256 byte 규칙을 통과한 값만 사용한다. `traceparent`는 W3C 형식의
version/trace-id/parent-id/flags와 16진수·zero 값 규칙을 추가로 검증하고,
`tracestate`는 동일한 단일 line·길이 제한을 적용한다. 사용자 지정 header 이름은
HTTP token이어야 한다. request ID가 없으면 `GenerateID`를 호출하고,
기본 generator는 `id.NewUUIDV7`을 사용한다. correlation ID가 없으면 request ID를
재사용한다. generator 오류는 그대로 반환하며 빈 생성 결과는 거부한다.

`TrustedProxy`가 nil이거나 false이면 auth subject와 trace headers를 무시한다.
request/correlation ID는 권한을 부여하지 않으므로 안전한 형식 검증을 통과하면
사용하지만, 응답 header 반영과 proxy 체인 판정은 후속 middleware가 소유한다.
`TrustedProxy`는 전역 설정이 아니며 호출자가 request별로 제공한다. helper는
인증·인가를 수행하지 않고 단지 caller-owned context 값을 저장한다.

`WithRequestContextOnRequest`는 원본 request를 변경하지 않고 `req.WithContext`
결과를 반환한다. nil request와 nil context는 각각 명시된 오류 또는
`context.Background()` fallback으로 처리하며, `RequestContextFromContext`는
없는 값에 대해 `(RequestContext{}, false)`를 반환한다.

## 실패 모드와 완화

1. 악성 header가 log/response injection을 일으킬 수 있다. 단일 line, visible
   ASCII, 길이 제한을 적용하고 거부된 값을 보존하지 않는다.
2. untrusted proxy가 auth subject를 주입할 수 있다. trusted proxy predicate가
   true일 때만 해당 header를 읽고, 기본값은 읽지 않는다.
3. 공개 오류가 내부 오류 detail을 노출할 수 있다. `ProblemError`만 명시적인
   detail을 제공하고 일반 error는 고정된 공개 문구로 매핑한다.
4. extension 값이 순환 구조이거나 표준 key를 덮어쓸 수 있다. marshal 전에
   key를 검증하고 `encoding/json` 오류를 반환한다.
5. caller cancellation을 background 작업으로 바꿀 수 있다. helper는 context를
   복사하거나 goroutine을 만들지 않고 request context를 그대로 연결한다.

## 테스트와 수용 기준

- status 범위 검증, 기본 status/title/type/detail 매핑, `errors.Is` 기반
  cancellation/deadline 매핑을 table-driven test로 검증한다.
- 표준 JSON field, extension 정렬/충돌, content type, status code, request
  instance, 직렬화 오류를 검증한다.
- trusted/untrusted header, invalid header, generation fallback, custom header
  names, request context round-trip, nil/cancellation context를 검증한다.
- `Example...` test와 `web/README.md`, `web/README.ko.md`를 추가한다.
- `go test -count=1 ./web`, `go test -race -count=1 ./web`, `go vet ./web`,
  `make fmt-check`, `make tidy-check`, `make lint`, `git diff --check`를
  실행한다.

## DoD

- [x] `web` public API와 package docs가 `#541` 범위만 노출한다.
- [x] 모든 신규 동작에 RED→GREEN TDD 증거가 있다.
- [x] unit, example, race, vet, lint, format, tidy 검증이 통과한다.
- [x] README 양쪽 언어가 API·trusted-header 경계를 동일하게 설명한다.
- [x] `#542`가 재사용할 수 있는 framework-neutral surface를 유지한다.
- [ ] PR은 `feat/web-api-541` head와 `develop` base를 사용하고 `#541` 및
  `#540`을 연결한다.

## 참고

- [RFC 9457](https://www.rfc-editor.org/rfc/rfc9457.html) — Problem Details for
  HTTP APIs; RFC 7807 successor.
- [RFC 7807](https://www.rfc-editor.org/info/rfc7807/) — issue terminology
  reference; obsolete status is explicitly documented above.
