# ratelimit

[English](README.md) | [한국어](README.ko.md)

`ratelimit` provides a process-local keyed token-bucket limiter and
standard-library HTTP middleware. It is intended for in-process request guards,
tenant throttles, and tests that need deterministic rejection diagnostics.

Choose a provider by ownership and traffic shape:

| Provider | Use when | Boundary |
|---|---|---|
| Local `ratelimit` | One process owns the quota. | Fast in-memory state; not shared across processes. |
| [`ratelimit/redis`](redis/README.md) | Multiple processes need a shared low-latency quota. | Caller-owned Redis; atomic Lua operation. |
| [`ratelimit/sql`](sql/README.md) | A moderate-QPS, database-only deployment already shares PostgreSQL. | Caller-owned PostgreSQL schema, pool, and cleanup; not a Redis replacement for high-QPS traffic. |

## Diagram

![ratelimit local runtime flow](../docs/images/readme-diagrams/ratelimit-local-runtime-flow.png)

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
such as `ratelimit/redis` and `ratelimit/sql`.

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

## Benchmark Results

These are local smoke numbers, not production capacity rankings. The run used
macOS arm64 on Apple M4 Pro. Lower `ns/op`, `B/op`, and `allocs/op` are better.

![ratelimit benchmark latency](../docs/images/readme-charts/ratelimit-benchmark-latency.png)

| Benchmark | ns/op | B/op | allocs/op |
|---|---:|---:|---:|
| `BenchmarkTokenBucketAllowAllowed` | 116.4 | 0 | 0 |
| `BenchmarkTokenBucketAllowRejected` | 76.76 | 0 | 0 |
| `BenchmarkHandlerAllowed` | 51.26 | 160 | 3 |

## Provider Conformance

`ratelimit/ratelimittest.Run` applies the same burst, refill, cancellation, and
exact-admission contract to local, Redis, and SQL providers. Distributed
providers expose redacted failures through `ratelimit.OperationError`. If
`errors.Is(err, ratelimit.ErrCommitUnknown)` is true, discard the zero result and
do not replay automatically because one debit may have committed.

Local, Redis, and SQL quota state is not shared. Simultaneous mixed-provider
serving can grant multiple full bursts and is prohibited. A safe canary uses an
independent namespace and an independent cohort. For cutover or rollback,
quiesce the old provider and wait a conservative full-refill window before
activating exactly one new provider, or record an approved extra-burst budget
for the overlap.

### Cutover and rollback runbook

1. Record the source/target provider, policy, namespace, cohort, and rollback
   destination. Separate namespaces do not separate users: route each cohort
   to exactly one provider. A new local instance is also a new full bucket.
2. Stop admitting requests to the old route, cancel queued requests, and wait
   for all in-flight `Allow` calls and admitted application work to finish.
   Cancellation alone does not prove that an already dispatched debit stopped.
3. If an operation returns `ErrCommitUnknown`, count its requested tokens as
   possibly spent. Do not replay it, fall back to another provider, or issue a
   refund. Redis Lua debit is not idempotent. Use a dedicated caller-owned Redis
   standalone client with `MaxRetries: -1` for this limiter.
   The go-redis default retries transport errors and may debit again before
   returning to this adapter; `MaxRetries: 0` selects that default rather than
   disabling retries. Do not install retrying middleware around `Allow`.
   An application request ID is not a debit deduplication key.
   Cluster clients also have a separate `MaxRedirects` retry loop: disabling
   `MaxRetries` alone is insufficient. Use `MaxRedirects: -1` as well and handle
   routing errors without blindly replaying a debit. Cluster transport behavior
   is not exercised by the standalone fixture below.
4. Without an approved extra-burst budget, wait at least the larger of the old
   and new full-refill durations (`Burst / RatePerSecond`), rounded up, measured
   from the last possibly committed debit after draining. Add a documented
   operational margin. Do not use key deletion, TTL expiration, or a process
   restart to reset quota. This wait is an operational cutover rule, not a
   mechanism for transferring quota or enforcing a global provider-spanning limit.
5. Activate the target route for its cohort. If overlap is explicitly approved,
   record its deadline, affected cohort, maximum extra burst, ongoing refill
   allowance, monitoring, and abort condition. Two fresh buckets can admit the
   sum of their bursts; a time-unbounded overlap has no fixed extra-burst bound.
6. Roll back using the same drain/wait procedure. Preserve the old namespace
   and local object when possible; recreating them grants fresh quota. Do not
   replay requests whose commit result was unknown before switching back.

Observe admission/rejection totals, `ErrCommitUnknown`, in-flight calls,
drain duration, active routing generation, and overlap-budget consumption.
Use fixed provider/operation labels and an explicit observer; raw user keys,
namespaces, and backend diagnostics must not become metric labels or logs.
Stop the rollout if two routes serve the same cohort unexpectedly, unknown
results increase, or the approved budget is exhausted.

The real-provider tests exercise all nine source/target combinations,
pre-canceled requests, separate cohorts, preserved old state, and a lost Redis
response after a successful Lua debit. The transport fixture injects `io.EOF`
after reading the real response: retries disabled yields one debit, while a
default-client control demonstrates two debits. These tests do not deploy a
routing controller or prove the caller's drain protocol. Before a real rollout,
manually record drain completion, the calculated refill wait and its elapsed
time, overlap-budget expiry, and a same-cohort rollback rehearsal. Run
Docker-backed packages serially:

```bash
go test -p 1 -count=1 -run '^TestProviderCutover' ./ratelimit/sql
go test -race -p 1 -count=1 -run '^TestProviderCutover' ./ratelimit/sql
```
