# testcontainers/mysql

[English](README.md) | [한국어](README.ko.md)

`testcontainers/mysql`은 integration test용 MySQL container를 시작하고 MySQL
driver와 함께 `database/sql`에서 사용할 수 있는 DSN을 반환합니다.

![testcontainers helper flow](../../docs/images/readme-diagrams/testcontainers-helper-flow.png)

## 가져오기

```go
import mysqltestcontainer "github.com/bluetape4k/bluetape-go/testcontainers/mysql"
```

## 사용 예

```go
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
t.Cleanup(cancel)

dsn := mysqltestcontainer.Start(ctx, t)
details := map[string]string{
    mysqltestcontainer.DSNKey: dsn,
}
db, err := sql.Open("mysql", details[mysqltestcontainer.DSNKey])
if err != nil {
    t.Fatalf("open mysql: %v", err)
}
t.Cleanup(func() {
    _ = db.Close()
})
```

## Shared Server API

MySQL DSN만 필요하면 `Start(ctx, t)`를 사용하세요. Host lookup, mapped port,
endpoint, connection details, cleanup, manual termination, 명시적 env export가
필요하면 shared Testcontainers server contract를 반환하는 `StartServer(ctx, t)`를
사용하세요.

예제는 `tcserver`가 `github.com/bluetape4k/bluetape-go/testcontainers/server`를 alias import한다고 가정합니다.

```go
srv := mysqltestcontainer.StartServer(ctx, t)
details, err := srv.ConnectionDetails(ctx)
if err != nil {
    t.Fatalf("mysql details: %v", err)
}
if err := tcserver.ExportEnv(t, details, map[string]string{
    mysqltestcontainer.DSNKey: "BLUETAPE_MYSQL_DSN",
}); err != nil {
    t.Fatal(err)
}
```

`tcserver.ExportEnv`는 `testing.TB.Setenv`를 사용합니다. `t.Parallel`을
호출하거나 parallel ancestor가 있는 테스트에서는 사용하지 마세요.

## 동작

- immutable `mysql:8.4@sha256:b3b90af2a6552ae30c266fdb7d5dd55f3afb72404bb78d37fe8a23eb857fd3fb` image reference를 사용합니다.
- Database, username, password는 `bluetape`로 생성합니다.
- 반환 connection string에 `parseTime=true`를 추가합니다.
- Container termination을 `t.Cleanup`에 등록합니다.
- DSN key는 `mysqltestcontainer.DSNKey` (`mysql.dsn`)로 노출합니다.
- Start failure는 Docker unavailable, image pull failure, readiness timeout,
  context cancellation, wrapper failure로 구분해 보고합니다.

## 운영 경계

- Docker 또는 다른 Testcontainers-compatible runtime이 필요합니다.
- Fixture는 test용이며 fixed test credential을 사용합니다.
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
go test -p 1 -count=1 ./testcontainers/mysql
```
