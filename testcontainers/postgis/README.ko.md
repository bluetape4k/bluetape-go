# testcontainers/postgis

`testcontainers/postgis`는 integration test에서 사용하는 digest-pinned
PostGIS image를 시작합니다. fixture가 container와 bounded cleanup을 소유하고,
호출자는 반환된 database connection을 소유합니다.

```go
dsn := postgistestcontainer.Start(ctx, t)
db, err := sql.Open("pgx", dsn)
```

image에는 `postgis` extension이 포함되어 있으며
`postgis.connection-string` detail을 제공합니다. 이 패키지는 production
connection manager가 아니라 integration-test helper입니다.

```bash
go test -p 1 -count=1 -timeout=10m ./testcontainers/postgis ./sqlkit/postgis
```
