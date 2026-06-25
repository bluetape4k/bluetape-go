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

## Shared Server API

Use `Start(ctx, t)` when a PostgreSQL connection string is enough. Use
`StartServer(ctx, t)` when a test needs the shared Testcontainers server
contract: host lookup, mapped ports, endpoints, connection details, cleanup,
manual termination, or explicit env export.

The example assumes `tcserver` aliases `github.com/bluetape4k/bluetape-go/testcontainers/server`.

```go
srv := postgrestestcontainer.StartServer(ctx, t)
details, err := srv.ConnectionDetails(ctx)
if err != nil {
    t.Fatalf("postgres details: %v", err)
}
if err := tcserver.ExportEnv(t, details, map[string]string{
    postgrestestcontainer.ConnectionStringKey: "BLUETAPE_POSTGRES_URL",
}); err != nil {
    t.Fatal(err)
}
```

`tcserver.ExportEnv` uses `testing.TB.Setenv`; do not call it from tests that
use `t.Parallel` or have parallel ancestors.

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
- Dynamic host port mapping is the default. Read mapped ports and exported env
  values after the container starts; they point to host ports, not
  container-internal ports.
- Fixed host ports are not exposed by this helper because they can collide in
  parallel local runs and CI jobs.
- Run Docker-backed Testcontainers packages serially when resources or ports are
  shared.
- CI jobs without Docker should skip `./testcontainers/...`; CI jobs that cover
  these helpers should run the packages with `-p 1`.

## Test

```bash
go test -p 1 -count=1 ./testcontainers/postgres
```
