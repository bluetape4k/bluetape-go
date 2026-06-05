# cache

`cache` defines framework-neutral cache contracts and a process-local TTL
implementation. It is the base package for local cache-aside code and for Redis
coordination wrappers that need a `LoadingCache`.

## Import

```go
import "github.com/bluetape4k/bluetape-go/cache"
```

## Usage

```go
localCache := cache.NewMemory[string, string]()

value, err := localCache.GetOrLoad(ctx, "catalog", time.Minute,
    func(ctx context.Context, key string) (string, error) {
        return loadCatalogValue(ctx, key)
    },
)
if err != nil {
    return err
}
```

## Behavior

- `ErrCacheMiss` is returned for missing or expired values and supports
  `errors.Is`.
- TTL `0` stores values without expiration; negative TTL values are rejected.
- `Memory.GetOrLoad` collapses same-key in-flight loader calls inside one cache
  instance.
- Cross-process invalidation and stampede protection are intentionally handled
  by `cache/redisnear` and `cache/rediscoord`.

## Test

```bash
go test -count=1 ./cache
```

## Benchmarks

```bash
go test -run '^$' -bench '^BenchmarkMemory' -benchmem ./cache
```
