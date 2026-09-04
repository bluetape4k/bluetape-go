# Redis Bucket

[한국어](README.ko.md)

`github.com/bluetape4k/bluetape-go/redis/bucket` is a small durable Redis
single-key primitive. It stores values through a caller-provided
`serialization.Serializer[V]`; it does not assume JSON or RedisJSON.

```go
bucket, err := redisbucket.New(client, redisbucket.Options[Account]{
    Namespace: "catalog:v1", HashTag: "tenant-a",
    Serializer: serialization.NewJSONSerializer[Account](),
})
if err != nil { return err }
ok, err := bucket.SetIfAbsent(ctx, "sku:42", account, 5*time.Minute)
```

The client is a narrow caller-owned interface (`Get`, `Set`, `SetNX`, `Del`,
and `Eval`). The package does not create or close clients, add retries, set
Redis persistence/eviction/ACL/TLS/maxmemory policy, or own metrics. Supply
deadlines and retry decisions at the call boundary.

## Operations

`Get` returns `(zero, false, nil)` for a missing or expired key and decodes a
present payload only after checking the response context. `Set` and
`SetIfAbsent` use Redis `SET`/`SETNX`. `GetAndDelete` and `CompareAndSet` use
single-key Lua scripts, so a read/delete or expected/replacement decision is
atomic. `Delete` is idempotent and ignores the deleted count.

Keys use `Namespace:bucket:{optional-hash-tag}:<logical-key>`. Structural
segments are validated by the shared Redis `KeyBuilder`; the logical key is
preserved byte-for-byte, including spaces, braces, and colons. A hash tag is a
Redis Cluster same-slot hint, not a tenant or authorization boundary.

TTL `0` means persistent, negative values return `btredis.ErrInvalidTTL`, and
positive sub-millisecond values are sent as 1ms. Other positive values are
truncated to whole milliseconds. Redis persistence and eviction remain
operator settings.

## Errors and cancellation

`Error` and the wrapped `btredis.OpError` expose only operation labels and a
stable redacted key ID in `Error()`/`%+v`. Use `errors.Is`/`errors.As` to inspect
`ErrSerialization`, `ErrInvalidPayload`, `ErrMalformedResult`, the provider
cause, or `btredis.ErrCommitUnknown`. A mutation command error or malformed Lua
result is commit-unknown because the command may have changed Redis. A
successful response followed by caller cancellation returns the context error
without a compensating write.

Nil or already-cancelled contexts are rejected before dispatch. The package
owns no goroutines or cleanup workers. Compose `cache.Memory` or
`cache/redisnear` for local/near-cache behavior and `cache/rediscoord` for
stampede coordination explicitly; this package only stores durable Redis
entries.

Normal CI uses mutex-safe fakes. The integration test starts the pinned Redis
Testcontainers fixture and verifies expiry, entry operations, and Lua
atomicity; run it serially with other Docker-backed suites.
