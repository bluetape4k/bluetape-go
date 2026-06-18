# testcontainers/postgres

[English](README.md) | [한국어](README.ko.md)

`testcontainers/postgres`는 integration test용 PostgreSQL container를 시작하고
`sslmode=disable`이 포함된 connection string을 반환합니다.

## 가져오기

```go
import postgrestestcontainer "github.com/bluetape4k/bluetape-go/testcontainers/postgres"
```

## 사용 예

```go
connString := postgrestestcontainer.Start(context.Background(), t)
conn, err := pgx.Connect(context.Background(), connString)
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

## 운영 경계

- Docker 또는 다른 Testcontainers-compatible runtime이 필요합니다.
- Fixture는 test용이며 fixed test credential을 사용합니다.

## 테스트

```bash
go test -count=1 ./testcontainers/postgres
```
