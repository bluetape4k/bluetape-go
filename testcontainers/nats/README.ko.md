# testcontainers/nats

[English](README.md) | [한국어](README.ko.md)

`testcontainers/nats`는 integration test용 NATS container를 시작하고 client
connection URL을 반환합니다.

![testcontainers helper flow](../../docs/images/readme-diagrams/testcontainers-helper-flow.png)

## 가져오기

```go
import natstestcontainer "github.com/bluetape4k/bluetape-go/testcontainers/nats"
```

## 사용 예

```go
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
t.Cleanup(cancel)

url := natstestcontainer.Start(ctx, t)
details := map[string]string{
    natstestcontainer.URLKey: url,
}
client, err := nats.Connect(details[natstestcontainer.URLKey], nats.Timeout(5*time.Second))
if err != nil {
    t.Fatalf("connect nats: %v", err)
}
t.Cleanup(client.Close)
```

## 동작

- `nats:2.10-alpine`을 사용합니다.
- Module-provided connection string을 반환합니다.
- Container termination을 `t.Cleanup`에 등록합니다.
- URL key는 `natstestcontainer.URLKey` (`nats.url`)로 노출합니다.
- Start failure는 Docker unavailable, image pull failure, readiness timeout,
  context cancellation, wrapper failure로 구분해 보고합니다.

## 운영 경계

- Docker 또는 다른 Testcontainers-compatible runtime이 필요합니다.
- Fixture는 test용이며 production NATS configuration을 노출하지 않습니다.
- Docker resource나 port를 공유하는 Testcontainers package는 serial로
  실행하세요.
- Docker가 없는 CI job은 `./testcontainers/...`를 제외하고, 이 helper를 검증하는
  CI job은 `-p 1`로 실행하세요.

## 테스트

```bash
go test -p 1 -count=1 ./testcontainers/nats
```
