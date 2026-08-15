# web/gin

[English](README.md) | [한국어](README.ko.md)

`web/gin` is a thin Gin adapter for the framework-neutral `web`, `ratelimit`,
`jwt`, and `resilience` contracts. Gin is intentionally isolated to this
package; the core packages remain usable from `net/http` and other adapters.

## Install and import

```go
import ginadapter "github.com/bluetape4k/bluetape-go/web/gin"
```

The adapter uses Gin `v1.12.0`. Do not import Gin from core packages.

## Bootstrap

`ExampleBootstrap` and `ExampleMigration` in
[`example_test.go`](example_test.go) are compile-checked fixtures. The normal
composition order is recovery, request context, rate limit, authentication,
and a route-level resilience wrapper:

```go
func buildRouter() (*gin.Engine, error) {
provider, err := jwt.NewFixedHMACProvider(jwt.HS256, []byte("0123456789abcdef0123456789abcdef"))
if err != nil { return nil, err }
limiter, err := ratelimit.New(ratelimit.Options{RatePerSecond: 10, Burst: 10})
if err != nil { return nil, err }
rateLimit, err := ginadapter.NewRateLimit(ginadapter.RateLimitOptions{Limiter: limiter})
if err != nil { return nil, err }
authentication, err := ginadapter.NewJWT(ginadapter.JWTOptions{Parser: provider})
if err != nil { return nil, err }
router := gin.New()
router.Use(gin.Recovery(), ginadapter.RequestContext(web.RequestContextOptions{}), rateLimit, authentication)
handler := func(c *gin.Context) { c.Status(http.StatusNoContent) }
router.GET("/orders", ginadapter.WrapResilience(handler, ginadapter.ResilienceOptions{}))
return router, nil
}
```

`RequestContext` stores the shared `web.RequestContext` in a request copy and
restores the original request pointer on return or panic. Restricted headers
are accepted only when the caller's `TrustedProxy` predicate returns true.

## Middleware contracts

- `AbortWithProblem` writes RFC 9457 `application/problem+json` and aborts the
  Gin chain. Unknown errors are redacted; a committed writer is never
  overwritten.
- `NewRateLimit` bridges the existing limiter. Allowed requests call the next
  Gin handler once. Rejections preserve `Retry-After` and
  `X-RateLimit-Remaining`; backend errors are redacted 503 Problems by
  default.
- `NewJWT` accepts exactly one case-insensitive `Bearer <token>` header. It
  rejects duplicate/comma-joined values, controls, whitespace, and tokens over
  8 KiB. Only the verified `*jwt.Reader` is stored; `JWTReader` retrieves it.
  Error callbacks receive a request copy without the Authorization header and
  an `AuthenticationError` without parser details or the raw token.
- `WrapResilience` is a route-level wrapper. Retry is safe only before a
  response is committed and when a request body can be replayed with
  `GetBody`. Committed responses and non-replayable bodies become
  `resilience.NonRetryable`; the writer is never rolled back or overwritten.

## Operational runbook

1. **Preflight** — run the example smoke test and package checks before a
   rollout:

   ```bash
   go test -run 'Example(Bootstrap|Migration)$' ./web/gin
   go test ./web/gin
   go test -race ./web/gin
   ```

   Request-context header-name mistakes are surfaced as a 400 on the first
   request. There is no separate startup-validation API; the examples are the
   deterministic readiness smoke test.
2. **Canary (5 minutes)** — route a small percentage of traffic through the Gin
   adapter. During the full window, readiness must remain `200` and a normal
   request must remain `2xx`.

   ```bash
   curl -i https://service.example/readyz
   # expected: HTTP/2 200 and the documented readiness body
   curl -i -H 'X-Auth-Subject: subject-1' https://service.example/orders
   # expected for a trusted request: 2xx and no raw error/token fields
   curl -i -H 'X-Auth-Subject: spoofed' https://service.example/orders
   # expected for an untrusted peer: the subject is ignored or a safe 4xx
   curl -i -H 'Authorization: Bearer <test-token>' https://service.example/orders
   # expected: 2xx for a valid token; missing/invalid token is a redacted 401
   curl -i https://service.example/orders
   # expected: 401 application/problem+json without token/parser detail
   ```

3. **Observe** — callbacks emit exactly `adapter`, `kind`, `status`,
   `committed`, `request_id`, and `duration`. Record `c.IsAborted()`,
   `c.Writer.Written()`, and `AuthenticationError.Kind` after the relevant
   middleware decision. Never record Authorization headers, raw tokens, or raw
   parser/backend errors. The callback request copy is already redacted.

   | callback | timing | fields |
   | --- | --- | --- |
   | rate-limit error handler | after abort, before response is returned | adapter, kind, status, committed, request_id, duration |
   | JWT error handler | after abort and sanitized request copy | adapter, kind, status, committed, request_id, duration |
   | resilience error handler | after policy result, before an uncommitted Problem is written | adapter, kind, status, committed, request_id, duration |

4. **Rollback** — if a raw token/parser error is observed, or the canary
   breaches its error budget, remove the adapter middleware and restore the
   previous `net/http` path. Do not retry a route after it has written a
   response.
5. **Recovery** — keep `gin.Recovery()` outside the adapter chain. Restore
   traffic only after readiness is `200`, normal requests are `2xx`, and the
   5xx/429/401 ratios have returned to the pre-canary baseline. Re-run the
   preflight and all probes before expanding traffic.

## Migration shape

Before: register the existing `http.Handler` directly on the Gin router or
serve it from the `net/http` path. After: keep the handler body unchanged and
wrap the route with `WrapResilience`, while adding `RequestContext`, `NewJWT`,
and `NewRateLimit` in the bootstrap order above. `ExampleMigration` is the
compile-checked reference for this before/after boundary.

## Verification

```bash
go test -count=1 ./web/gin
go test -race -count=1 ./web/gin
go vet ./web/gin
```
