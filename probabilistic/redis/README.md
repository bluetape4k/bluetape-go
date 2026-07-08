# probabilistic/redis

English | [한국어](README.ko.md)

`probabilistic/redis` provides Redis-backed shared Bloom filters and
HyperLogLog cardinality estimates. Bloom filters keep configuration immutable in
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

## Test

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
