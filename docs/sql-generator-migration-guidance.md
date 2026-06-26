# SQL Generator and Migration Guidance

[English](sql-generator-migration-guidance.md) | [한국어](sql-generator-migration-guidance.ko.md)

This guide records the optional SQL generator and migration boundary selected
by [issue #100](https://github.com/bluetape4k/bluetape-go/issues/100) for the
relational SQL epic [issue #101](https://github.com/bluetape4k/bluetape-go/issues/101).
The default bluetape-go direction stays runtime-first: keep SQL visible,
transaction boundaries explicit, and generated code outside the core package
unless an application chooses it.

## Selection Matrix

| Need | Choose | Boundary |
|---|---|---|
| Full driver control, tiny query surface, or driver-specific behavior | Direct `database/sql` or driver-native APIs such as `pgx` | Caller owns SQL, pool lifecycle, transactions, and scanning. |
| Shared transaction, row mapping, cardinality, and simple inspectable PostgreSQL-first builders | [`sqlkit`](../sqlkit/README.md) | Core runtime helper. No schema metadata, migrations, generated models, or ORM state. |
| Stable handwritten SQL with generated type-safe Go methods | [sqlc](https://docs.sqlc.dev/) | Optional application workflow. Generated packages live outside core `sqlkit`. |
| Schema-derived type-safe SQL builder and model files from a live database | [Jet](https://github.com/go-jet/jet) | Optional application workflow. Generation requires a database/schema source and isolated output. |
| Larger runtime query builder than `sqlkit`, without making it the default | goqu | Application-level dependency if callers need broad dialect/query-builder coverage. |
| Entity graph modeling, schema-as-code, or generated data-access layer | ent | Deferred for bluetape-go core. Use at an application boundary when entity modeling is the product need. |
| ORM-style model lifecycle, hooks, eager loading, or active record-like workflows | Bun or GORM | Application-level choice. Not the first SQL milestone default and not wrapped by `sqlkit`. |
| Schema diff, migration file planning, optional linting, and apply workflow | [Atlas](https://atlasgo.io/) | External tool boundary. bluetape-go documents usage but does not wrap migration execution. |

## Runtime-First Boundary

- Keep business repositories explicit: accept `context.Context` and a caller-owned
  `sqlkit.Session`, `*sql.DB`, `*sql.Tx`, or generated query handle.
- Keep generated code in application-owned packages such as `internal/db/sqlc`
  or `internal/db/jet`, not in `sqlkit`.
- Commit generated source only when the application owns that package and review
  policy accepts generated code. Do not commit scratch output under `.tmp`.
- Use `sqlkit.WithTx` to define the transaction boundary, then call direct SQL,
  generated sqlc queries, Jet statements, or `sqlkit` statements inside that
  callback.

Example repository shape:

```go
func CreateAccount(ctx context.Context, db *sql.DB, id int64, name string) error {
    return sqlkit.WithTx(ctx, db, nil, func(ctx context.Context, tx *sql.Tx) error {
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
}
```

Generated packages can use the same boundary. Adapt the constructor call to the
generator's API, but keep `WithTx` as the transaction owner:

```go
func LoadGeneratedAccount(ctx context.Context, db *sql.DB, id int64) (Account, error) {
    var account Account
    err := sqlkit.WithTx(ctx, db, nil, func(ctx context.Context, tx *sql.Tx) error {
        queries := accountsqlc.New(tx)
        row, err := queries.GetAccount(ctx, id)
        if err != nil {
            return err
        }
        account = Account{ID: row.ID, Name: row.Name}
        return nil
    })
    return account, err
}
```

## Isolated sqlc Example

sqlc is optional. Its docs describe `sqlc generate` as reading schema/query SQL
from paths configured in `sqlc.yaml` and writing generated Go code. Keep this
workflow in a scratch or application-owned package.

```bash
tmp=.tmp/sql-guidance/sqlc
rm -rf "$tmp"
mkdir -p "$tmp/schema" "$tmp/query" "$tmp/gen"

cat > "$tmp/schema/schema.sql" <<'SQL'
create table accounts (
  id bigint primary key,
  name text not null
);
SQL

cat > "$tmp/query/accounts.sql" <<'SQL'
-- name: GetAccount :one
select id, name
from accounts
where id = $1;
SQL

cat > "$tmp/sqlc.yaml" <<'YAML'
version: "2"
sql:
  - engine: "postgresql"
    schema: "schema"
    queries: "query"
    gen:
      go:
        package: "accountsqlc"
        out: "gen"
        sql_package: "database/sql"
YAML

(cd "$tmp" && sqlc generate)
```

After generation, move the output only if the application explicitly owns the
generated package. Otherwise leave it under `.tmp` and remove it.

## Isolated Jet Example

Jet is optional. Its generator connects to a database, reads schema metadata,
and writes SQL builder/model files to the destination path. Use an isolated
output directory because Jet cleans generated destination content.

```bash
tmp=.tmp/sql-guidance/jet
rm -rf "$tmp"
mkdir -p "$tmp/gen"

# Requires a running PostgreSQL database with the target schema.
DATABASE_URL='postgresql://user:pass@localhost:5432/app?sslmode=disable'
jet -dsn="$DATABASE_URL" -schema=public -path="$tmp/gen"
```

Use Jet when schema-derived builder types reduce risk more than they add
workflow cost. Keep generated imports at the application edge and use
`sqlkit.WithTx` or the application's transaction boundary around execution.

## Atlas Migration Boundary

Atlas is the recommended external migration tool boundary for the first SQL
milestone. It can plan migration files from a desired schema and apply pending
versioned migrations, but bluetape-go should not wrap those commands or hide
schema changes behind repository helpers.

```bash
tmp=.tmp/sql-guidance/atlas
rm -rf "$tmp"
mkdir -p "$tmp/schema" "$tmp/migrations"

cat > "$tmp/schema/schema.sql" <<'SQL'
create table accounts (
  id bigint primary key,
  name text not null
);
SQL

# DEV_DATABASE_URL should point to a disposable dev database.
atlas migrate diff create_accounts \
  --dir "file://$tmp/migrations" \
  --to "file://$tmp/schema/schema.sql" \
  --dev-url "$DEV_DATABASE_URL"
```

Application deployment can run `atlas migrate apply` against its database URL.
Keep that workflow in application CI/CD or operator runbooks, not inside
`sqlkit`. Migration linting is also an Atlas workflow, but current Atlas docs
describe `atlas migrate lint` as an authenticated Pro feature; keep it in
project CI only when that account boundary is explicit.

## References

- [Issue #100 research note](research/2026-06-26-issue-100-sql-repository-scope.md)
- [Issue #101 relational SQL epic](https://github.com/bluetape4k/bluetape-go/issues/101)
- [sqlc documentation](https://docs.sqlc.dev/)
- [Jet generator documentation](https://github.com/go-jet/jet/wiki/Generator)
- [Atlas migration planning](https://atlasgo.io/versioned/diff)
- [Atlas migration linting](https://atlasgo.io/versioned/lint)
- [Atlas migration apply](https://atlasgo.io/versioned/apply)
