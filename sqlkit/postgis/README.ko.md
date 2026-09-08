# sqlkit/postgis

`sqlkit/postgis`는 작은 `database/sql` 호환 PostGIS Point 값과 검토 가능한
spatial statement를 제공합니다. 좌표는 `X=longitude`, `Y=latitude`이며 유효한
값은 EWKB에 명시적인 SRID를 보존합니다.

```go
point, _ := postgis.NewWGS84Point(37.4979, 127.0276)
stmt, _ := postgis.InsertPoint("places", "location", point)
_, err := stmt.Exec(ctx, db)
```

`CreateSpatialTableSQL`은 `geometry(Point, srid) NOT NULL` column을 만들고,
`CreateSpatialIndexSQL`은 GIST index를 만듭니다. `WithinDistance`는 같은 SRID의
geometry에 indexed `ST_DWithin` predicate를 사용하고, `WithinBounds`는
bounding-box operator를 사용합니다. `Scan`은 EWKB/WKB/WKT를 읽으며 실패하면
receiver를 초기화합니다.

이 package는 database handle, migration, projection, ORM state 또는
cross-dialect abstraction을 소유하지 않습니다. Local integration test는
[`testcontainers/postgis`](../../testcontainers/postgis/README.ko.md)의
digest-pinned fixture를 사용합니다.

```bash
go test -p 1 -count=1 -timeout=10m ./testcontainers/postgis ./sqlkit/postgis
go test -race -count=1 ./sqlkit/postgis
```
