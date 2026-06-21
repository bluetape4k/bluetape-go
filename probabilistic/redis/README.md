# probabilistic/redis

English | [한국어](README.ko.md)

`probabilistic/redis` provides Redis-backed shared Bloom filters. It keeps Bloom
configuration immutable in Redis metadata, validates that metadata through Lua
scripts before every read or mutation, and stores shared bits in a Redis bitmap
string.

![probabilistic redis bloom runtime](../../docs/images/readme-diagrams/probabilistic-redis-bloom-runtime.png)

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

## Redis State

Redis Bloom uses one Cluster-safe hash-tagged key pair per namespace.

| Key suffix | Type | Purpose |
|---|---|---|
| `:bits` | bitmap string | Bloom bits read and written with `GETBIT`, `SETBIT`, `BITCOUNT`, and `STRLEN`. |
| `:config` | hash | Immutable metadata checked before every shared-state operation. |

Namespaces must be stable operational identifiers. Do not place raw user IDs,
emails, tokens, secrets, passwords, credentials, or API keys in namespaces.

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

## Operational Boundaries

- Redis persistence, eviction policy, TLS, AUTH, ACLs, and backup policy are
  caller-owned.
- Prefer `noeviction` or reserved memory for shared filters. Evicting `:bits`
  while `:config` remains can violate the no-false-negative expectation until
  the namespace is rebuilt.
- Treat `Clear` as an administrative action. Recovery from accidental deletion
  should rebuild into a new namespace, verify readers, and retire old keys only
  after the new namespace is accepted.

## Test

```bash
go test -count=1 ./probabilistic/redis
```
