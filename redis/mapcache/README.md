# Redis MapCache

[한국어](README.ko.md)

`github.com/bluetape4k/bluetape-go/redis/mapcache` is a typed Redis map
primitive implemented as independent key-per-entry values. It is durable Redis
storage, not a Java-style distributed `ConcurrentMap` or a local cache.

```go
cache, err := redismap.New(client, redismap.Options[Account]{
    Namespace: "catalog:v1", HashTag: "tenant-a",
    Serializer: serialization.NewJSONSerializer[Account](),
})
if err != nil { return err }
ok, err := cache.CompareAndSet(ctx, "sku:42", old, next, 10*time.Minute)
```

`Client` is caller-owned and contains only `Set`, `SetNX`, `Del`, and `Eval`. The
package does not own client lifecycle, retries, iteration, map-wide
clear, transactions across entries, Redis persistence/eviction/ACL/TLS, or
maxmemory policy.

## Keys, TTL, and operations

Entries use `Namespace:map:{optional-hash-tag}:<logical-key>`. Logical key bytes
are preserved exactly; structural segments are validated and a hash tag is only
a Redis Cluster same-slot hint. Every entry has its own TTL: `0` is persistent,
negative values return `btredis.ErrInvalidTTL`, and positive sub-millisecond
values normalize to 1ms. Other positive values are truncated to milliseconds.

`Get`, `Set`, `SetIfAbsent`, `GetAndDelete`, `CompareAndSet`, and `Delete` have
the same value and context contract as `redisbucket`. `Get` and the Lua
operations use bounded reads; `MaxPayloadBytes` defaults to `1 MiB` and accepts
`1..64 MiB`. Oversized writes are rejected before dispatch, while an oversized
existing value makes `Get`, `GetAndDelete`, or CAS return `ErrPayloadTooLarge`
without decode, replacement, or deletion. The Lua operations are single-entry
atomic operations, so unrelated map entries are not locked. They require Redis
ACL permission for `GETRANGE`, `EXISTS`, and `EVAL` in addition to `SET`,
`SETNX`, and `DEL`.

## Boundaries and errors

Values use the caller serializer; JSON is not implicit. `Error` and wrapped
`btredis.OpError` redact raw keys, payloads, and provider text. Use
`errors.Is`/`errors.As` for package sentinels, provider causes, and
`btredis.ErrCommitUnknown`. Nil/cancelled contexts do not dispatch. A mutation
error or malformed Lua result is commit-unknown; `ErrPayloadTooLarge` retains
the existing value. A success response followed by cancellation returns the
context error without retrying or compensating.

`cache.Memory` is process-local, `cache/redisnear` owns near-cache invalidation,
and `cache/rediscoord` owns stampede/loading coordination. Compose those layers
explicitly when needed; MapCache does not provide eviction, invalidation,
iteration, or stampede prevention.

Normal CI uses deep-copying fakes. The integration test uses the pinned Redis
Testcontainers fixture and verifies independent entry expiry, empty and
oversized values, concurrent CAS, and bounded Lua operations; execute it
serially with other Docker-backed tests.
