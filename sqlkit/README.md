# sqlkit

[English](README.md) | [한국어](README.ko.md)

`sqlkit` provides small `database/sql` helpers for context-aware transactions
and explicit row mapping. It keeps SQL caller-owned and visible.

## Import

```go
import "github.com/bluetape4k/bluetape-go/sqlkit"
```

## Usage

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

## Selection Guide

| Need | Use | Notes |
|---|---|---|
| Run one transaction | `WithTx` | Caller owns `*sql.DB`; `sqlkit` owns only the started transaction. |
| Share code across `*sql.DB` and `*sql.Tx` | `Session`, `Queryer`, `Execer` | Small interfaces matching `database/sql` methods. |
| Map any number of rows | `QueryAll` | The mapper receives `*sql.Rows` and calls `Scan`. |
| Map zero or one row | `QueryOptional` | Returns `(value, false, nil)` for no rows. |
| Require exactly one row | `QueryOne` | Returns `ErrNoRows` or `ErrTooManyRows` for cardinality failures. |
| Scan one column | `ScanOne` | Convenience mapper for a single destination pointer. |

## Behavior

- `sqlkit` does not build SQL. Pass SQL strings and args explicitly.
- `sqlkit` does not manage pool lifecycle, migrations, schema metadata,
  generated code, model hooks, cache invalidation, or ORM state.
- `WithTx` commits only when the callback returns nil. Callback errors trigger
  rollback and preserve the original error for `errors.Is` / `errors.As`.
- `QueryAll`, `QueryOptional`, and `QueryOne` close `*sql.Rows` on success and
  failure paths.
- Query helpers preserve driver and context errors with `%w`.
- Use direct `database/sql`, `pgx`, sqlc, Jet, ent, Bun, GORM, or goqu when
  callers need driver-native APIs, generated type-safe query code, entity
  modeling, migration orchestration, or a larger query builder/ORM surface.

## Test

```bash
go test -count=1 ./sqlkit
go test -race -count=1 ./sqlkit
```
