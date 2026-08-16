# web

[English](README.md) | [한국어](README.ko.md)

`web` provides framework-neutral helpers for `net/http` handlers. It covers two
boundaries: RFC 9457 Problem Details responses and validated request context
values. It does not implement a framework adapter, authentication, authorization,
middleware policy, logger/MDC integration, or background work.

## Import

```go
import "github.com/bluetape4k/bluetape-go/web"
```

## Problem details

Use `ProblemError` when an application error is safe to expose. Unknown errors
are mapped to `500 Internal Server Error` without copying their detail into the
response. Cancellation maps to `408 Request Timeout`; a deadline maps to `504
Gateway Timeout`.

```go
type invalidOrderError struct{}

func (invalidOrderError) Error() string { return "order total is invalid" }

func (invalidOrderError) ProblemDetails() web.Problem {
    problem, _ := web.NewProblem(422, "Invalid order", "order total is invalid")
    return problem
}

func handler(w http.ResponseWriter, r *http.Request) {
    if err := web.WriteProblem(w, r, invalidOrderError{}); err != nil {
        // The response writer or problem value rejected the response.
        return
    }
}
```

`WriteProblem` validates status and extension keys, serializes the body before
writing the status, sets the exact `application/problem+json` media type, and
uses the URL's escaped path (without query or fragment) for `instance` when a
request is present, so credential-like query values are never echoed. A nil
request leaves `instance` empty; a nil error or writer returns
`web.ErrInvalidProblem`.

## Request context

`WithRequestContextOnRequest` copies a request and stores a `RequestContext` in
its context. Request and correlation IDs are accepted after single-line,
visible-ASCII, and 256-byte validation. A missing request ID uses the injected
`GenerateID` function or the default UUID v7 generator; a missing correlation ID
reuses the request ID.

Auth subject and W3C `traceparent`/`tracestate` values are read only when the
request-specific `TrustedProxy` predicate returns true. The helper does not
decide authentication or authorization and does not reflect these values into
response headers. The original request and its cancellation context remain
unchanged.

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

`#542` adds HTTP middleware conformance. The Gin adapter is available in
[`web/gin`](gin/README.md) under `#543`; the Echo adapter remains in `#544`.
JWT/JWKS provider behavior remains outside this package.

## Test

```bash
go test -count=1 ./web
go test -race -count=1 ./web
```
