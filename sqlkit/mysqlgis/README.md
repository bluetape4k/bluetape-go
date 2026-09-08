# sqlkit/mysqlgis

`sqlkit/mysqlgis` provides a small `database/sql`-compatible MySQL Point value
and inspectable spatial statements. Coordinates use `X=longitude` and
`Y=latitude`; `Value` returns WKB while query helpers bind the explicit SRID.

```go
point, _ := mysqlgis.NewWGS84Point(37.4979, 127.0276)
stmt, _ := mysqlgis.InsertPoint("places", "location", point)
_, err := db.ExecContext(ctx, stmt.SQL, stmt.Args...)
```

MySQL 8.4 geographic SRS uses an explicit `axis-order=long-lat` option in every
constructor/reader query. `WithinDistance` uses metres via
`ST_Distance_Sphere`; `WithinBounds` uses an indexed `MBRContains` predicate.
Read `ST_AsBinary` and `ST_SRID` together, then call `Point.ScanWithSRID`.

This package does not provide an ORM, projection engine, retry policy, or a
cross-dialect abstraction.

```bash
go test -p 1 -count=1 -timeout=10m ./sqlkit/mysqlgis
go test -race -count=1 ./sqlkit/mysqlgis
```
