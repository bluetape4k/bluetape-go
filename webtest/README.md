# webtest

[English](README.md) | [한국어](README.ko.md)

`webtest` is a framework-neutral test support package for `net/http` middleware
conformance. It runs each scenario with a fresh request, response recorder, and
observation snapshot so future framework adapters can reuse the same contract.

It is test support, not production middleware. It has no framework or external
service dependency. Gin, Echo, and Fiber adapters are separate follow-up work.

## Import

```go
import "github.com/bluetape4k/bluetape-go/webtest"
```

## Scenario

`Adapter` has the narrow `func(http.Handler) http.Handler` shape. `Run` applies
it to `Next`, invokes `NewRequest`, and passes a defensive `Observation` copy to
`Assert`.

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

Each scenario uses a two-second default timeout. A timeout cancels the request
context, waits for bounded cleanup, and fails the test; it never turns a late
handler return into success. Set `Scenario.Timeout` when a case needs a
different bounded limit.

`Observation` contains the status, copied headers/body, next-call count, and the
request that reached `Next`. The runner does not mutate global logger or
transport state. `CloseTracker` is available for resources whose close
ownership belongs to the middleware, such as a retryable `RoundTripper`
response body.

## Conformance scope

The current `net/http` cases cover `web` problem/context boundaries,
`resilience` status and cancellation behavior, `ratelimit` keys and rejection
mapping, response-body ownership, caller-owned resilience events, and panic
finalization boundaries. They do not implement a framework adapter or a new
production recovery policy.

## Test

```bash
go test -count=1 ./webtest
go test -race -count=1 ./webtest
go test -run Example -count=1 ./webtest
```
