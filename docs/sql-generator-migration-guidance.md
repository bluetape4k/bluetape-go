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

## Integration Policy

| Tool | Documentation and examples | Optional adapter | Core dependency |
|---|---|---|---|
| GORM | Show caller-owned pool initialization and ORM-owned transactions. | Consider only a separate package after repeated caller evidence and dependency approval. | Reject exposing `*gorm.DB`, models, hooks, or sessions from core. |
| ent | Show a caller-owned `*sql.DB`, generated-client isolation, and ent-owned transactions. | Consider only a narrow application adapter with lifecycle tests. | Reject generated entities, clients, schema, or privacy hooks in core. |
| Bun | Show a caller-owned pool and queries bound to an existing `*sql.Tx`. | Consider only when a stable application boundary cannot use `database/sql`. | Reject `bun.DB`, `bun.Tx`, models, or hooks in core. |
| sqlc | Show application-owned generated packages, `DBTX`, and `Queries.WithTx`. | Prefer a caller-owned narrow interface over a bluetape-go adapter. | Reject imports of generated application queries in core. |
| Jet | Show isolated generator output and statement execution with `*sql.DB` or `*sql.Tx`. | Prefer a caller-owned wrapper. | Reject generated tables/models or the Jet runtime in core. |
| Atlas | Show migration planning and apply in application CI/CD and runbooks. | Not a runtime adapter candidate. | Reject wrapping Atlas execution or migration state in `sqlkit`. |

Documentation is not a runtime compatibility guarantee. An optional adapter
requires a separate issue, comparative dependency evidence, lifecycle and error
contracts, tests, and approval. This guide adds none.

## Runtime-First Boundary

Provider and repository APIs accept the smallest standard boundary they need,
not ORM state merely because the application uses an ORM.

| Work | Accepted handle | Ownership |
|---|---|---|
| Shared direct SQL or `sqlkit` work | `sqlkit.Session` | The caller owns `*sql.DB` or `*sql.Tx`; the callee never closes the pool or commits/rolls back the transaction. |
| Pool-level work that may start a transaction | `*sql.DB` or `sqlkit.Beginner` | The caller owns the pool; a helper owns only a transaction it starts. |
| Work inside an existing transaction | `*sql.Tx` | The layer that calls `BeginTx` is the only commit/rollback owner. |
| Generated package | Caller-owned narrow interface or transaction-bound generated handle | The application owns generation, package location, and transaction binding. |
| ORM lifecycle | ORM client or session in the application | Never pass ORM state to provider APIs. |

Using the same `*sql.DB` means sharing a connection pool, not sharing an active
transaction. Claim atomicity only when every participant is bound to the same
transaction through a documented public API. Otherwise split the work or let
the framework own the full unit of work. Never imply atomicity across different
frameworks.

Keep generated code in application-owned packages such as `internal/db/sqlc`
or `internal/db/jet`, never in core packages. Commit generated source only when
the application owns that package and review policy accepts it. Never commit
scratch output under `.tmp`.

### Standard session boundary

```go
func LoadAccount(ctx context.Context, session sqlkit.Session, id int64) (Account, error) {
    var account Account
    err := session.QueryRowContext(
        ctx,
        "select id, name from accounts where id = $1",
        id,
    ).Scan(&account.ID, &account.Name)
    return account, err
}
```

See the compile-checked example in
[`sqlkit/orm_boundary_example_test.go`](../sqlkit/orm_boundary_example_test.go).

### GORM: share the pool, keep ORM transactions in the application

```go
gormDB, err := gorm.Open(mysql.New(mysql.Config{
    Conn: sqlDB,
}), &gorm.Config{})
```

This shares only the pool, not a pre-existing `*sql.Tx`. Use GORM's documented
transaction callback or session APIs. Pass standard handles to providers only
when the application can bind the exact same transaction through a supported
public API.

### ent: isolate generated clients and let ent own ent transactions

```go
drv := entsql.OpenDB(dialect.Postgres, sqlDB)
client := ent.NewClient(ent.Driver(drv))
```

An ent transaction's `tx.Client()` belongs only in application code that
already accepts `*ent.Client`; never pass it to provider APIs.

### Bun: bind a query to a caller-owned transaction

```go
bunDB := bun.NewDB(sqlDB, pgdialect.New())

tx, err := sqlDB.BeginTx(ctx, nil)
if err != nil {
    return err
}
defer tx.Rollback()

_, err = bunDB.NewInsert().
    Conn(tx).
    Model(account).
    Exec(ctx)
if err != nil {
    return err
}
return tx.Commit()
```

The caller of `BeginTx` is the sole commit/rollback owner. Alternatively,
Bun's `RunInTx` keeps the full unit of work inside its callback.

### sqlc: bind generated queries to `*sql.Tx`

```go
func LoadGeneratedAccount(
    ctx context.Context,
    db *sql.DB,
    queries *accountsqlc.Queries,
    id int64,
) (Account, error) {
    var account Account
    err := sqlkit.WithTx(ctx, db, nil, func(ctx context.Context, tx *sql.Tx) error {
        row, err := queries.WithTx(tx).GetAccount(ctx, id)
        if err != nil {
            return err
        }
        account = Account{ID: row.ID, Name: row.Name}
        return nil
    })
    return account, err
}
```

The compile-checked example uses an application-owned binder and narrow
interface; core imports no generated code.

### Jet: keep generated imports at the application edge

```go
tx, err := db.BeginTx(ctx, nil)
if err != nil {
    return err
}
defer tx.Rollback()

var accounts []model.Account
if err := stmt.QueryContext(ctx, tx, &accounts); err != nil {
    return err
}
return tx.Commit()
```

Generated imports and generator output remain outside core, and the code that
calls `BeginTx` owns transaction completion.

### Atlas: keep migration ownership outside runtime providers

Atlas has no runtime handle in provider or repository APIs. Migration diff,
lint, and apply remain in application CI/CD or operator runbooks. Runtime
repositories assume the schema is ready and do not invoke Atlas.

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
- [GORM existing database connection](https://gorm.io/docs/connecting_to_the_database.html)
- [ent sql.DB integration](https://entgo.io/docs/sql-integration/)
- [ent transactions](https://entgo.io/docs/transactions/)
- [Bun existing transaction integration](https://bun.uptrace.dev/guide/golang-orm.html#using-bun-with-existing-code)
- [Bun transactions](https://bun.uptrace.dev/guide/transactions.html)
- [sqlc transactions](https://docs.sqlc.dev/en/latest/howto/transactions.html)
- [Jet generator documentation](https://github.com/go-jet/jet/wiki/Generator)
- [Jet transaction execution](https://github.com/go-jet/jet/wiki/FAQ#how-to-execute-jet-statement-in-sql-transaction)
- [Atlas migration planning](https://atlasgo.io/versioned/diff)
- [Atlas migration linting](https://atlasgo.io/versioned/lint)
- [Atlas migration apply](https://atlasgo.io/versioned/apply)
