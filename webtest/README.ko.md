# webtest

[English](README.md) | [한국어](README.ko.md)

`webtest`는 `net/http` middleware의 계약을 검증하는 framework-neutral test
support package다. 각 scenario를 새 request, response recorder, observation
snapshot으로 실행하므로 이후 framework adapter가 같은 계약을 재사용할 수
있다.

이 package는 테스트 지원 코드이며 production middleware가 아니다. framework나
외부 서비스 dependency를 추가하지 않는다. Gin, Echo, Fiber adapter는 별도
후속 작업에서 다룬다.

## Import

```go
import "github.com/bluetape4k/bluetape-go/webtest"
```

## Scenario

`Adapter`는 `func(http.Handler) http.Handler`라는 좁은 경계를 사용한다.
`Run`은 adapter를 `Next`에 적용하고 `NewRequest`를 호출한 뒤, 복사한
`Observation`을 `Assert`에 전달한다.

```go
webtest.Run(t, webtest.Scenario{
    Name:    "accepts a request",
    Adapter: func(next http.Handler) http.Handler { return next },
    NewRequest: func(ctx context.Context) *http.Request {
        return httptest.NewRequestWithContext(ctx, http.MethodGet, "http://example.test/", nil)
    },
    Next: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
        w.WriteHeader(http.StatusNoContent)
    }),
    Assert: func(t *testing.T, got webtest.Observation) {
        if got.StatusCode != http.StatusNoContent {
            t.Fatalf("status = %d", got.StatusCode)
        }
    },
})
```

각 scenario는 기본 2초 timeout을 사용한다. timeout이 발생하면 request
context를 취소하고 제한된 시간 동안 cleanup을 기다린 뒤 테스트를 실패시킨다.
늦게 반환한 handler를 성공으로 바꾸지 않는다. 다른 제한 시간이 필요하면
`Scenario.Timeout`을 설정한다.

`Observation`에는 status, 복사한 header/body, next 호출 횟수, `Next`에 도달한
request가 들어 있다. runner는 global logger나 transport 상태를 변경하지
않는다. middleware가 소유하는 resource의 close를 검증할 때는
`CloseTracker`를 사용할 수 있다. 예를 들어 retry하는 `RoundTripper`의
response body close를 확인하는 데 사용한다.

## Conformance 범위

현재 `net/http` case는 `web`의 problem/context 경계, `resilience`의 status와
cancellation, `ratelimit`의 key와 거부 응답, response body 소유권,
caller-owned resilience event, panic finalization 경계를 확인한다. framework
adapter나 새 production recovery policy는 구현하지 않는다.

## Test

```bash
go test -count=1 ./webtest
go test -race -count=1 ./webtest
go test -run Example -count=1 ./webtest
```
