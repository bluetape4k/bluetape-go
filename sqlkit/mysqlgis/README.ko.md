# sqlkit/mysqlgis

`sqlkit/mysqlgis`는 작은 `database/sql` 호환 MySQL Point 값과 검토 가능한
spatial statement를 제공합니다. 좌표는 `X=longitude`, `Y=latitude`이며,
`Value`는 WKB를 반환하고 query helper가 명시적인 SRID를 bind합니다.

```go
point, _ := mysqlgis.NewWGS84Point(37.4979, 127.0276)
stmt, _ := mysqlgis.InsertPoint("places", "location", point)
_, err := db.ExecContext(ctx, stmt.SQL, stmt.Args...)
```

MySQL 8.4 geographic SRS는 모든 constructor/reader query에
`axis-order=long-lat` 옵션을 명시해야 합니다. `WithinDistance`는
`ST_Distance_Sphere`의 meter 단위를 사용하고, `WithinBounds`는 indexed
`MBRContains` predicate를 사용합니다. `ST_AsBinary`와 `ST_SRID`를 함께
읽은 뒤 `Point.ScanWithSRID`를 호출합니다.

이 패키지는 ORM, projection engine, retry policy, cross-dialect abstraction을
제공하지 않습니다.

```bash
go test -p 1 -count=1 -timeout=10m ./sqlkit/mysqlgis
go test -race -count=1 ./sqlkit/mysqlgis
```
