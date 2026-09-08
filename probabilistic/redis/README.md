# probabilistic/redis

English | [한국어](README.ko.md)

`probabilistic/redis` provides Redis-backed shared Bloom filters and
HyperLogLog cardinality estimates, plus module-backed Cuckoo filters with deletion.
Bloom filters keep configuration immutable in
Redis metadata, validate that metadata through Lua scripts before every read or
mutation, and store shared bits in a Redis bitmap string. HyperLogLog uses core
Redis `PFADD`, `PFCOUNT`, and `PFMERGE` commands.

![probabilistic redis runtime map](../../docs/images/readme-diagrams/probabilistic-redis-bloom-runtime.png)

## Import

```go
import redisbloom "github.com/bluetape4k/bluetape-go/probabilistic/redis"
```

## Usage

```go
cfg, err := probabilistic.NewConfig(1_000_000, 0.01)
if err != nil {
    return err
}

filter, err := redisbloom.NewStringBloomFilter(ctx, redisClient, "auth:tenant-a:login-attempts", cfg)
if err != nil {
    return err
}

changed, err := filter.Put(ctx, "candidate-key")
if err != nil {
    return err
}
if !changed {
    // All hashed bits were already set. This is not duplicate certainty.
}

mayExist, err := filter.MightContain(ctx, "candidate-key")
```

Use `NewBytesBloomFilter` for `[]byte` values or `NewBloomFilter` with an
explicit deterministic `probabilistic.Hasher[T]` for custom value types.

## HyperLogLog

Use HyperLogLog when callers need approximate distinct counts, not membership
checks:

```go
hll, err := redisbloom.NewStringHyperLogLog(redisClient, "auth:tenant-a:active-users")
if err != nil {
    return err
}

changed, err := hll.Add(ctx, "user-1", "user-2")
if err != nil {
    return err
}

estimate, err := hll.Count(ctx)
```

`NewBytesHyperLogLog` supports `[]byte` values. `NewHyperLogLog` accepts a
custom deterministic `probabilistic.Hasher[T]`. Values are transformed through
the hasher and then stored as SHA-256 hex digests, so Redis receives stable
identifiers rather than raw caller values.

`Merge(ctx, sourceNamespaces...)` merges source HLL namespaces into the receiver
namespace with `PFMERGE` while preserving the receiver's existing estimate.

## Workshop Adoption

Workshop examples demonstrate application-level use outside this package:
[`probabilistic-dedupe-admission`](https://github.com/bluetape4k/bluetape-go-workshop/tree/develop/examples/probabilistic-dedupe-admission)
uses probabilistic admission control, and
[`shared-redis-bloom-admission`](https://github.com/bluetape4k/bluetape-go-workshop/tree/develop/examples/shared-redis-bloom-admission)
uses this Redis-backed Bloom surface. Redis HyperLogLog adoption is tracked in
workshop issue
[#151](https://github.com/bluetape4k/bluetape-go-workshop/issues/151).

## Redis State

Redis Bloom uses one Cluster-safe hash-tagged key pair per namespace.

| Key suffix | Type | Purpose |
|---|---|---|
| `:bits` | bitmap string | Bloom bits read and written with `GETBIT`, `SETBIT`, `BITCOUNT`, and `STRLEN`. |
| `:config` | hash | Immutable metadata checked before every shared-state operation. |

Redis HyperLogLog uses one Cluster-safe hash-tagged key per namespace:

```text
bluetape:probabilistic:hll:v1:{namespace}
```

Namespaces must be stable operational identifiers. Do not place raw user IDs,
emails, tokens, secrets, passwords, credentials, or API keys in namespaces.

## Redis Assumptions

- The current Bloom and HyperLogLog surfaces use ordinary Redis commands only.
  No RedisBloom module is required for `NewStringBloomFilter`,
  `NewBytesBloomFilter`, `NewHyperLogLog`, `NewStringHyperLogLog`, or
  `NewBytesHyperLogLog`.
- The Testcontainers suite runs against Redis `redis:7.4-alpine`; production
  deployments should provide the same core command families used here: Lua
  script execution, hash/string bitmap commands, and HyperLogLog commands.
- RedisBloom module commands such as `CF.ADD`, `CF.EXISTS`, and other `CF*`
  Cuckoo operations are intentionally not part of this package yet. They remain
  follow-up scope until module availability, ACL, persistence, and
  Testcontainers coverage are explicit.
- Bloom capacity is fixed by `probabilistic.Config`: `ExpectedInsertions`,
  `FalsePositiveProbability`, bit size, hash count, and the hasher key become
  immutable Redis metadata for the namespace. Changing those values requires a
  new namespace and rebuild.
- HyperLogLog capacity and error rate are Redis-owned. Callers choose it for
  approximate distinct counts, not membership checks or duplicate certainty.

## Behavior

- `MightContain(ctx, value) == false` means the value is definitely absent.
- `MightContain(ctx, value) == true` means the value may be present; false
  positives are possible.
- `Put(ctx, value) == false` means every hashed bit was already set. It does not
  prove the exact value had already been inserted.
- `Clear(ctx)` deletes shared bitmap state while preserving config metadata.
- `BitCount`, `IsEmpty`, `ApproximateElementCount`, and `ExpectedFPP` read
  operational metadata from the shared bitmap.
- Config or hasher mismatch is treated as an error before reading or mutating
  shared state.
- HyperLogLog `Count(ctx)` is approximate cardinality. It does not answer
  whether a value was inserted.
- HyperLogLog `Add(ctx, values...)` reports Redis `PFADD` state changes, not
  duplicate certainty.

## Operational Boundaries

- Redis persistence, eviction policy, TLS, AUTH, ACLs, and backup policy are
  caller-owned.
- Prefer `noeviction` or reserved memory for shared filters. Evicting `:bits`
  while `:config` remains can violate the no-false-negative expectation until
  the namespace is rebuilt.
- Treat `Clear` as an administrative action. Recovery from accidental deletion
  should rebuild into a new namespace, verify readers, and retire old keys only
  after the new namespace is accepted.
- HyperLogLog keys are ordinary Redis keys. Caller-owned persistence, eviction,
  ACL, and backup policy still apply.

Diagnostic checks normally start with the concrete key family involved:

```text
HGETALL bluetape:probabilistic:bloom:v1:{namespace}:config
STRLEN  bluetape:probabilistic:bloom:v1:{namespace}:bits
BITCOUNT bluetape:probabilistic:bloom:v1:{namespace}:bits
PFCOUNT bluetape:probabilistic:hll:v1:{namespace}
EXISTS  bluetape:probabilistic:hll:v1:{namespace}
PTTL    bluetape:probabilistic:hll:v1:{namespace}
```

## Cuckoo filters

`NewCuckoo` adds a separate `bluetape:probabilistic:cuckoo:v1` key family.
It requires a CF-capable Redis server; a plain Redis 7.4 client or the presence
of go-redis CF methods does not prove server support. No new Go dependency is required.

```go
client := redis.NewClient(&redis.Options{
    Addr: "localhost:6379", MaxRetries: -1,
    DialTimeout: 3 * time.Second, ReadTimeout: 3 * time.Second,
    WriteTimeout: 3 * time.Second, ContextTimeoutEnabled: true,
})
defer client.Close()
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()
filter, err := redisbloom.NewCuckoo(redisbloom.CuckooOptions{
    Client: client, Namespace: "tenant-a:members",
})
if err != nil { return err }
if err = filter.Reserve(ctx, redisbloom.CuckooReserveOptions{
    Capacity: 1024, Expansion: 0,
}); err != nil { return err }
if err = filter.Add(ctx, "item"); err != nil { return err }
mayExist, err := filter.Exists(ctx, "item")
```

- `Reserve` never replaces an existing key. Capacity is 4..2^30 and at least
  twice BucketSize. BucketSize zero means 2; MaxIterations zero means 20.
  Expansion zero **disables growth**; the maximum is 32768. The server rounds
  capacity/positive expansion to powers of two and may fill before capacity.
  These are trusted operator settings, not safe tenant quotas: separately bound
  memory, CPU, and namespace count; never forward untrusted request options.
- `Add` uses `CF.INSERT NOCREATE` with one item: no implicit creation, batch,
  hashing, or adapter retry. Items preserve bytes, including empty strings and
  NUL, up to 64 KiB. Namespace follows existing 1..128-byte ASCII validation.
- `Exists` is approximate; false also covers missing and wrong-type keys.
  It is not a health check. `Count` may overestimate even repeated identical
  items in tiny filters. Neither result is an exact ownership ledger.
- `Delete` removes one known successful insertion only. Never infer deletion
  authority from `Exists`, count unknown mutations as successful, or exceed
  known insertions. Deleting never-added items can remove another fingerprint
  and cause false negatives. Do not use this filter for authorization or fencing.
- `ErrCuckooUnsupported` identifies a definite unknown CF command, not ACL or
  transport failure. Middleware-wrapped or joined errors are conservatively
  not classified as unsupported; mutations retain commit-unknown instead.
  `ErrCuckooReply` rejects malformed responses and
  `ErrCuckooFull` reports insertion rejection due to space or expansion-resource
  exhaustion; the reply cannot distinguish these causes. Dispatched mutation failures or
  cancellation may include `redis.ErrCommitUnknown`; do not automatically replay.
- Every mutation path must disable client, middleware, and caller retry. For
  Cluster, also bound redirect replay (`MaxRedirects: -1`). The narrow client
  interface cannot enforce settings. Cluster transport is not tested here.
- Client construction/Close, credentials, TLS, deadlines, IO timeouts, quotas,
  and observation are caller-owned. The adapter adds no logger or goroutine.
  Top-level operation errors redact payload/provider text; unwrapped causes
  may contain secrets and must not be logged directly. The endpoint is trusted;
  the adapter cannot bound a hostile server's allocation inside RESP decoding.
- Nil context is invalid. Pre-cancellation sends no command; cancellation after
  a response returns no successful result. A non-cooperative client is not
  force-closed or detached into a goroutine.

The fake-only `ExampleNewCuckoo` demonstrates a successful-insertion ledger
and an unknown response without network access. To run the module-positive
fixture (Redis 8.8.1 pinned by digest), separately from other Docker suites:

```bash
BLUETAPE_CUCKOO_INTEGRATION=1 go test -p 1 -count=1 -timeout=5m ./probabilistic/redis -run '^TestCuckoo'
BLUETAPE_CUCKOO_INTEGRATION=1 go test -race -p 1 -count=1 -timeout=5m ./probabilistic/redis -run '^TestCuckoo'
```

Default tests cover fake CF boundaries and plain Redis unsupported behavior;
an opt-in test not run is not positive capability evidence.

## Existing Bloom and HyperLogLog tests

The package tests start Redis `redis:7.4-alpine` through Testcontainers for Go.
Container startup is bounded to 90 seconds, readiness pings use short bounded
contexts inside a 10 second window, and live Redis operations plus cleanup use
package-local operation timeouts.

Coverage includes:

- Bloom filter configuration reuse, mismatch/corrupt metadata handling, clear,
  false-negative protection, Lua command shape, cancellation, and concurrent
  calls.
- HyperLogLog add/count/merge, byte and custom hasher values, invalid options,
  cancelled contexts, redacted Redis errors, raw value non-disclosure, and
  concurrent calls.
- `GoroutineStressTester` and `AsyncJobTester` coverage for concurrent Redis
  operations and cancellation behavior.

```bash
go test -count=1 ./probabilistic/redis
go test -race -count=1 ./probabilistic/redis
```

Run this package serially with other Testcontainers packages. The repository
`make test` and `make race` targets already use `-p 1` for that reason.
