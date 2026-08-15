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
router := gin.New()
router.Use(gin.Recovery())
router.Use(ginadapter.RequestContext(web.RequestContextOptions{}))
router.Use(rateLimit, authentication)
router.GET("/orders", ginadapter.WrapResilience(handler, ginadapter.ResilienceOptions{}))
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
2. **Canary** — route a small percentage of traffic through the Gin adapter.
   Probe one trusted and one untrusted request-context header, a valid and
   missing JWT, and an allowed and rejected rate-limit request.
3. **Observe** — record status, request ID, `c.IsAborted()`, and
   `c.Writer.Written()` in callbacks. Record `AuthenticationError.Kind`, not
   Authorization headers, raw tokens, or raw parser/backend errors.
4. **Rollback** — remove the adapter middleware from the canary route and
   restore the previous `net/http` path. Do not retry a route after it has
   written a response.
5. **Recovery** — keep `gin.Recovery()` outside the adapter chain. Investigate
   401/429/503 Problem responses using the safe status and kind fields, then
   replay the preflight and canary probes before restoring traffic.

## Verification

```bash
go test -count=1 ./web/gin
go test -race -count=1 ./web/gin
go vet ./web/gin
```
