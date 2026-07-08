# testcontainers/mongodb

[English](README.md) | [한국어](README.ko.md)

`testcontainers/mongodb`는 integration test용 MongoDB container를 시작하고
Testcontainers MongoDB module이 제공하는 connection URI를 반환합니다.

![testcontainers helper flow](../../docs/images/readme-diagrams/testcontainers-helper-flow.png)

## 가져오기

```go
import mongodbtestcontainer "github.com/bluetape4k/bluetape-go/testcontainers/mongodb"
```

## 사용 예

```go
ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
t.Cleanup(cancel)

uri := mongodbtestcontainer.Start(ctx, t)
details := map[string]string{
    mongodbtestcontainer.URIKey: uri,
}
client, err := mongo.Connect(options.Client().ApplyURI(details[mongodbtestcontainer.URIKey]))
if err != nil {
    t.Fatalf("connect mongodb: %v", err)
}
t.Cleanup(func() {
    cleanupCtx, cleanupCancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
    defer cleanupCancel()
    _ = client.Disconnect(cleanupCtx)
})
```

## Shared Server API

MongoDB URI만 필요하면 `Start(ctx, t)`를 사용합니다. Host lookup, mapped port,
endpoint, connection details, cleanup, manual termination, 명시적 env export가
필요하면 shared Testcontainers server contract를 반환하는 `StartServer(ctx, t)`를
사용하세요.

예제는 `tcserver`가 `github.com/bluetape4k/bluetape-go/testcontainers/server`를 alias import한다고 가정합니다.

```go
srv := mongodbtestcontainer.StartServer(ctx, t)
details, err := srv.ConnectionDetails(ctx)
if err != nil {
    t.Fatalf("mongodb details: %v", err)
}
if err := tcserver.ExportEnv(t, details, map[string]string{
    mongodbtestcontainer.URIKey: "BLUETAPE_MONGODB_URI",
}); err != nil {
    t.Fatal(err)
}
```

`tcserver.ExportEnv`는 `testing.TB.Setenv`를 사용합니다. `t.Parallel`을
호출하거나 parallel ancestor가 있는 테스트에서는 사용하지 마세요.

## 동작

- `mongo:7.0` 이미지를 사용합니다.
- Testcontainers MongoDB module connection URI를 반환합니다.
- Container termination을 `t.Cleanup`에 등록합니다.
- URI key는 `mongodbtestcontainer.URIKey` (`mongodb.uri`)로 노출합니다.
- Start failure는 Docker unavailable, image pull failure, readiness timeout,
  context cancellation, wrapper failure로 구분해 보고합니다.

## 운영 경계

- Docker 또는 다른 Testcontainers-compatible runtime이 필요합니다.
- Fixture는 container만 시작합니다. MongoDB client, database, collection,
  credential, index, cleanup data는 caller-owned입니다.
- Dynamic host port mapping이 기본입니다. Mapped port와 exported env value는
  container 시작 후 읽어야 하며, container-internal port가 아니라 host port를
  가리킵니다.
- Fixed host port는 parallel local run과 CI job에서 충돌할 수 있어 이 helper가
  노출하지 않습니다.
- Docker resource나 port를 공유하는 Testcontainers package는 serial로
  실행하세요.
- Docker가 없는 CI job은 `./testcontainers/...`를 제외하고, 이 helper를 검증하는
  CI job은 `-p 1`로 실행하세요.

## 테스트

```bash
go test -p 1 -count=1 ./testcontainers/mongodb
```
