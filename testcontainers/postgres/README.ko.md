# testcontainers/postgres

[English](README.md) | [한국어](README.ko.md)

`testcontainers/postgres`는 integration test용 PostgreSQL container를 시작하고
`sslmode=disable`이 포함된 connection string을 반환합니다.

![testcontainers helper flow](../../docs/images/readme-diagrams/testcontainers-helper-flow.png)

## 가져오기

```go
import postgrestestcontainer "github.com/bluetape4k/bluetape-go/testcontainers/postgres"
```

## 사용 예

```go
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
t.Cleanup(cancel)

connString := postgrestestcontainer.Start(ctx, t)
details := map[string]string{
    postgrestestcontainer.ConnectionStringKey: connString,
}
conn, err := pgx.Connect(ctx, details[postgrestestcontainer.ConnectionStringKey])
if err != nil {
    t.Fatalf("connect postgres: %v", err)
}
t.Cleanup(func() {
    _ = conn.Close(context.Background())
})
```

## 동작

- `postgres:16-alpine`을 사용합니다.
- Database, username, password는 `bluetape`로 생성합니다.
- PostgreSQL module의 basic wait strategy를 적용합니다.
- Container termination을 `t.Cleanup`에 등록합니다.
- Connection string key는 `postgrestestcontainer.ConnectionStringKey`
  (`postgres.connection-string`)로 노출합니다.
- Start failure는 Docker unavailable, image pull failure, readiness timeout,
  context cancellation, wrapper failure로 구분해 보고합니다.

## 운영 경계

- Docker 또는 다른 Testcontainers-compatible runtime이 필요합니다.
- Fixture는 test용이며 fixed test credential을 사용합니다.
- Docker resource나 port를 공유하는 Testcontainers package는 serial로
  실행하세요.
- Docker가 없는 CI job은 `./testcontainers/...`를 제외하고, 이 helper를 검증하는
  CI job은 `-p 1`로 실행하세요.

## 테스트

```bash
go test -p 1 -count=1 ./testcontainers/postgres
```
