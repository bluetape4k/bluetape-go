# sqlkit

[English](README.md) | [한국어](README.ko.md)

`sqlkit`은 context-aware transaction, 명시적 row mapping, PostgreSQL 우선
inspectable SQL statement builder를 위한 작은 `database/sql` helper를
제공합니다. SQL 문자열과 args는 caller가 소유하며 숨겨진 SQL을 만들지
않습니다.

## Import

```go
import "github.com/bluetape4k/bluetape-go/sqlkit"
```

## 사용 예

```go
err := sqlkit.WithTx(ctx, db, nil, func(ctx context.Context, tx *sql.Tx) error {
    stmt, err := sqlkit.InsertInto("accounts").
        Columns("id", "name").
        Values(id, name).
        Build()
    if err != nil {
        return err
    }
    _, err = stmt.Exec(ctx, tx)
    return err
})
if err != nil {
    return err
}

stmt, err := sqlkit.SelectFrom("accounts").
    Columns("name").
    Where("id = ?", id).
    Build()
if err != nil {
    return err
}

name, err := sqlkit.QueryOne(ctx, db, stmt.SQL, func(rows *sql.Rows) (string, error) {
    var value string
    if err := rows.Scan(&value); err != nil {
        return "", err
    }
    return value, nil
}, stmt.Args...)
if err != nil {
    return err
}
_ = name

// stmt.SQL은 inspectable합니다:
// select "name" from "accounts" where id = $1
// stmt.Args는 []any{id}입니다.
```

## 선택 가이드

[SQL generator/migration guide](../docs/sql-generator-migration-guidance.ko.md)는
direct `database/sql`, `sqlkit`, sqlc, Jet, ent, Bun, GORM, goqu, Atlas를 언제
선택할지 정리합니다.

| 필요 | 사용 | 비고 |
|---|---|---|
| 하나의 transaction 실행 | `WithTx` | caller가 `*sql.DB` lifecycle을 소유하고 `sqlkit`은 시작한 transaction만 소유합니다. |
| `*sql.DB`와 `*sql.Tx`를 같은 코드에서 사용 | `Session`, `Queryer`, `Execer` | `database/sql` method와 맞춘 작은 interface입니다. |
| 여러 row mapping | `QueryAll` | mapper가 `*sql.Rows`를 받아 `Scan`을 호출합니다. |
| 0 또는 1개 row mapping | `QueryOptional` | row가 없으면 `(value, false, nil)`을 반환합니다. |
| 정확히 1개 row 요구 | `QueryOne` | cardinality 실패 시 `ErrNoRows` 또는 `ErrTooManyRows`를 반환합니다. |
| 단일 column scan | `ScanOne` | destination pointer를 위한 convenience mapper입니다. |
| 간단한 SQL 생성 | `SelectFrom`, `InsertInto`, `Update`, `DeleteFrom` | 명시적인 PostgreSQL-style SQL과 복사된 args를 생성합니다. |
| 생성된 mutation 실행 | `Statement.Exec` | `ExecContext`를 감싼 context-aware wrapper입니다. |

## 동작

- Builder output은 PostgreSQL 우선이며 `$1`, `$2`, ... placeholder를
  사용합니다. 첫 builder slice에는 broad dialect abstraction이 없습니다.
- Builder는 table/column identifier를 검증하고 double quote 처리합니다.
  `Where` fragment는 caller-owned SQL이며, 그 안의 `?` value placeholder만
  PostgreSQL placeholder로 변환합니다.
- `Update`와 `DeleteFrom`은 accidental full-table mutation을 피하기 위해
  기본적으로 `Where` clause를 요구합니다.
- `sqlkit`은 pool lifecycle, migration, schema metadata, generated code,
  model hook, cache invalidation, ORM state를 관리하지 않습니다.
- `WithTx`는 callback이 nil을 반환할 때만 commit합니다. callback error는
  rollback을 유발하며 원래 error를 `errors.Is` / `errors.As`로 확인할 수
  있게 보존합니다.
- `QueryAll`, `QueryOptional`, `QueryOne`은 success/failure path 모두에서
  `*sql.Rows`를 닫습니다.
- Query helper는 driver error와 context error를 `%w`로 보존합니다.
- driver-native API, generated type-safe query code, entity modeling,
  migration orchestration, non-PostgreSQL placeholder generation, first-class
  join builder node, 큰 query builder/ORM surface가 필요하면 direct
  `database/sql`, `pgx`, sqlc, Jet, ent, Bun, GORM, goqu를 사용하세요.
- sqlc, Jet, Atlas는 application workflow boundary에 둡니다. `sqlkit`은 이
  도구들에 대한 runtime dependency를 의도적으로 추가하지 않습니다.

## Test

```bash
go test -count=1 ./sqlkit
go test -race -count=1 ./sqlkit
```
