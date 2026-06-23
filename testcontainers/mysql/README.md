# testcontainers/mysql

[English](README.md) | [한국어](README.ko.md)

`testcontainers/mysql` starts a MySQL container for integration tests and
returns a DSN suitable for `database/sql` with the MySQL driver.

![testcontainers helper flow](../../docs/images/readme-diagrams/testcontainers-helper-flow.png)

## Import

```go
import mysqltestcontainer "github.com/bluetape4k/bluetape-go/testcontainers/mysql"
```

## Usage

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

Use `Start(ctx, t)` when a MySQL DSN is enough. Use `StartServer(ctx, t)` when a
test needs the shared Testcontainers server contract: host lookup, mapped ports,
endpoints, connection details, cleanup, manual termination, or explicit env
export.

The example assumes `tcserver` aliases `github.com/bluetape4k/bluetape-go/testcontainers/server`.

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

`tcserver.ExportEnv` uses `testing.TB.Setenv`; do not call it from tests that
use `t.Parallel` or have parallel ancestors.

## Behavior

- Uses `mysql:8.4`.
- Creates database, username, and password as `bluetape`.
- Adds `parseTime=true` to the returned connection string.
- Registers container termination with `t.Cleanup`.
- Exposes the data source name key as `mysqltestcontainer.DSNKey`
  (`mysql.dsn`).
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
go test -p 1 -count=1 ./testcontainers/mysql
```
