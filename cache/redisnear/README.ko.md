# cache/redisnear

[English](README.md) | [한국어](README.ko.md)

`cache/redisnear`는 process-local `cache.LoadingCache[string,V]`에 Redis Pub/Sub
invalidation을 더합니다. Redis는 invalidation bus로만 사용되고, 값은 각
process-local cache 안에 남습니다.

## 다이어그램

![Redis near-cache invalidation sequence](../../docs/images/readme-diagrams/redisnear-invalidation-sequence.png)

## 가져오기

```go
import "github.com/bluetape4k/bluetape-go/cache/redisnear"
```

## 사용 예

```go
client := redis.NewClient(&redis.Options{Addr: "localhost:6379"})

near, err := redisnear.NewPubSub[string](ctx, redisnear.Options[string]{
    Client:    client,
    Namespace: "catalog",
})
if err != nil {
    return err
}
defer near.Close()

value, err := near.GetOrLoad(ctx, "item:1", time.Minute,
    func(ctx context.Context, key string) (string, error) {
        return loadCatalogValue(ctx, key)
    },
)
```

## 동작

- `Set`, `Delete`, `Clear`는 local cache를 변경하고 peer invalidation을 게시합니다.
- Peer cache는 영향을 받은 entry를 삭제하고 다음 miss에서 자신의 loader로 다시 채웁니다.
- `GetOrLoad`는 local cache만 채우며 invalidation을 게시하지 않습니다.
- Receive error는 local value를 지우고 optional `OnError` hook으로 보고됩니다.
- `Close`는 idempotent입니다. Close 이후 operation은 `ErrClosed`를 반환합니다.

## 운영 경계

- Redis Pub/Sub message는 invalidation command이지 authentication boundary가 아닙니다. Production에서는 Redis ACL/TLS와 namespace/channel separation을 사용하세요.
- Redis publish가 실패해도 local mutation은 rollback되지 않습니다.
- Delete와 clear는 이미 실행 중인 loader를 취소하지 않습니다.

## 테스트

```bash
go test -count=1 ./cache/redisnear
```

## 벤치마크

```bash
go test -run '^$' -bench '^BenchmarkNearCache' -benchmem ./cache/redisnear
```
