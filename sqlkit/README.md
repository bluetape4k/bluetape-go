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

## JSON and Encrypted Columns

The column helpers keep SQL NULL separate from values that happen to be empty
or JSON `null`.

| Stored value | Helper | SQL NULL | Non-NULL empty/null value |
|---|---|---|---|
| JSON/JSONB text or bytes | `JSONColumn[T]` | `Valid=false` | JSON literal `null` has `Valid=true` |
| BYTEA/BLOB envelope | `EncryptedBytesColumn` | `Valid=false` | Empty or nil plaintext with `Valid=true` is encrypted |
| TEXT/VARCHAR base64 envelope | `EncryptedStringColumn` | `Valid=false` | An empty string with `Valid=true` is encrypted |

Use `JSONColumn[T]` directly with `database/sql`:

```go
type Profile struct {
    Name string `json:"name"`
}

var profile sqlkit.JSONColumn[Profile]
err := db.QueryRowContext(ctx,
    "select profile from accounts where id = $1", id,
).Scan(&profile)

updated := sqlkit.JSONColumn[Profile]{
    Data:  Profile{Name: "Ada"},
    Valid: true,
}
_, err = db.ExecContext(ctx,
    "update accounts set profile = $1 where id = $2", updated, id,
)
```

Encrypted columns reuse a caller-owned `encrypt.Encryptor` and copy the
associated data supplied to their constructors:

```go
payload := sqlkit.NewEncryptedBytesColumn(encryptor,
    []byte("table=secrets:column=payload"))
payload.Data = []byte("secret payload")
payload.Valid = true

stmt, err := sqlkit.InsertInto("secrets").
    Columns("id", "payload").
    Values(id, payload).
    Build()

loaded, err := sqlkit.QueryOne(ctx, db,
    "select payload from secrets where id = $1",
    func(rows *sql.Rows) ([]byte, error) {
        column := sqlkit.NewEncryptedBytesColumn(encryptor,
            []byte("table=secrets:column=payload"))
        if err := rows.Scan(&column); err != nil {
            return nil, err
        }
        return append([]byte(nil), column.Data...), nil
    }, id)
```

Generated query parameters can accept the standard `driver.Valuer` contract
without a sqlc, Jet, or ORM runtime dependency:

```go
note := sqlkit.NewEncryptedStringColumn(encryptor,
    []byte("table=secrets:column=note"))
note.Data = "secret text"
note.Valid = true

err := queries.UpdateSecret(ctx, UpdateSecretParams{
    ID:   id,
    Note: note, // generated field accepts driver.Valuer
})
```

`DefaultJSONColumnMaxBytes` and
`DefaultEncryptedColumnMaxPlaintextBytes` are 1 MiB.
`DefaultEncryptedColumnMaxCiphertextBytes` is 2 MiB. A zero limit selects the
default, a positive value overrides it, and a negative value returns
`ErrInvalidColumnValue`. Oversized source or output returns
`ErrColumnValueTooLarge`.

`Scan` copies driver-owned bytes, clears the previous value before decoding,
and leaves the column invalid after any failure. Encrypted constructors also
copy associated data. Errors preserve the sqlkit and `encrypt` sentinels for
`errors.Is`, while their strings omit JSON, plaintext, ciphertext, keys, and
associated data. Random nonces mean encrypted values cannot support equality,
ordering, or filtering queries.

## Diagram

![sqlkit helper contract map](../docs/images/readme-diagrams/sqlkit-helper-contract-map.png)

The contract map shows that `sqlkit` stays at the `database/sql` boundary:
callers own handles and SQL, while helpers provide transaction control, row
mapping, inspectable PostgreSQL-first statements, and small safety checks.

![sqlkit column scan and value sequence](../docs/images/readme-diagrams/sqlkit-column-scan-value-sequence.png)

The column sequence shows how `database/sql` invokes `driver.Valuer` and
`sql.Scanner`, including NULL branches, bounded JSON or encryption work, and
the rule that failed scans never publish partial state.

![sqlkit transaction and query sequence](../docs/images/readme-diagrams/sqlkit-tx-query-sequence.png)

The sequence follows builder output through `WithTx`, `Statement.Exec`,
commit/rollback handling, `QueryOne`, explicit mapper scanning, row closing,
and cardinality errors.

## Engine-specific GIS helpers

Spatial SQL stays outside the PostgreSQL-first core builder. Use the independent
engine package that matches the database contract:

| Engine | Package | Contract |
|---|---|---|
| PostGIS | [`sqlkit/postgis`](postgis/README.md) | EWKB/SRID values, spatial DDL, indexed `ST_DWithin`, and bounding-box helpers. |
| MySQL 8.4 | [`sqlkit/mysqlgis`](mysqlgis/README.md) | SRID-constrained WKB values, axis-order-aware constructors, spherical distance, and MBR helpers. |
| MariaDB | [`sqlkit/mariadbgis`](mariadbgis/README.md) | SRID-constrained WKB values, engine-native constructors, distance, and MBR helpers. |

These helpers intentionally do not create a shared dialect abstraction. They
keep engine-specific SRID, axis-order, index, and distance semantics visible.

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

## Workshop Adoption

Runnable workshop examples live in
[`bluetape-go-workshop`](https://github.com/bluetape4k/bluetape-go-workshop)
instead of this package README:
[`sql-access-strategy-decision`](https://github.com/bluetape4k/bluetape-go-workshop/tree/develop/examples/sql-access-strategy-decision),
[`sql-order-repository`](https://github.com/bluetape4k/bluetape-go-workshop/tree/develop/examples/sql-order-repository),
[`sql-transaction-boundary`](https://github.com/bluetape4k/bluetape-go-workshop/tree/develop/examples/sql-transaction-boundary),
[`gin-sql-crud-api`](https://github.com/bluetape4k/bluetape-go-workshop/tree/develop/examples/gin-sql-crud-api), and
[`gin-sql-order-service`](https://github.com/bluetape4k/bluetape-go-workshop/tree/develop/examples/gin-sql-order-service).

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
