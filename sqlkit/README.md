# sqlkit

[English](README.md) | [한국어](README.ko.md)

`sqlkit` provides small `database/sql` helpers for context-aware transactions,
explicit row mapping, and PostgreSQL-first inspectable SQL statement builders.
It keeps SQL caller-owned and visible.

## Import

```go
import "github.com/bluetape4k/bluetape-go/sqlkit"
```

## Usage

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

// stmt.SQL is inspectable:
// select "name" from "accounts" where id = $1
// stmt.Args is []any{id}.
```

## Selection Guide

See the [SQL generator/migration guide](../docs/sql-generator-migration-guidance.md)
for when to use direct `database/sql`, `sqlkit`, sqlc, Jet, ent, Bun, GORM,
goqu, or Atlas.

| Need | Use | Notes |
|---|---|---|
| Run one transaction | `WithTx` | Caller owns `*sql.DB`; `sqlkit` owns only the started transaction. |
| Share code across `*sql.DB` and `*sql.Tx` | `Session`, `Queryer`, `Execer` | Small interfaces matching `database/sql` methods. |
| Map any number of rows | `QueryAll` | The mapper receives `*sql.Rows` and calls `Scan`. |
| Map zero or one row | `QueryOptional` | Returns `(value, false, nil)` for no rows. |
| Require exactly one row | `QueryOne` | Returns `ErrNoRows` or `ErrTooManyRows` for cardinality failures. |
| Scan one column | `ScanOne` | Convenience mapper for a single destination pointer. |
| Build simple SQL | `SelectFrom`, `InsertInto`, `Update`, `DeleteFrom` | Produces explicit PostgreSQL-style SQL and copied args. |
| Execute a built mutation | `Statement.Exec` | Context-aware wrapper around `ExecContext`. |

## Behavior

- Builder output is PostgreSQL-first and uses `$1`, `$2`, ... placeholders.
  There is no broad dialect abstraction in this first slice.
- Builders validate and double-quote table/column identifiers. `Where`
  fragments are caller-owned SQL; their `?` value placeholders are rewritten to
  PostgreSQL placeholders.
- `Update` and `DeleteFrom` require a `Where` clause by default to avoid
  accidental full-table mutations.
- `sqlkit` does not manage pool lifecycle, migrations, schema metadata,
  generated code, model hooks, cache invalidation, or ORM state.
- `WithTx` commits only when the callback returns nil. Callback errors trigger
  rollback and preserve the original error for `errors.Is` / `errors.As`.
- `QueryAll`, `QueryOptional`, and `QueryOne` close `*sql.Rows` on success and
  failure paths.
- Query helpers preserve driver and context errors with `%w`.
- Use direct `database/sql`, `pgx`, sqlc, Jet, ent, Bun, GORM, or goqu when
  callers need driver-native APIs, generated type-safe query code, entity
  modeling, migration orchestration, non-PostgreSQL placeholder generation,
  joins as first-class builder nodes, or a larger query builder/ORM surface.
- Keep sqlc, Jet, and Atlas at the application workflow boundary; `sqlkit`
  intentionally adds no runtime dependency on those tools.

## Test

```bash
go test -count=1 ./sqlkit
go test -race -count=1 ./sqlkit
```
