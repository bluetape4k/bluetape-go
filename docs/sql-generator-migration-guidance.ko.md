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

## Runtime-First 경계

- business repository는 명시적으로 유지합니다. `context.Context`와 caller-owned
  `sqlkit.Session`, `*sql.DB`, `*sql.Tx`, generated query handle을 받습니다.
- generated code는 `internal/db/sqlc` 또는 `internal/db/jet` 같은 application-owned
  package에 둡니다. `sqlkit`에는 넣지 않습니다.
- application이 generated package를 소유하고 review policy가 허용할 때만
  generated source를 commit합니다. `.tmp` scratch output은 commit하지 않습니다.
- `sqlkit.WithTx`로 transaction boundary를 정하고, callback 안에서 직접 SQL,
  sqlc generated query, Jet statement, `sqlkit` statement를 호출합니다.

예시 repository 형태:

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

generated package도 같은 boundary를 사용할 수 있습니다. constructor 호출은
generator API에 맞추되, transaction 소유권은 `WithTx`에 둡니다.

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
- [Jet generator documentation](https://github.com/go-jet/jet/wiki/Generator)
- [Atlas migration planning](https://atlasgo.io/versioned/diff)
- [Atlas migration linting](https://atlasgo.io/versioned/lint)
- [Atlas migration apply](https://atlasgo.io/versioned/apply)
