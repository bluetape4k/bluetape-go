# testcontainers/mongodb

[English](README.md) | [한국어](README.ko.md)

`testcontainers/mongodb` starts a MongoDB container for integration tests and
returns the module-provided connection URI.

![testcontainers helper flow](../../docs/images/readme-diagrams/testcontainers-helper-flow.png)

## Import

```go
import mongodbtestcontainer "github.com/bluetape4k/bluetape-go/testcontainers/mongodb"
```

## Usage

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

Use `Start(ctx, t)` when a MongoDB URI is enough. Use `StartServer(ctx, t)` when
a test needs the shared Testcontainers server contract: host lookup, mapped
ports, endpoints, connection details, cleanup, manual termination, or explicit
env export.

The example assumes `tcserver` aliases `github.com/bluetape4k/bluetape-go/testcontainers/server`.

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

`tcserver.ExportEnv` uses `testing.TB.Setenv`; do not call it from tests that
use `t.Parallel` or have parallel ancestors.

## Behavior

- Uses `mongo:7.0`.
- Returns the Testcontainers MongoDB module connection URI.
- Registers container termination with `t.Cleanup`.
- Exposes the URI key as `mongodbtestcontainer.URIKey` (`mongodb.uri`).
- Start failures are categorized as Docker unavailable, image pull failure,
  readiness timeout, context cancellation, or wrapper failure.

## Operational Boundaries

- Docker or another Testcontainers-compatible runtime must be available.
- The fixture starts containers only. MongoDB clients, databases, collections,
  credentials, indexes, and cleanup data remain caller-owned.
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
go test -p 1 -count=1 ./testcontainers/mongodb
```
