# SQL Generator와 Migration 가이드

[English](sql-generator-migration-guidance.md) | [한국어](sql-generator-migration-guidance.ko.md)

이 문서는 relational SQL epic
[#101](https://github.com/bluetape4k/bluetape-go/issues/101)을 위해
[#100](https://github.com/bluetape4k/bluetape-go/issues/100)에서 선택한 optional
SQL generator와 migration 경계를 정리합니다. bluetape-go의 기본 방향은
runtime-first입니다. SQL은 보이게 두고, transaction boundary는 명시하며,
application이 선택하지 않는 한 generated code는 core package 밖에 둡니다.

## 선택 Matrix

| 필요 | 선택 | 경계 |
|---|---|---|
| driver 제어, 작은 query surface, driver-specific 동작 | 직접 `database/sql` 또는 `pgx` 같은 driver-native API | caller가 SQL, pool lifecycle, transaction, scanning을 소유합니다. |
| shared transaction, row mapping, cardinality, 간단한 PostgreSQL-first inspectable builder | [`sqlkit`](../sqlkit/README.ko.md) | core runtime helper입니다. schema metadata, migration, generated model, ORM state는 없습니다. |
| 안정적인 handwritten SQL과 generated type-safe Go method | [sqlc](https://docs.sqlc.dev/) | optional application workflow입니다. generated package는 core `sqlkit` 밖에 둡니다. |
| live database schema에서 생성한 type-safe SQL builder/model file | [Jet](https://github.com/go-jet/jet) | optional application workflow입니다. database/schema source와 격리된 output이 필요합니다. |
| `sqlkit`보다 큰 runtime query builder가 필요하지만 기본값으로 삼지 않음 | goqu | broad dialect/query-builder coverage가 필요한 application-level dependency입니다. |
| entity graph modeling, schema-as-code, generated data-access layer | ent | bluetape-go core에서는 보류합니다. entity modeling 자체가 필요할 때 application boundary에서 사용합니다. |
| ORM-style model lifecycle, hook, eager loading, active record 계열 workflow | Bun 또는 GORM | application-level 선택입니다. 첫 SQL milestone 기본값이 아니며 `sqlkit`이 감싸지 않습니다. |
| schema diff, migration file planning, optional linting, apply workflow | [Atlas](https://atlasgo.io/) | external tool boundary입니다. bluetape-go는 사용 경계만 문서화하고 migration 실행을 감싸지 않습니다. |

## 통합 정책

| 도구 | 문서와 예제 | Optional adapter | Core dependency |
|---|---|---|---|
| GORM | caller-owned pool 초기화와 ORM-owned transaction을 보여 줍니다. | 실제 caller의 반복된 요구와 dependency 승인을 확인한 뒤 별도 package로만 검토합니다. | core에서 `*gorm.DB`, model, hook, session을 노출하지 않습니다. |
| ent | caller-owned `*sql.DB`, generated client 격리, ent-owned transaction을 보여 줍니다. | lifecycle test를 갖춘 좁은 application adapter만 검토합니다. | core에서 generated entity, client, schema, privacy hook을 노출하지 않습니다. |
| Bun | caller-owned pool과 기존 `*sql.Tx`에 query를 연결하는 방법을 보여 줍니다. | 안정적인 application boundary가 `database/sql`을 사용할 수 없을 때만 검토합니다. | core에서 `bun.DB`, `bun.Tx`, model, hook을 노출하지 않습니다. |
| sqlc | application-owned generated package, `DBTX`, `Queries.WithTx`를 보여 줍니다. | bluetape-go adapter보다 caller-owned narrow interface를 우선합니다. | core에서 application의 generated query를 import하지 않습니다. |
| Jet | 격리된 generator output과 `*sql.DB` 또는 `*sql.Tx`를 사용한 statement 실행을 보여 줍니다. | caller-owned wrapper를 우선합니다. | core에서 generated table/model이나 Jet runtime을 사용하지 않습니다. |
| Atlas | application CI/CD와 runbook에서 migration을 계획하고 적용하는 방법을 보여 줍니다. | runtime adapter 대상이 아닙니다. | `sqlkit`에서 Atlas 실행이나 migration state를 감싸지 않습니다. |

문서와 예제 제공은 허용되지만 runtime compatibility를 보장하지는 않습니다.
Optional adapter는 별도 issue에서 해당 use case의 근거와 maintenance cost를
검토하고, 비교 가능한 dependency 근거와 승인, lifecycle/error contract와 test를
갖춰야 합니다. ORM/generated state나 runtime dependency를 core에 노출하는
방식은 허용하지 않습니다. 이 가이드는 어떤 adapter도 추가하지 않습니다.

## Runtime-First 경계

Provider와 repository API는 application이 ORM을 사용한다는 이유로 ORM state를
받지 않습니다. 필요한 범위에서 가장 작은 표준 경계만 받습니다.

| 작업 | 허용 handle | 소유권 |
|---|---|---|
| direct SQL 또는 `sqlkit` 작업 공유 | `sqlkit.Session` | caller가 `*sql.DB` 또는 `*sql.Tx`를 소유하며, callee는 pool을 닫거나 transaction을 commit/rollback하지 않습니다. |
| transaction을 시작할 수 있는 pool-level 작업 | `*sql.DB` 또는 `sqlkit.Beginner` | caller가 pool을 소유하며, helper는 자신이 시작한 transaction만 소유합니다. |
| 기존 transaction 안의 작업 | `*sql.Tx` | `BeginTx`를 호출한 계층만 commit/rollback을 소유합니다. |
| generated package | caller-owned narrow interface 또는 transaction-bound generated handle | application이 generation, package 위치, transaction 연결을 소유합니다. |
| ORM lifecycle | application 내부의 ORM client 또는 session | ORM state를 provider API에 전달하지 않습니다. 정확히 하나의 application composition-root shutdown owner만 wrapper 또는 shared pool을 닫을 수 있으며, feature/library code는 `Close`를 호출하지 않습니다. |

같은 `*sql.DB`를 사용한다는 사실은 connection pool을 공유한다는 뜻일 뿐, 실행
중인 transaction을 공유한다는 뜻은 아닙니다. 한 계층이 transaction을 시작하고
모든 참여자가 공식 public API를 통해 같은 transaction에 연결될 때만 atomicity를
보장할 수 있습니다. Framework가 표준 transaction 경계를 연결할 수 없다면 작업을
분리하거나 framework가 전체 unit of work를 소유하게 하세요. 서로 다른 framework
사이의 atomicity를 암시해서는 안 됩니다.

generated code는 `internal/db/sqlc` 또는 `internal/db/jet` 같은
application-owned package에 두고 core package에는 넣지 않습니다. application이
해당 package를 소유하고 review policy가 허용할 때만 generated source를
commit합니다. `.tmp`의 scratch output은 절대 commit하지 않습니다.

### 표준 session 경계

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

compile-checked 예제는
[`sqlkit/orm_boundary_example_test.go`](../sqlkit/orm_boundary_example_test.go)에서
확인할 수 있습니다.

### GORM: pool은 공유하고 ORM transaction은 application에서 소유

```go
gormDB, err := gorm.Open(mysql.New(mysql.Config{
    Conn: sqlDB,
}), &gorm.Config{})
```

이 코드는 pool만 공유하며, 이미 실행 중인 `*sql.Tx`를 공유하지 않습니다. GORM의
[공식 transaction callback](https://gorm.io/docs/transactions.html) 또는 session
API를 사용하세요. application이 지원되는 public API를 통해 정확히 같은
transaction에 연결할 수 있을 때만 표준 handle을 provider에 전달합니다.

### ent: generated client를 격리하고 ent transaction은 ent가 소유

```go
drv := entsql.OpenDB(dialect.Postgres, sqlDB)
client := ent.NewClient(ent.Driver(drv))
```

ent transaction의 `tx.Client()`는 이미 `*ent.Client`를 받는 application code에서만
사용합니다. provider API에는 절대 전달하지 않습니다. `client.Close()`는 underlying
driver를 닫으므로 정확히 하나의 application composition-root shutdown owner만
client 또는 shared pool을 닫을 수 있습니다. Pool을 공유할 때 feature/library
code는 `client.Close()`를 호출해서는 안 됩니다.

### Bun: query를 caller-owned transaction에 연결

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

`BeginTx`를 호출한 caller만 commit/rollback을 소유합니다. Bun의 `RunInTx`를
사용한다면 전체 unit of work를 callback 안에 둡니다. `bunDB.Close()`는 underlying
`*sql.DB`를 닫으므로 정확히 하나의 application composition-root shutdown owner만
wrapper 또는 shared pool을 닫을 수 있습니다. Pool을 공유할 때 feature/library
code는 `bunDB.Close()`를 호출해서는 안 됩니다.

### sqlc: generated query를 `*sql.Tx`에 연결

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

compile-checked 예제는 application-owned binder와 narrow interface를 사용하며,
core는 generated code를 import하지 않습니다.

### Jet: generated import를 application edge에 유지

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

generated import와 generator output은 core 밖에 유지하며, `BeginTx`를 호출한
코드가 transaction 완료를 소유합니다.

### Atlas: migration 소유권을 runtime provider 밖에 유지

Atlas runtime handle은 provider 또는 repository API에 존재하지 않습니다.
Migration diff, lint, apply는 application CI/CD 또는 operator runbook에 둡니다.
Runtime repository는 schema가 준비되었다고 가정하며 Atlas를 호출하지 않습니다.

## 격리된 sqlc 예제

sqlc는 optional입니다. sqlc 문서는 `sqlc generate`가 `sqlc.yaml`에 설정된
schema/query SQL path를 읽고 Go code를 생성한다고 설명합니다. 이 workflow는
scratch directory 또는 application-owned package 안에서만 사용합니다.

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

생성 후에는 application이 package를 명시적으로 소유할 때만 output을 옮깁니다.
그 외에는 `.tmp` 아래에 두고 삭제합니다.

## 격리된 Jet 예제

Jet은 optional입니다. Jet generator는 database에 연결해 schema metadata를 읽고
SQL builder/model file을 destination path에 생성합니다. Jet은 generated
destination 내용을 정리하므로 output directory를 격리합니다.

```bash
tmp=.tmp/sql-guidance/jet
rm -rf "$tmp"
mkdir -p "$tmp/gen"

# 대상 schema가 있는 실행 중인 PostgreSQL database가 필요합니다.
DATABASE_URL='postgresql://user:pass@localhost:5432/app?sslmode=disable'
jet -dsn="$DATABASE_URL" -schema=public -path="$tmp/gen"
```

schema-derived builder type이 workflow 비용보다 더 큰 위험 감소를 제공할 때만
Jet을 사용합니다. generated import는 application edge에 두고,
`sqlkit.WithTx` 또는 application의 transaction boundary 안에서 실행합니다.

## Atlas Migration 경계

첫 SQL milestone에서 Atlas는 권장 external migration tool boundary입니다. Atlas는
desired schema에서 migration file을 계획하고 pending versioned migration을
적용할 수 있지만, bluetape-go는 이 명령을 감싸거나 repository helper 뒤로 schema
변경을 숨기지 않습니다.

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

# DEV_DATABASE_URL은 disposable dev database를 가리켜야 합니다.
atlas migrate diff create_accounts \
  --dir "file://$tmp/migrations" \
  --to "file://$tmp/schema/schema.sql" \
  --dev-url "$DEV_DATABASE_URL"
```

application deployment는 자체 database URL에 대해 `atlas migrate apply`를 실행할
수 있습니다. 이 workflow는 `sqlkit` 내부가 아니라 application CI/CD 또는 operator
runbook에 둡니다. migration linting도 Atlas workflow이지만, 현재 Atlas 문서는
`atlas migrate lint`를 authenticated Pro feature로 설명합니다. 따라서 계정 경계가
명확한 project CI에서만 사용합니다.

## 참고

- [Issue #100 research note](research/2026-06-26-issue-100-sql-repository-scope.md)
- [Issue #101 relational SQL epic](https://github.com/bluetape4k/bluetape-go/issues/101)
- [sqlc documentation](https://docs.sqlc.dev/)
- [GORM 기존 database connection](https://gorm.io/docs/connecting_to_the_database.html)
- [GORM transaction](https://gorm.io/docs/transactions.html)
- [ent sql.DB 통합](https://entgo.io/docs/sql-integration/)
- [ent transaction](https://entgo.io/docs/transactions/)
- [Bun 기존 transaction 통합](https://bun.uptrace.dev/guide/golang-orm.html#using-bun-with-existing-code)
- [Bun transaction](https://bun.uptrace.dev/guide/transactions.html)
- [sqlc transaction](https://docs.sqlc.dev/en/latest/howto/transactions.html)
- [Jet generator documentation](https://github.com/go-jet/jet/wiki/Generator)
- [Jet transaction 실행](https://github.com/go-jet/jet/wiki/FAQ#how-to-execute-jet-statement-in-sql-transaction)
- [Atlas migration planning](https://atlasgo.io/versioned/diff)
- [Atlas migration linting](https://atlasgo.io/versioned/lint)
- [Atlas migration apply](https://atlasgo.io/versioned/apply)
