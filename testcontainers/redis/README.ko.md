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
details := map[string]string{
    redistestcontainer.AddressKey: addr,
}
client := redis.NewClient(&redis.Options{Addr: details[redistestcontainer.AddressKey]})
t.Cleanup(func() {
    _ = client.Close()
})
```

## 동작

- `redis:7.4-alpine`을 사용합니다.
- Redis readiness를 기다린 뒤 반환합니다.
- Container termination을 `t.Cleanup`에 등록합니다.
- Address key는 `redistestcontainer.AddressKey` (`redis.address`)로
  노출합니다.
- Start failure는 Docker unavailable, image pull failure, readiness timeout,
  context cancellation, wrapper failure로 구분해 보고합니다.

## 운영 경계

- Docker 또는 다른 Testcontainers-compatible runtime이 필요합니다.
- Helper는 test용이며 production Redis configuration을 노출하지 않습니다.
- Docker resource나 port를 공유하는 Testcontainers package는 serial로
  실행하세요.
- Docker가 없는 CI job은 `./testcontainers/...`를 제외하고, 이 helper를 검증하는
  CI job은 `-p 1`로 실행하세요.

## 테스트

```bash
go test -p 1 -count=1 ./testcontainers/redis
```
