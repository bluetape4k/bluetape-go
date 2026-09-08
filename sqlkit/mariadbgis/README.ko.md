# sqlkit/mariadbgis

`sqlkit/mariadbgis`는 작은 `database/sql` 호환 MariaDB Point 값과 엔진별
spatial statement를 제공합니다. 좌표는 `X=longitude`, `Y=latitude`이며,
`Value`는 WKB를 반환하고 `ST_SRID`로 읽은 SRID는 `ScanWithSRID`가 복원합니다.

```go
point, _ := mariadbgis.NewWGS84Point(37.4979, 127.0276)
stmt, _ := mariadbgis.InsertPoint("places", "location", point)
_, err := db.ExecContext(ctx, stmt.SQL, stmt.Args...)
```

MariaDB는 `POINT` column에 `REF_SYSTEM_ID`를 사용하고, 생성에는
`ST_PointFromWKB`, indexed `MBRContains` predicate에는 `ST_PolyFromWKB`를
사용합니다. `WithinDistance`는 `ST_Distance_Sphere`의 meter 단위입니다.
MariaDB SRID metadata는 엔진이 소유하며, 이 helper가 projection이나
cross-dialect parity를 보장하지는 않습니다.

```bash
go test -p 1 -count=1 -timeout=10m ./sqlkit/mariadbgis
go test -race -count=1 ./sqlkit/mariadbgis
```
