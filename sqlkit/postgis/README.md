# sqlkit/postgis

`sqlkit/postgis` provides a small `database/sql`-compatible PostGIS Point value
and inspectable spatial statements. Coordinates use `X=longitude` and
`Y=latitude`; valid values retain an explicit SRID in EWKB.

```go
point, _ := postgis.NewWGS84Point(37.4979, 127.0276)
stmt, _ := postgis.InsertPoint("places", "location", point)
_, err := stmt.Exec(ctx, db)
```

`CreateSpatialTableSQL` emits a `geometry(Point, srid) NOT NULL` column and
`CreateSpatialIndexSQL` emits a GIST index. `WithinDistance` uses the indexed
`ST_DWithin` predicate for geometries with matching SRID; `WithinBounds` uses
the bounding-box operator. `Scan` accepts EWKB/WKB/WKT and clears the receiver
on failure.

The package owns neither a database handle nor migrations, projections, ORM
state, or a cross-dialect abstraction. The local integration test uses the
digest-pinned PostGIS fixture in [`testcontainers/postgis`](../../testcontainers/postgis/README.md).

```bash
go test -p 1 -count=1 -timeout=10m ./testcontainers/postgis ./sqlkit/postgis
go test -race -count=1 ./sqlkit/postgis
```
