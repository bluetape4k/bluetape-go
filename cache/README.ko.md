# cache

[English](README.md) | [한국어](README.ko.md)

`cache`는 framework와 무관한 cache contract와 process-local TTL 구현을 제공합니다. Local cache-aside 코드와 `LoadingCache`가 필요한 Redis coordination wrapper의 기반 패키지입니다.

## 가져오기

```go
import "github.com/bluetape4k/bluetape-go/cache"
```

## 사용 예

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

## 동작

- 값이 없거나 만료되면 `ErrCacheMiss`를 반환하며 `errors.Is`로 비교할 수 있습니다.
- TTL `0`은 만료 없이 저장합니다. 음수 TTL은 거부합니다.
- `Memory.GetOrLoad`는 하나의 cache instance 안에서 같은 key에 대한 동시 loader 호출을 접습니다.
- Cross-process invalidation과 stampede protection은 의도적으로 `cache/redisnear`와 `cache/rediscoord`가 담당합니다.

## 테스트

```bash
go test -count=1 ./cache
```

## 벤치마크

```bash
go test -run '^$' -bench '^BenchmarkMemory' -benchmem ./cache
```
