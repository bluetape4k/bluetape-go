# redis/lock

`redis/lock` provides a single-Redis-instance fenced lock. Each successful
acquisition receives a persistent, monotonically increasing fencing token.

## Usage

```go
import redislock "github.com/bluetape4k/bluetape-go/redis/lock"

lock, err := redislock.New(client, redislock.Options{
    Key: "locks:billing-rollup",
    TTL: 30 * time.Second,
})
if err != nil {
    return err
}

lease, err := lock.Acquire(ctx)
if err != nil {
    return err
}
defer func() {
    cleanupCtx, cancel := context.WithTimeout(context.Background(), time.Second)
    defer cancel()
    _, _ = lease.Release(cleanupCtx)
}()

fencingToken := lease.FencingToken()
```

`TryAcquire` makes one immediate attempt. `Acquire` retries only
`ErrNotAcquired` with bounded backoff until the caller context is canceled or
deadlined. `Release` is owner-safe and idempotent: an expired or replaced lease
returns `(false, nil)`.

## Fencing contract

`FencingToken` protects an external resource only when that resource stores the
greatest accepted token and rejects older tokens. The lock itself cannot stop
work that continues after its TTL expires. A stale holder and a fresh holder
can overlap, so choose a TTL that bounds the critical section and enforce token
ordering at the resource boundary.

## Errors and cleanup

Provider failures are returned as sanitized `btredis.OpError` values and keep
their cause available to `errors.Is`/`errors.As`. A mutation error after Redis
dispatch also includes `btredis.ErrCommitUnknown`; inspect Redis state before
assuming whether the acquire or release committed. Use a fresh, bounded
cleanup context after a request context is canceled.

The package hashes caller-owned logical keys for Redis storage, cluster hash
tags, and diagnostics. Raw keys and owner tokens are not included in provider
error strings. This primitive is not Redlock, does not provide FIFO fairness,
and intentionally has no watchdog renewal.

## Verification

```bash
go test -p 1 -count=1 ./redis/lock
go test -p 1 -race -count=1 ./redis/lock
go test -count=1 ./redis/lock -run Example
```
