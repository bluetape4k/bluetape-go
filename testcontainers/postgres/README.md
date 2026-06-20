# testcontainers/postgres

[English](README.md) | [한국어](README.ko.md)

`testcontainers/postgres` starts a PostgreSQL container for integration tests
and returns a connection string with `sslmode=disable`.

![testcontainers helper flow](../../docs/images/readme-diagrams/testcontainers-helper-flow.png)

## Import

```go
import postgrestestcontainer "github.com/bluetape4k/bluetape-go/testcontainers/postgres"
```

## Usage

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

## Behavior

- Uses `postgres:16-alpine`.
- Creates database, username, and password as `bluetape`.
- Applies the PostgreSQL module's basic wait strategies.
- Registers container termination with `t.Cleanup`.

## Operational Boundaries

- Docker or another Testcontainers-compatible runtime must be available.
- The fixture is for tests and uses fixed test credentials.

## Test

```bash
go test -count=1 ./testcontainers/postgres
```
