# sqlkit

[English](README.md) | [한국어](README.ko.md)

`sqlkit`은 context-aware transaction과 명시적 row mapping을 위한 작은
`database/sql` helper를 제공합니다. SQL 문자열과 args는 caller가 소유하며
숨겨진 SQL을 만들지 않습니다.

## Import

```go
import "github.com/bluetape4k/bluetape-go/sqlkit"
```

## 사용 예

```go
err := sqlkit.WithTx(ctx, db, nil, func(ctx context.Context, tx *sql.Tx) error {
    _, err := tx.ExecContext(ctx, `
        insert into accounts (id, name) values ($1, $2)
    `, id, name)
    return err
})
if err != nil {
    return err
}

name, err := sqlkit.QueryOne(ctx, db, `
    select name from accounts where id = $1
`, func(rows *sql.Rows) (string, error) {
    var value string
    if err := rows.Scan(&value); err != nil {
        return "", err
    }
    return value, nil
}, id)
if err != nil {
    return err
}
_ = name
```

## 선택 가이드

| 필요 | 사용 | 비고 |
|---|---|---|
| 하나의 transaction 실행 | `WithTx` | caller가 `*sql.DB` lifecycle을 소유하고 `sqlkit`은 시작한 transaction만 소유합니다. |
| `*sql.DB`와 `*sql.Tx`를 같은 코드에서 사용 | `Session`, `Queryer`, `Execer` | `database/sql` method와 맞춘 작은 interface입니다. |
| 여러 row mapping | `QueryAll` | mapper가 `*sql.Rows`를 받아 `Scan`을 호출합니다. |
| 0 또는 1개 row mapping | `QueryOptional` | row가 없으면 `(value, false, nil)`을 반환합니다. |
| 정확히 1개 row 요구 | `QueryOne` | cardinality 실패 시 `ErrNoRows` 또는 `ErrTooManyRows`를 반환합니다. |
| 단일 column scan | `ScanOne` | destination pointer를 위한 convenience mapper입니다. |

## 동작

- `sqlkit`은 SQL을 생성하지 않습니다. SQL 문자열과 args를 명시적으로
  전달합니다.
- `sqlkit`은 pool lifecycle, migration, schema metadata, generated code,
  model hook, cache invalidation, ORM state를 관리하지 않습니다.
- `WithTx`는 callback이 nil을 반환할 때만 commit합니다. callback error는
  rollback을 유발하며 원래 error를 `errors.Is` / `errors.As`로 확인할 수
  있게 보존합니다.
- `QueryAll`, `QueryOptional`, `QueryOne`은 success/failure path 모두에서
  `*sql.Rows`를 닫습니다.
- Query helper는 driver error와 context error를 `%w`로 보존합니다.
- driver-native API, generated type-safe query code, entity modeling,
  migration orchestration, 큰 query builder/ORM surface가 필요하면 direct
  `database/sql`, `pgx`, sqlc, Jet, ent, Bun, GORM, goqu를 사용하세요.

## Test

```bash
go test -count=1 ./sqlkit
go test -race -count=1 ./sqlkit
```
