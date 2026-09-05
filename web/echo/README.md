# web/echo

[English](README.md) | [한국어](README.ko.md)

`web/echo` is a thin Echo adapter for the framework-neutral `web`, `ratelimit`,
`jwt`, and `resilience` contracts. Echo is isolated to this package; core
packages and the Gin adapter do not import it.

## Install and bootstrap

```go
import (
    "net/http"

    "github.com/bluetape4k/bluetape-go/jwt"
    "github.com/bluetape4k/bluetape-go/ratelimit"
    "github.com/bluetape4k/bluetape-go/web"
    echoadapter "github.com/bluetape4k/bluetape-go/web/echo"
    "github.com/labstack/echo/v4"
    "github.com/labstack/echo/v4/middleware"
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
    server.Use(middleware.Recover(), echoadapter.RequestContext(web.RequestContextOptions{}), rateLimit, authentication)
    server.GET("/orders", echoadapter.WrapResilience(func(c echo.Context) error {
        return c.NoContent(http.StatusNoContent)
    }, echoadapter.ResilienceOptions{}))
    return server, nil
}
```

The compile-checked [`example_test.go`](example_test.go) contains the same
bootstrap and a migration example using `echo.WrapHandler`.

## Middleware contracts

- `RequestContext`, `NewRateLimit`, `NewJWT`, and `WrapResilience` all treat a
  nil downstream handler as a terminal `404 Not Found` response before doing
  request-specific work. They do not call a parser, limiter, or resilience
  policy on this path. Non-nil downstream handlers keep their existing chain and
  response behavior. The rationale and acceptance mapping are recorded in the
  [#692 design](../../docs/superpowers/specs/2026-09-05-issue-692-echo-nil-downstream-design.md).
- `AbortWithProblem` writes `application/problem+json` and does not overwrite a
  committed Echo response. Unknown errors are redacted by the framework-neutral
  `web` mapper.
- `RequestContext` stores validated request context values on a copied request
  and restores the original request pointer when the middleware returns or
  panics. Restricted headers require the caller's `TrustedProxy` predicate;
  duplicate values fail closed instead of selecting an ambiguous identity.
- `NewRateLimit` calls the next Echo handler once only when the limiter allows
  the request. Rejections preserve `Retry-After` and
  `X-RateLimit-Remaining`; backend failures are safe 503 Problems. A custom
  `ErrorHandler` is terminal and caller-owned: it must choose the status/body
  (and redact any backend error it records). The adapter returns nil after the
  callback, so a callback that writes nothing can leave Echo's default 200
  response; write a terminal response or delegate explicitly to the outer
  error handler. If the default Problem writer fails on a rejection, backend
  error, or cancellation response, the adapter keeps the committed response,
  does not retry or overwrite it, and stores a fixed-message observer under
  `DefaultRateLimitWriteErrorContextKey`; read it with `RateLimitWriteError`.
  The observer's `Unwrap` preserves `errors.Is` for the caller-owned cause, but
  its public message is redacted. Custom callbacks keep their existing
  caller-owned write and observation policy. See the [#693 design](../../docs/superpowers/specs/2026-09-05-issue-693-echo-rate-limit-write-observer-design.md)
  for the path-by-path acceptance mapping.
- `NewJWT` accepts exactly one case-insensitive `Bearer <token>` header. It
  rejects duplicate/comma-joined values, controls, whitespace, and tokens over
  8 KiB. `JWTReader` returns only the verified `*jwt.Reader` stored in context.
  The default failure response is a redacted 401. A custom callback receives a
  request copy without the configured authentication header or Authorization;
  its body is `http.NoBody` so callback inspection cannot consume the original
  request. Use `ContextParser` when parser I/O must observe cancellation. If a
  value supplied through the legacy `Parser` field also implements
  `ContextParser`, the adapter automatically uses `ParseContext` with the
  request context. A legacy-only `Parser` remains synchronous: cancellation is
  checked before and after `Parse`, and an in-flight cancellation waits for a
  blocking call to return. Parser cancellation is intentionally reported as the
  same redacted 401 authentication failure as the Gin adapter.
- `WrapResilience` is a route-level wrapper. Request context and replayable
  bodies are cloned for each attempt. A non-replayable body is delivered to the
  first attempt and then fails closed if a retry would be required; committed
  responses and cancellation are also non-retryable. A redacted observer error
  is stored under `DefaultResilienceErrorContextKey` and can be read with
  `ResilienceError` so an outer Echo logger or error handler can record the
  low-cardinality failure without exposing its cause. Echo's context store has
  no public key enumeration API, so adapter-owned key mutations are restored
  after failed attempts; a store mutation followed by a route error is
  non-retryable, while a successful attempt keeps the mutation for the request.
  The observer's `Unwrap` preserves the cause for `errors.Is`; callers must keep
  that cause out of public logs and responses.
  Direct writes through `c.Response().Writer` are tracked as committed and stop
  retries; prefer Echo's response methods so status and size bookkeeping remain
  available. Install `middleware.Recover()` as the outermost middleware for
  panic-to-500 handling.

  The custom `ErrorHandler` callbacks for rate-limit, JWT, and resilience are
  terminal and caller-owned. Each callback must write the final Echo response
  or deliberately hand off to an outer error handler; returning without a
  response can otherwise leave Echo's default 200 status.

### JWT parser cancellation migration

| Configuration | Cancellation and lifecycle contract |
| --- | --- |
| `ContextParser: provider` | The provider receives the request context and owns cooperative cancellation. The adapter waits for `ParseContext` to return and never force-stops provider work. |
| `Parser: provider` where `provider` also implements `ContextParser` | This is a source-compatible migration path. The adapter detects the additional method and calls `ParseContext` with the request context. |
| `Parser: legacyProvider` only | This is the synchronous compatibility path. Cancellation is checked before and after `Parse`; an in-flight cancellation waits for `Parse` to return, then produces the redacted 401 without calling the downstream handler. The adapter does not spawn a goroutine, so it cannot detach or leak a blocked parse call. |

For a parser that performs I/O or can block, implement `ContextParser` and make
the provider return when `ctx.Done()` is closed. Keeping `Parse` for existing
callers is compatible with the automatic upgrade above; new Echo configuration
should set `ContextParser` explicitly so the cancellation requirement is visible.
Legacy `Parser` remains supported for local synchronous parsers and is not
deprecated in `0.21.0`. The rationale and lifecycle tests are recorded in the
[#694 design](../../docs/superpowers/specs/2026-09-05-issue-694-echo-jwt-parser-cancellation-design.md).

## Migration

Keep a legacy `http.Handler` behind Echo with `echo.WrapHandler(handler)`. New
routes can use `WrapResilience` while `RequestContext`, `NewRateLimit`, and
`NewJWT` remain optional middleware. The adapter does not add a global logger,
framework abstraction, or Fiber support.

## Framework boundary differences

The `net/http` core writes directly to `http.ResponseWriter` and returns handler
errors to its caller. Echo handlers return `error`, expose `echo.Context` storage,
and track `Response().Committed`; this adapter stops the Echo chain on rejected
middleware and never overwrites a committed response. The Gin adapter uses Gin's
abort/index and writer state instead, so Gin-specific abort behavior is not
silently reproduced in Echo. Keep framework-native error handling at the outer
Echo boundary and use `echo.WrapHandler` for legacy `http.Handler` routes.

Request-path benchmark evidence is tracked in
[`docs/research/outputs/issue-544`](../../docs/research/outputs/issue-544/README.md).

## Verification

```bash
go test -run '^Example$|^Example_migration$' -count=1 ./web/echo
go test -count=1 ./web/echo
go test -race -count=1 ./web/echo
go vet ./web/echo
```
