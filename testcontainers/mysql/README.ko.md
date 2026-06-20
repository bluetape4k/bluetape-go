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
dsn := mysqltestcontainer.Start(context.Background(), t)
db, err := sql.Open("mysql", dsn)
if err != nil {
    t.Fatalf("open mysql: %v", err)
}
t.Cleanup(func() {
    _ = db.Close()
})
```

## 동작

- `mysql:8.4`를 사용합니다.
- Database, username, password는 `bluetape`로 생성합니다.
- 반환 connection string에 `parseTime=true`를 추가합니다.
- Container termination을 `t.Cleanup`에 등록합니다.

## 운영 경계

- Docker 또는 다른 Testcontainers-compatible runtime이 필요합니다.
- Fixture는 test용이며 fixed test credential을 사용합니다.

## 테스트

```bash
go test -count=1 ./testcontainers/mysql
```
