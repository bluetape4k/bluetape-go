# redis/semaphore

`redis/semaphore` provides a bounded Redis semaphore. Each permit is an exact
owner-token member in a Redis sorted set and expires after its configured TTL.

## Usage

```go
import redissem "github.com/bluetape4k/bluetape-go/redis/semaphore"

semaphore, err := redissem.New(client, redissem.Options{
    Key:     "limits:partner-api",
    Permits: 8,
    TTL:     30 * time.Second,
})
if err != nil {
    return err
}

lease, err := semaphore.Acquire(ctx)
if err != nil {
    return err
}
defer func() {
    cleanupCtx, cancel := context.WithTimeout(context.Background(), time.Second)
    defer cancel()
    _, _ = lease.Release(cleanupCtx)
}()
```

`TryAcquire` makes one immediate attempt. `Acquire` retries only
`ErrNotAcquired` with bounded backoff until the caller context is canceled or
deadlined. Each acquire atomically removes expired sorted-set members before
checking capacity. `Release` removes only the exact owner-token member and is
idempotent.

## Operational boundaries

The semaphore does not issue fencing tokens. After a permit TTL expires, work
from the old holder can overlap with a new holder. Keep critical sections
bounded by the TTL or enforce an external resource version/ownership check.
This primitive is not FIFO/fair, does not use Redlock, and has no watchdog
renewal. A caller owns the Redis client, context, retry policy for provider
errors, and cleanup context.

Provider failures are sanitized `btredis.OpError` values. If a mutation was
dispatched but its result is unknown, `btredis.ErrCommitUnknown` is preserved;
reconcile Redis state before assuming a permit was or was not created. Raw
logical keys and owner tokens are not included in error strings.

## Verification

```bash
go test -p 1 -count=1 ./redis/semaphore
go test -p 1 -race -count=1 ./redis/semaphore
go test -count=1 ./redis/semaphore -run Example
```
