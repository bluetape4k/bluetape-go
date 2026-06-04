# ratelimit

`ratelimit` provides a process-local keyed token-bucket limiter and
standard-library HTTP middleware. It is intended for in-process request guards,
tenant throttles, and tests that need deterministic rejection diagnostics.

Use [`ratelimit/redis`](redis/README.md) when multiple processes must share one
bucket.

## Install

```go
import "github.com/bluetape4k/bluetape-go/ratelimit"
```

## Local Token Bucket

```go
limiter, err := ratelimit.New(ratelimit.Options{
    RatePerSecond: 10,
    Burst:         20,
})
if err != nil {
    return err
}

result, err := limiter.Allow(ctx, "tenant:blue", 1)
if err != nil {
    return err
}
if !result.Allowed {
    return fmt.Errorf("retry after %s", result.RetryAfter)
}
```

Rejected attempts are normal results, not errors. Errors are reserved for
invalid input, context cancellation, and backend failures in implementations
such as `ratelimit/redis`.

## HTTP Middleware

```go
handler, err := ratelimit.NewHandler(next, ratelimit.HandlerOptions{
    Limiter: limiter,
    KeyFunc: func(r *http.Request) string {
        return authenticatedTenantID(r)
    },
})
```

The default key function uses `Request.RemoteAddr` only. It does not trust
`X-Forwarded-For`, `Forwarded`, or other proxy headers. Services behind trusted
proxies should provide an explicit `KeyFunc` based on authenticated tenant,
user, or API-key identity.

Default middleware behavior:

- allowed attempts delegate to the wrapped handler;
- rejected attempts return `429 Too Many Requests`;
- rejected attempts set `Retry-After` when a retry delay is known;
- backend/key errors return `503 Service Unavailable`;
- `ErrorHandler` can replace the response policy.

## Operational Boundary

- State is process-local and held in memory.
- `IdleTTL` removes inactive key state; the default is at least one minute and
  at least two full refill windows.
- Requests for more tokens than `Burst` are validation errors because the bucket
  can never satisfy them.
- The limiter is concurrency-safe but does not provide FIFO fairness.

## Tests

```bash
go test -count=1 ./ratelimit
go test -race -count=1 ./ratelimit
```

Stress and cancellation coverage uses
`testing/concurrency.GoroutineStressTester` and
`testing/concurrency.AsyncJobTester`.

## Benchmarks

```bash
make bench-ratelimit
```

Measured paths:

- local allowed path;
- local rejected path;
- HTTP middleware allowed path.

Lower `ns/op` and lower allocations are better. Redis-backed benchmark scope is
kept separate because it depends on external Redis latency and deployment
topology.
