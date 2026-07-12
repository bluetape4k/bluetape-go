# lock/redis

[English](README.md) | [한국어](README.ko.md)

`lock/redis` provides a small single-Redis-instance owner-token lock for
coordination work that needs TTL cleanup and owner-safe unlock semantics.

## Diagram

![Redis lock owner-token lifecycle](../../docs/images/readme-diagrams/redis-lock-owner-token-lifecycle.png)

![Redis lock owner-token sequence](../../docs/images/readme-diagrams/redis-lock-owner-token-sequence.png)

## Import

```go
import (
    redislock "github.com/bluetape4k/bluetape-go/lock/redis"
    btredis "github.com/bluetape4k/bluetape-go/redis"
)
```

## Usage

```go
mutex, err := redislock.New(client, redislock.Options{
    Key: "locks:billing-rollup",
    TTL: 30 * time.Second,
})
if err != nil {
    return err
}

lockCtx, lockCancel := context.WithTimeout(ctx, 5*time.Second)
defer lockCancel()

cleanupLease := func(lease *redislock.Lease) error {
    var lastErr error
    for range 2 {
        cleanupCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
        _, cleanupErr := lease.Unlock(cleanupCtx)
        cancel()
        if cleanupErr == nil { // released or confirmed absent after a lost reply
            return nil
        }
        lastErr = cleanupErr
        if !errors.Is(cleanupErr, btredis.ErrCommitUnknown) {
            break
        }
    }
    return lastErr // TTL is the final fallback
}

lease, err := mutex.TryLock(lockCtx)
if lease != nil && err != nil {
    cleanupErr := cleanupLease(lease)
    return errors.Join(err, cleanupErr) // classify err before bare context errors
}
if errors.Is(err, redislock.ErrNotAcquired) {
    return nil
}
if err != nil {
    return err
}
defer func() {
    _ = cleanupLease(lease)
}()
```

## Behavior

- `TryLock` performs one non-blocking acquire attempt with Redis `SET NX` and
  a TTL.
- `Lease.Unlock` deletes the Redis key only when the stored token still matches
  the lease token.
- A custom token may be supplied through `Options.Token`; otherwise each
  acquire generates a random owner token.
- Context cancellation is preserved for Redis commands.
- Redis command failures preserve their cause for `errors.Is` and `errors.As`,
  while diagnostic messages redact raw lock keys and owner tokens.
- Cleanup may use a fresh context after request cancellation, but it should be
  bounded with an explicit timeout.

## Operational Boundaries

- This is not Redlock quorum and does not provide fencing tokens.
- TTL renewal and blocking retry loops are intentionally not included.
- Choose a TTL that safely covers the protected operation or compose renewal at
  a higher layer.

## Test

```bash
go test -count=1 ./lock/redis
```

## Conformance And Commit-Unknown Recovery

`locktest.Run` validates the Redis provider at real SetNX/Eval boundaries. Check
`redis.ErrCommitUnknown` before context errors. If `TryLock` returns `lease != nil`
with an error, immediately attempt bounded cleanup. A lost `Unlock` returns false
and the typed error; retry the same lease callback. Compare-delete protects a
replacement owner, and TTL is the final fallback. Custom nonblank token bytes,
including surrounding whitespace, are no longer trimmed.
