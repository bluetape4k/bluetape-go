# testcontainers/postgis

`testcontainers/postgis` starts the digest-pinned PostGIS image used by
integration tests. The fixture owns the container and registers bounded cleanup
with the caller's `testing.TB`; callers own the returned database connection.

```go
dsn := postgistestcontainer.Start(ctx, t)
db, err := sql.Open("pgx", dsn)
```

The fixture enables the `postgis` extension image and exposes
`postgis.connection-string`. It is an integration-test helper, not a production
connection manager.

```bash
go test -p 1 -count=1 -timeout=10m ./testcontainers/postgis ./sqlkit/postgis
```
