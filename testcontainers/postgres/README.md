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

## Behavior

- Uses `postgres:16-alpine`.
- Creates database, username, and password as `bluetape`.
- Applies the PostgreSQL module's basic wait strategies.
- Registers container termination with `t.Cleanup`.
- Exposes the connection string key as
  `postgrestestcontainer.ConnectionStringKey`
  (`postgres.connection-string`).
- Start failures are categorized as Docker unavailable, image pull failure,
  readiness timeout, context cancellation, or wrapper failure.

## Operational Boundaries

- Docker or another Testcontainers-compatible runtime must be available.
- The fixture is for tests and uses fixed test credentials.
- Run Docker-backed Testcontainers packages serially when resources or ports are
  shared.
- CI jobs without Docker should skip `./testcontainers/...`; CI jobs that cover
  these helpers should run the packages with `-p 1`.

## Test

```bash
go test -p 1 -count=1 ./testcontainers/postgres
```
