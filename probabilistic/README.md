# probabilistic

[English](README.md) | [한국어](README.ko.md)

`probabilistic` provides first-party probabilistic data structures. The current
package surface is an in-memory Bloom filter.

## Import

```go
import "github.com/bluetape4k/bluetape-go/probabilistic"
```

## Bloom Filter

```go
cfg, err := probabilistic.NewConfig(1_000_000, 0.01)
if err != nil {
    return err
}

filter, err := probabilistic.NewStringBloomFilter(cfg)
if err != nil {
    return err
}

filter.Put("user:42")
if filter.MightContain("user:42") {
    // The value may be present.
}
```

For non-string or non-`[]byte` values, provide an explicit hasher with a stable
compatibility key:

```go
hasher, err := probabilistic.NewHasher("int-decimal", func(v int) []byte {
    return []byte(strconv.Itoa(v))
})
```

Package-created filters can be merged only when their config and hasher key match.
Custom hasher functions must be deterministic and goroutine-safe. Callers own
the stability of the compatibility key: two filters with the same key are treated
as merge-compatible.

## Behavior

- `MightContain` returning `false` means the value is definitely absent.
- `MightContain` returning `true` means the value may be present; false
  positives are possible.
- Successful inserts that are not followed by `Clear` should not produce false
  negatives.
- `Put` returns whether at least one bit changed. A `false` return does not
  prove the value already existed.
- Deletion is unsupported.
- The implementation is goroutine-safe for concurrent `Put`, `MightContain`,
  `PutAll`, `Clear`, and metadata reads when the hasher is goroutine-safe.
- The package has no context-aware I/O or background job boundary.

## Errors

Sentinel errors support `errors.Is`:

- `ErrInvalidConfig`
- `ErrIncompatibleFilter`
- `ErrNilFilter`
- `ErrNilHasher`
- `ErrEmptyHasherKey`

## Deferred Scope

Redis-backed Bloom, Cuckoo, and HyperLogLog support is deferred to
[#182](https://github.com/bluetape4k/bluetape-go/issues/182). This package does
not use Redis, RedisBloom, or Testcontainers.

## Test

```bash
go test -count=1 ./probabilistic
go test -race -count=1 ./probabilistic
```
