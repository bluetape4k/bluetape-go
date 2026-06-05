# testcontainers/mysql

[English](README.md) | [한국어](README.ko.md)

`testcontainers/mysql` starts a MySQL container for integration tests and
returns a DSN suitable for `database/sql` with the MySQL driver.

## Import

```go
import mysqltestcontainer "github.com/bluetape4k/bluetape-go/testcontainers/mysql"
```

## Usage

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

## Behavior

- Uses `mysql:8.4`.
- Creates database, username, and password as `bluetape`.
- Adds `parseTime=true` to the returned connection string.
- Registers container termination with `t.Cleanup`.

## Operational Boundaries

- Docker or another Testcontainers-compatible runtime must be available.
- The fixture is for tests and uses fixed test credentials.

## Test

```bash
go test -count=1 ./testcontainers/mysql
```
