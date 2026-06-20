# testcontainers/redis

[English](README.md) | [한국어](README.ko.md)

`testcontainers/redis`는 integration test용 Redis container를 시작하고 mapped
`host:port` address를 반환합니다.

![testcontainers helper flow](../../docs/images/readme-diagrams/testcontainers-helper-flow.png)

## 가져오기

```go
import redistestcontainer "github.com/bluetape4k/bluetape-go/testcontainers/redis"
```

## 사용 예

```go
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
t.Cleanup(cancel)

addr := redistestcontainer.Start(ctx, t)
client := redis.NewClient(&redis.Options{Addr: addr})
t.Cleanup(func() {
    _ = client.Close()
})
```

## 동작

- `redis:7.4-alpine`을 사용합니다.
- Redis readiness를 기다린 뒤 반환합니다.
- Container termination을 `t.Cleanup`에 등록합니다.
- Fatal test failure는 supplied `testing.T`로 보고됩니다.

## 운영 경계

- Docker 또는 다른 Testcontainers-compatible runtime이 필요합니다.
- Helper는 test용이며 production Redis configuration을 노출하지 않습니다.

## 테스트

```bash
go test -count=1 ./testcontainers/redis
```
