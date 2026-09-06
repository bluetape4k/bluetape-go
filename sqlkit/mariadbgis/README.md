# sqlkit/mariadbgis

`sqlkit/mariadbgis` provides a small `database/sql`-compatible MariaDB Point
value and engine-specific spatial statements. Coordinates use
`X=longitude`, `Y=latitude`; `Value` returns WKB and `ScanWithSRID` restores the
SRID returned by `ST_SRID`.

```go
point, _ := mariadbgis.NewWGS84Point(37.4979, 127.0276)
stmt, _ := mariadbgis.InsertPoint("places", "location", point)
_, err := db.ExecContext(ctx, stmt.SQL, stmt.Args...)
```

MariaDB uses `REF_SYSTEM_ID` on the `POINT` column, `ST_PointFromWKB` for
construction, and `ST_PolyFromWKB` for the indexed `MBRContains` predicate.
`WithinDistance` uses `ST_Distance_Sphere` metres. MariaDB SRID metadata is
engine-owned; the helper does not imply projection or cross-dialect parity.

```bash
go test -p 1 -count=1 -timeout=10m ./sqlkit/mariadbgis
go test -race -count=1 ./sqlkit/mariadbgis
```
