# web

[English](README.md) | [한국어](README.ko.md)

`web`은 `net/http` handler가 공통으로 사용할 수 있는 framework-neutral helper를
제공합니다. 범위는 RFC 9457 Problem Details 응답과 검증된 request context 값의 두
경계입니다. Framework adapter, 인증·인가, middleware policy, logger/MDC 연동,
background 작업은 포함하지 않습니다.

## 가져오기

```go
import "github.com/bluetape4k/bluetape-go/web"
```

## Problem details

Application이 공개해도 되는 오류는 `ProblemError`로 표현합니다. 알 수 없는 오류는
detail을 응답에 복사하지 않고 `500 Internal Server Error`로 변환합니다.
Cancellation은 `408 Request Timeout`, deadline은 `504 Gateway Timeout`으로
변환합니다.

```go
type invalidOrderError struct{}

func (invalidOrderError) Error() string { return "order total is invalid" }

func (invalidOrderError) ProblemDetails() web.Problem {
    problem, _ := web.NewProblem(422, "Invalid order", "order total is invalid")
    return problem
}

func handler(w http.ResponseWriter, r *http.Request) {
    if err := web.WriteProblem(w, r, invalidOrderError{}); err != nil {
        // Response writer 또는 problem 값이 응답을 거부한 경우입니다.
        return
    }
}
```

`WriteProblem`은 status와 extension key를 검증하고 body를 status 쓰기 전에
직렬화합니다. 성공하면 정확히 `application/problem+json` media type을 설정하고,
request가 있으면 `URL.RequestURI()`를 `instance`로 사용합니다. nil request에서는
`instance`를 비워 두며, nil error 또는 writer는 `web.ErrInvalidProblem`을
반환합니다.

## Request context

`WithRequestContextOnRequest`는 request를 복사하고 그 context에
`RequestContext`를 저장합니다. Request ID와 correlation ID는 단일 line,
visible ASCII, 최대 256 byte 규칙을 통과한 뒤 사용합니다. Request ID가 없으면
주입한 `GenerateID` 또는 기본 UUID v7 generator를 사용하고, correlation ID가
없으면 request ID를 재사용합니다.

Auth subject와 W3C `traceparent`/`tracestate`는 request별 `TrustedProxy` predicate가
true일 때만 읽습니다. 이 helper는 인증·인가를 결정하지 않고 이 값을 response
header에 반영하지도 않습니다. 원본 request는 바뀌지 않으며 기존 cancellation
context도 유지됩니다.

```go
requestWithContext, value, err := web.WithRequestContextOnRequest(req, web.RequestContextOptions{
    TrustedProxy: func(r *http.Request) bool { return r.Header.Get("X-Edge") == "trusted" },
})
if err != nil {
    return err
}
_ = requestWithContext
_ = value
```

`#542`에서 HTTP middleware conformance를 추가합니다. Gin과 Echo adapter는 각각
`#543`, `#544`에서 다루며 JWT/JWKS 동작은 이 package 범위 밖에 둡니다.

## 테스트

```bash
go test -count=1 ./web
go test -race -count=1 ./web
```
