# Issue #531 ORM 및 Generated SQL Boundaries 구현 계획

> 한국어 재작성 범위: 이 계획 문서는 한국어 운영 문서로 읽히도록 제목, 판단, 작업 설명, 위험, 검증, 롤백 문맥을 한국어로 정리한다. 명령, 경로, API 이름, 이슈/PR 번호, 브랜치명, 코드 블록, 테스트 출력 같은 증거 문자열은 정확성을 위해 원문 그대로 보존한다.


> **에이전트 작업자용:** 필수 하위 스킬: 사용 superpowers:subagent-driven-development (권장) 또는 superpowers:executing-plans to 이 계획을 작업 단위로 구현. 단계는 checkbox (`- [ ]`) 추적 문법을 사용.

**목표:** Clarify how applications combine bluetape-go 함께 GORM, ent, Bun, sqlc, Jet, 및 Atlas while keeping ORM state 및 optional dependencies outside core packages.

**아키텍처:** 유지 production packages unchanged. 추가 dependency-free external-패키지 example that compile the 공유 `sqlkit.Session` 및 transaction-bound generated-handle patterns, then expand the 기존 영문/한국어 SQL generator guide 함께 a policy matrix 및 upstream-verified integration snippets. Treat pool sharing, transaction sharing, 및 migration ownership as separate contracts.

**기술 스택:** Go 1.26, `database/sql`, 기존 `sqlkit` interfaces, Go example, bilingual Markdown, official GORM/ent/Bun/sqlc/Jet/Atlas documentation, GitHub CLI.

---

## 실행 제약

- Work 만 in `/Users/debop/work/bluetape4k/bluetape-go/.worktrees/docs-issue-531-orm-boundary-examples` on `docs/issue-531-orm-boundary-examples`.
- Design authority: `docs/superpowers/specs/2026-07-14-issue-531-orm-generated-sql-boundaries-design.md` at commit `ff23d6d`.
- 유지 production behavior unchanged. No production `.go` file, `go.mod`, `go.sum`, schema, migration, workf낮음, diagram, 또는 generated source may change.
- 다음을 하지 않는다: add GORM, ent, Bun, sqlc, Jet, Atlas, goqu, 또는 Bob dependencies.
- 사용 `apply_patch` for file edits. 사용 `gofmt` 만 for the new Go example file.
- Public documentation, commits, 및 PR metadata are 영문. 유지 the 한국어 guide source-equivalent 및 natural rather than literal.
- 검증 external API names against official/upstream sources immediately 전에 editing. 다음을 하지 않는다: document internal fields 또는 unsupported transaction binding.
- 실행 heavyweight checks serially. Stop 전에 merge; CI green 만 authorizes a merge-approval request.
- CodeGraph is 아님 exposed in this session. Direct source inspection is the recorded fallback for `sqlkit.Session` 및 `sqlkit.WithTx`.

## 파일 지도

| Path | 책임 |
|---|---|
| `sqlkit/orm_boundary_example_test.go` | Compile-check standard session 및 application-owned generated-handle boundaries without optional dependencies |
| `docs/sql-generator-migration-guidance.md` | 영문 policy matrix, ownership rules, 및 upstream-backed tool example |
| `docs/sql-generator-migration-guidance.ko.md` | Natural 한국어 source-equivalent guide |
| `docs/superpowers/specs/2026-07-14-issue-531-orm-generated-sql-boundaries-design.md` | Approved design authority; 없음 implementation edits expected |
| `docs/superpowers/plans/2026-07-14-issue-531-orm-generated-sql-boundaries-plan.md` | This execution 계약 |

`sqlkit/README.md` 및 `sqlkit/README.ko.md` already link their locale-specific guide under the selection section. 검증 those links, but do 아님 edit the README pair unless the link is missing 또는 incorrect; a scope expansion requires a revised plan.

### 작업 1: 추가 dependency-free compile example

**파일:**
- 생성: `sqlkit/orm_boundary_example_test.go`

This maintenance task adds 없음 production behavior, so manufacturing a RED production 테스트 is N/A. The gate is that the new 공개 example compiles against the 기존 `sqlkit` contracts without adding a dependency.

- [ ] **단계 1: 생성 the standard session 및 generated-handle example**

생성 `sqlkit/orm_boundary_example_test.go` 함께 this exact content:

```go
package sqlkit_test

import (
	"context"
	"database/sql"

	"github.com/bluetape4k/bluetape-go/sqlkit"
)

var (
	_ sqlkit.Session = (*sql.DB)(nil)
	_ sqlkit.Session = (*sql.Tx)(nil)
)

type boundaryAccount struct {
	ID   int64
	Name string
}

type generatedAccountQueries interface {
	GetAccount(context.Context, int64) (boundaryAccount, error)
}

type generatedAccountBinder func(*sql.Tx) generatedAccountQueries

func ExampleSession_repositoryBoundary() {
	load := func(ctx context.Context, session sqlkit.Session, id int64) (boundaryAccount, error) {
		var account boundaryAccount
		err := session.QueryRowContext(
			ctx,
			"select id, name from accounts where id = $1",
			id,
		).Scan(&account.ID, &account.Name)
		return account, err
	}

	fromPool := func(ctx context.Context, db *sql.DB, id int64) (boundaryAccount, error) {
		return load(ctx, db, id)
	}
	fromTransaction := func(ctx context.Context, tx *sql.Tx, id int64) (boundaryAccount, error) {
		return load(ctx, tx, id)
	}

	_, _, _ = load, fromPool, fromTransaction
}

func ExampleWithTx_generatedQueryHandle() {
	load := func(
		ctx context.Context,
		db *sql.DB,
		bind generatedAccountBinder,
		id int64,
	) (boundaryAccount, error) {
		var account boundaryAccount
		err := sqlkit.WithTx(ctx, db, nil, func(ctx context.Context, tx *sql.Tx) error {
			queries := bind(tx)
			var err error
			account, err = queries.GetAccount(ctx, id)
			return err
		})
		return account, err
	}

	_ = load
}
```

- [ ] **단계 2: Format 및 compile the example**

실행:

```bash
gofmt -w sqlkit/orm_boundary_example_test.go
go test -count=1 ./sqlkit -run '^(ExampleSession_repositoryBoundary|ExampleWithTx_generatedQueryHandle)$'
```

예상: `ok github.com/bluetape4k/bluetape-go/sqlkit`; `go.mod` 및 `go.sum` remain unchanged.

- [ ] **단계 3: 리뷰 the example boundary**

실행:

```bash
rg -n 'gorm|entgo|uptrace|go-jet|atlasgo|sqlc' sqlkit/orm_boundary_example_test.go
git diff --check
git diff -- sqlkit/orm_boundary_example_test.go
```

예상: the dependency search returns 없음 matches; the diff contains 만 external-패키지 example, standard-library imports, 및 the 기존 `sqlkit` import; diff check exits 0.

- [ ] **단계 4: 커밋 the compile example**

```bash
git add sqlkit/orm_boundary_example_test.go
git commit -m "docs: add SQL boundary compile examples"
```

예상: one new `_test.go` file is committed 및 the worktree is clean.

### 작업 2: Expand the 영문 boundary guide

**파일:**
- Modify: `docs/sql-generator-migration-guidance.md`

- [ ] **단계 1: Refresh the official source ledger**

Open 및 verify these exact primary sources:

```text
https://gorm.io/docs/connecting_to_the_database.html
https://entgo.io/docs/sql-integration/
https://entgo.io/docs/transactions/
https://bun.uptrace.dev/guide/golang-orm.html#using-bun-with-existing-code
https://bun.uptrace.dev/guide/transactions.html
https://docs.sqlc.dev/en/latest/howto/transactions.html
https://github.com/go-jet/jet/wiki/FAQ#how-to-execute-jet-statement-in-sql-transaction
https://github.com/go-jet/jet/wiki/Generator
https://atlasgo.io/versioned/diff
https://atlasgo.io/versioned/apply
```

Expected identifiers:

- GORM: `mysql.New(mysql.Config{Conn: sqlDB})` initializes GORM from 호출자-owned `*sql.DB`.
- ent: `entsql.OpenDB(dialect.Postgres, sqlDB)` 및 `tx.Client()` remain documented.
- Bun: `bun.NewDB(sqldb, pgdialect.New())` 및 `.Conn(tx)` remain documented; `bun.Tx` embeds `*sql.Tx`.
- sqlc: generated `DBTX`, `Queries.WithTx(*sql.Tx)`, 및 returned transaction-bound `*Queries` remain documented.
- Jet: `stmt.QueryContext(ctx, tx, &dest)` 및 `stmt.ExecContext(ctx, tx)` remain documented.
- Atlas: `migrate diff` 및 `migrate apply` remain external CLI workf낮음s.

If an identifier has changed, update both locale snippets to the currently documented 공개 form 전에 continuing. Never substitute an internal field.

- [ ] **단계 2: Insert the policy matrix 후 `## Selection Matrix`**

After the current selection table 및 전에 `## Runtime-First Boundary`, insert:

```markdown
## Integration Policy

| Tool | Documentation and examples | Optional adapter | Core dependency |
|---|---|---|---|
| GORM | Show caller-owned pool initialization and ORM-owned transaction boundaries. | Consider only in a separate package after repeated caller evidence and dependency approval. | Rejected: do not expose `*gorm.DB`, models, hooks, or sessions from core providers. |
| ent | Show caller-owned `*sql.DB`, generated client isolation, and ent-owned transactions. | Consider only for a narrow application adapter with independent lifecycle tests. | Rejected: do not expose generated entities, clients, schema, or privacy hooks from core providers. |
| Bun | Show caller-owned pool setup and binding Bun queries to an existing `*sql.Tx`. | Consider only when a stable application boundary cannot use `database/sql` directly. | Rejected: do not expose `bun.DB`, `bun.Tx`, models, or hooks from core providers. |
| sqlc | Show application-owned generated packages, `DBTX`, and `Queries.WithTx`. | A caller-owned narrow interface is preferred to a bluetape-go adapter. | Rejected: do not generate or import application query packages in core. |
| Jet | Show isolated generator output and statement execution with `*sql.DB` or `*sql.Tx`. | A caller-owned statement/query wrapper is preferred to a core adapter. | Rejected: do not import generated tables/models or Jet runtime packages in core. |
| Atlas | Show migration planning and apply commands in application CI/CD or runbooks. | Not a runtime adapter candidate. | Rejected: do not wrap Atlas execution or migration state in `sqlkit`. |

Documentation does not imply a runtime compatibility guarantee. An optional
adapter requires a separate issue, comparative dependency evidence, lifecycle
and error contracts, tests, and explicit approval. No adapter is added by this
guide.
```

- [ ] **단계 3: 교체 the runtime-first section 함께 explicit handle ownership**

교체 the 기존 `## Runtime-First Boundary` section through the line 전에 `## Isolated sqlc Example` 함께:

````markdown
## Runtime-First Boundary

Provider and repository APIs should accept the smallest standard boundary that
matches their work. They should not accept ORM-specific state merely because an
application uses an ORM elsewhere.

| Caller situation | Accept | Ownership |
|---|---|---|
| Shared direct SQL or `sqlkit` query code | `sqlkit.Session` | The caller owns the concrete `*sql.DB` or `*sql.Tx`; the callee never closes or commits it. |
| Pool-level work that may start a transaction | `*sql.DB` or `sqlkit.Beginner` | The caller owns pool configuration and close; the transaction helper owns only transactions it starts. |
| Work inside an existing transaction | `*sql.Tx` | The layer that called `BeginTx` is the only commit/rollback owner. |
| Generated query package | A caller-owned narrow interface or transaction-bound generated handle | The application owns generation, package location, and binding to `*sql.DB` or `*sql.Tx`. |
| ORM lifecycle work | The ORM's client/session inside the application layer | Do not pass ORM state into bluetape-go providers. |

Sharing one `*sql.DB` shares a pool, not an active transaction. Claim atomicity
only when one layer starts the transaction and every participant is bound to
that same transaction through a documented public API. If a framework cannot
bind the standard transaction boundary, split the work or let the framework own
the complete unit of work; do not imply cross-framework atomicity.

Keep generated code in application-owned packages such as `internal/db/sqlc`
or `internal/db/jet`, not in `sqlkit`. Commit generated source only when the
application owns that package and its review policy accepts generated code.
Never commit scratch output under `.tmp`.

### Standard session boundary

Use `sqlkit.Session` when the same repository function should work with either
`*sql.DB` or `*sql.Tx`:

```go
func LoadAccount(ctx context.Context, session sqlkit.Session, id int64) (Account, 오류) {
    var account Account
    err := session.QueryRowContext(
        ctx,
        "select id, name from accounts where id = $1",
        id,
    ).Scan(&account.ID, &account.Name)
    return account, err
}
```

The compile-checked form lives in
[`sqlkit/orm_boundary_example_test.go`](../sqlkit/orm_boundary_example_test.go).

### GORM: share the pool, keep ORM transactions in the application

GORM can initialize from a caller-owned `*sql.DB` using a driver-specific
configuration:

```go
gormDB, err := gorm.Open(mysql.New(mysql.Config{
    Conn: sqlDB,
}), &gorm.Config{})
```

This shares the pool only. It does not bind a pre-existing `*sql.Tx`. Keep a
GORM-owned unit of work inside GORM's documented transaction callback/session,
and pass standard handles to bluetape-go providers only when the application
can bind the exact same transaction through a supported public API.

### ent: isolate generated clients and let ent own ent transactions

Create an ent SQL driver from the caller-owned pool, then keep the generated
client in the application package:

```go
drv := entsql.OpenDB(dialect.Postgres, sqlDB)
client := ent.NewClient(ent.Driver(drv))
```

For an ent-owned transaction, pass `tx.Client()` only to application code that
already accepts `*ent.Client`. Do not pass `*ent.Client`, `*ent.Tx`, or generated
entities into bluetape-go provider APIs.

### Bun: bind a query to a caller-owned transaction

Bun accepts a caller-owned pool and can bind a query builder to an existing
`*sql.Tx`:

```go
bunDB := bun.NewDB(sqlDB, pgdialect.New())

tx, err := sqlDB.BeginTx(ctx, nil)
if err != nil {
    return err
}
defer tx.롤백()

_, err = bunDB.NewInsert().
    Conn(tx).
    Model(account).
    Exec(ctx)
if err != nil {
    return err
}
return tx.커밋()
```

The code that calls `BeginTx` remains the sole commit/rollback owner. When Bun
starts the transaction with `RunInTx`, keep the whole unit of work inside its
callback and use the embedded standard transaction only through documented Bun
behavior.

### sqlc: bind generated queries to `*sql.Tx`

Keep generated `Queries` in the application and bind them inside the
transaction owner:

```go
func LoadGeneratedAccount(
    ctx context.Context,
    db *sql.DB,
    queries *accountsqlc.Queries,
    id int64,
) (Account, 오류) {
    var account Account
    err := sqlkit.WithTx(ctx, db, nil, func(ctx context.Context, tx *sql.Tx) 오류 {
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

The compile-checked guide example uses an application-owned binder and narrow
query interface so bluetape-go does not import generated code.

### Jet: keep generated imports at the application edge

Jet statements accept the standard transaction at execution time:

```go
tx, err := db.BeginTx(ctx, nil)
if err != nil {
    return err
}
defer tx.롤백()

var accounts []model.Account
if err := stmt.QueryContext(ctx, tx, &accounts); err != nil {
    return err
}
return tx.커밋()
```

Keep generated table/model imports and generator output outside core packages.
The code that starts the transaction owns commit and rollback.

### Atlas: keep migration ownership outside runtime providers

Atlas has no runtime query handle to pass into `sqlkit`. Keep `migrate diff`,
lint policy, and `migrate apply` in application CI/CD or operator runbooks.
Runtime repositories should assume the required schema version already exists
and should not invoke Atlas commands.
````

- [ ] **단계 4: Extend the reference list**

추가 these official sources to `## References`, preserving the 기존 entries:

```markdown
- [GORM existing database connection](https://gorm.io/docs/connecting_to_the_database.html)
- [ent `sql.DB` integration](https://entgo.io/docs/sql-integration/)
- [ent transactions](https://entgo.io/docs/transactions/)
- [Bun integration with existing transactions](https://bun.uptrace.dev/guide/golang-orm.html#using-bun-with-existing-code)
- [Bun transactions](https://bun.uptrace.dev/guide/transactions.html)
- [sqlc transactions](https://docs.sqlc.dev/en/latest/howto/transactions.html)
- [Jet transaction execution](https://github.com/go-jet/jet/wiki/FAQ#how-to-execute-jet-statement-in-sql-transaction)
```

예상: every technical identifier in the new 영문 section is backed by one of these primary sources 또는 an 기존 Atlas/Jet source.

### 작업 3: 추가 the source-equivalent 한국어 guide

**파일:**
- Modify: `docs/sql-generator-migration-guidance.ko.md`
- Modify: `docs/sql-generator-migration-guidance.md` 만 for parity corrections found during this task

- [ ] **단계 1: Insert the 한국어 policy matrix 후 `## 선택 Matrix`**

After the current selection table 및 전에 `## Runtime-First 경계`, insert:

```markdown
## 통합 정책

| 도구 | 문서와 예제 | Optional adapter | Core dependency |
|---|---|---|---|
| GORM | Caller-owned pool 초기화와 ORM-owned transaction 경계를 보여 줍니다. | 반복되는 caller evidence와 dependency 승인이 있는 별도 package에서만 검토합니다. | 거부: core provider에서 `*gorm.DB`, model, hook, session을 노출하지 않습니다. |
| ent | Caller-owned `*sql.DB`, generated client 격리, ent-owned transaction을 보여 줍니다. | 독립적인 lifecycle test가 있는 좁은 application adapter에서만 검토합니다. | 거부: core provider에서 generated entity, client, schema, privacy hook을 노출하지 않습니다. |
| Bun | Caller-owned pool 설정과 Bun query를 기존 `*sql.Tx`에 연결하는 방법을 보여 줍니다. | 안정적인 application 경계를 `database/sql`로 직접 표현할 수 없을 때만 검토합니다. | 거부: core provider에서 `bun.DB`, `bun.Tx`, model, hook을 노출하지 않습니다. |
| sqlc | Application-owned generated package, `DBTX`, `Queries.WithTx`를 보여 줍니다. | bluetape-go adapter보다 caller-owned narrow interface를 우선합니다. | 거부: core에서 application query package를 생성하거나 import하지 않습니다. |
| Jet | 격리된 generator output과 `*sql.DB` 또는 `*sql.Tx`를 이용한 statement 실행을 보여 줍니다. | Core adapter보다 caller-owned statement/query wrapper를 우선합니다. | 거부: core에서 generated table/model 또는 Jet runtime package를 import하지 않습니다. |
| Atlas | Application CI/CD 또는 runbook에서 migration plan과 apply 명령을 사용하는 방법을 보여 줍니다. | Runtime adapter 검토 대상이 아닙니다. | 거부: `sqlkit`이 Atlas 실행이나 migration state를 감싸지 않습니다. |

문서에 통합 방법이 있다는 사실이 runtime compatibility를 보장하지는 않습니다. Optional
adapter를 추가하려면 별도 issue, dependency 비교 근거, lifecycle과 error 계약, test,
명시적인 승인이 필요합니다. 이 가이드에서는 adapter를 추가하지 않습니다.
```

- [ ] **단계 2: 교체 the 한국어 runtime-first section 함께 explicit ownership**

교체 the 기존 `## Runtime-First 경계` section through the line 전에 `## 격리된 sqlc 예제` 함께:

````markdown
## Runtime-First 경계

Provider와 repository API는 작업에 필요한 가장 작은 표준 경계를 받아야 합니다.
Application의 다른 부분에서 ORM을 사용한다는 이유만으로 ORM-specific state를 받으면
안 됩니다.

| Caller 상황 | 받을 경계 | 소유권 |
|---|---|---|
| Direct SQL 또는 `sqlkit` query code 공유 | `sqlkit.Session` | Caller가 concrete `*sql.DB` 또는 `*sql.Tx`를 소유합니다. Callee는 이를 close하거나 commit하지 않습니다. |
| Transaction을 시작할 수 있는 pool-level 작업 | `*sql.DB` 또는 `sqlkit.Beginner` | Caller가 pool 설정과 close를 소유합니다. Transaction helper는 자신이 시작한 transaction만 소유합니다. |
| 기존 transaction 안의 작업 | `*sql.Tx` | `BeginTx`를 호출한 계층만 commit/rollback합니다. |
| Generated query package | Caller-owned narrow interface 또는 transaction-bound generated handle | Application이 generation, package 위치, `*sql.DB`/`*sql.Tx` binding을 소유합니다. |
| ORM lifecycle 작업 | Application layer 안의 ORM client/session | ORM state를 bluetape-go provider에 전달하지 않습니다. |

같은 `*sql.DB`를 사용한다는 사실은 connection pool을 공유한다는 뜻일 뿐, 실행 중인
transaction을 공유한다는 뜻은 아닙니다. 한 계층이 transaction을 시작하고 모든 참여자가
공식 public API를 통해 같은 transaction에 연결될 때만 atomicity를 보장할 수 있습니다.
Framework가 표준 transaction 경계를 연결할 수 없다면 작업을 분리하거나 framework가
전체 unit of work를 소유하게 하세요. 서로 다른 framework 사이의 atomicity를 암시해서는
안 됩니다.

Generated code는 `sqlkit`이 아니라 `internal/db/sqlc` 또는 `internal/db/jet` 같은
application-owned package에 둡니다. Application이 generated package를 소유하고 review
policy가 generated code를 허용할 때만 source를 commit합니다. `.tmp`의 scratch output은
commit하지 않습니다.

### 표준 session 경계

같은 repository function을 `*sql.DB`와 `*sql.Tx`에서 모두 사용하려면
`sqlkit.Session`을 받습니다.

```go
func LoadAccount(ctx context.Context, session sqlkit.Session, id int64) (Account, 오류) {
    var account Account
    err := session.QueryRowContext(
        ctx,
        "select id, name from accounts where id = $1",
        id,
    ).Scan(&account.ID, &account.Name)
    return account, err
}
```

Compile-checked 형태는
[`sqlkit/orm_boundary_example_test.go`](../sqlkit/orm_boundary_example_test.go)에 있습니다.

### GORM: pool은 공유하고 ORM transaction은 application에서 소유

GORM은 driver별 설정을 사용해 caller-owned `*sql.DB`로 초기화할 수 있습니다.

```go
gormDB, err := gorm.Open(mysql.New(mysql.Config{
    Conn: sqlDB,
}), &gorm.Config{})
```

이 방식은 pool만 공유하며 기존 `*sql.Tx`를 연결하지는 않습니다. GORM-owned unit of
work는 GORM이 문서화한 transaction callback/session 안에 둡니다. Application이 지원되는
public API로 정확히 같은 transaction을 연결할 수 있을 때만 표준 handle을 bluetape-go
provider에 전달합니다.

### ent: generated client를 격리하고 ent transaction은 ent가 소유

Caller-owned pool로 ent SQL driver를 만들고 generated client는 application package에
유지합니다.

```go
drv := entsql.OpenDB(dialect.Postgres, sqlDB)
client := ent.NewClient(ent.Driver(drv))
```

ent가 transaction을 소유할 때 `tx.Client()`는 이미 `*ent.Client`를 받는 application
code에만 전달합니다. `*ent.Client`, `*ent.Tx`, generated entity를 bluetape-go provider
API에 전달하지 않습니다.

### Bun: query를 caller-owned transaction에 연결

Bun은 caller-owned pool을 받을 수 있으며 query builder를 기존 `*sql.Tx`에 연결할 수
있습니다.

```go
bunDB := bun.NewDB(sqlDB, pgdialect.New())

tx, err := sqlDB.BeginTx(ctx, nil)
if err != nil {
    return err
}
defer tx.롤백()

_, err = bunDB.NewInsert().
    Conn(tx).
    Model(account).
    Exec(ctx)
if err != nil {
    return err
}
return tx.커밋()
```

`BeginTx`를 호출한 code만 commit/rollback합니다. Bun이 `RunInTx`로 transaction을
시작했다면 전체 unit of work를 callback 안에 두고, embedded standard transaction은
Bun이 문서화한 동작을 통해서만 사용합니다.

### sqlc: generated query를 `*sql.Tx`에 연결

Generated `Queries`는 application에 두고 transaction owner 안에서 연결합니다.

```go
func LoadGeneratedAccount(
    ctx context.Context,
    db *sql.DB,
    queries *accountsqlc.Queries,
    id int64,
) (Account, 오류) {
    var account Account
    err := sqlkit.WithTx(ctx, db, nil, func(ctx context.Context, tx *sql.Tx) 오류 {
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

Compile-checked guide example은 application-owned binder와 narrow query interface를
사용하므로 bluetape-go가 generated code를 import하지 않습니다.

### Jet: generated import를 application edge에 유지

Jet statement는 실행 시점에 표준 transaction을 받을 수 있습니다.

```go
tx, err := db.BeginTx(ctx, nil)
if err != nil {
    return err
}
defer tx.롤백()

var accounts []model.Account
if err := stmt.QueryContext(ctx, tx, &accounts); err != nil {
    return err
}
return tx.커밋()
```

Generated table/model import와 generator output은 core package 밖에 둡니다. Transaction을
시작한 code가 commit과 rollback을 소유합니다.

### Atlas: migration 소유권을 runtime provider 밖에 유지

Atlas에는 `sqlkit`에 전달할 runtime query handle이 없습니다. `migrate diff`, lint policy,
`migrate apply`는 application CI/CD 또는 operator runbook에서 관리합니다. Runtime
repository는 필요한 schema version이 이미 준비되어 있다고 가정하며 Atlas command를
호출하지 않습니다.
````

- [ ] **단계 3: 추가 모든 한국어 reference links**

추가 한국어 link labels for the same URLs:

```markdown
- [GORM existing database connection](https://gorm.io/docs/connecting_to_the_database.html)
- [ent `sql.DB` integration](https://entgo.io/docs/sql-integration/)
- [ent transaction](https://entgo.io/docs/transactions/)
- [Bun existing transaction 통합](https://bun.uptrace.dev/guide/golang-orm.html#using-bun-with-existing-code)
- [Bun transaction](https://bun.uptrace.dev/guide/transactions.html)
- [sqlc transaction](https://docs.sqlc.dev/en/latest/howto/transactions.html)
- [Jet transaction 실행](https://github.com/go-jet/jet/wiki/FAQ#how-to-execute-jet-statement-in-sql-transaction)
```

- [ ] **단계 4: 검증 locale parity 및 natural 한국어**

실행:

```bash
for file in docs/sql-generator-migration-guidance.md docs/sql-generator-migration-guidance.ko.md; do
  rg -c '^## ' "$file"
  rg -c '^### ' "$file"
  rg -c '^```' "$file"
  rg -c '^\|' "$file"
done
diff -u \
  <(rg -o 'https://[^)]+' docs/sql-generator-migration-guidance.md | sort -u) \
  <(rg -o 'https://[^)]+' docs/sql-generator-migration-guidance.ko.md | sort -u)
```

예상: the four counts match by locale 및 the URL diff is empty. Manually reject 영문 sentence skeletons, vague promotional language, 또는 inconsistent translations of pool, transaction, handle, lifecycle, 및 ownership.

- [ ] **단계 5: 검증 the 기존 README links 및 local references**

실행:

```bash
rg -n 'sql-generator-migration-guidance\.md' sqlkit/README.md
rg -n 'sql-generator-migration-guidance\.ko\.md' sqlkit/README.ko.md
test -f sqlkit/orm_boundary_example_test.go
test -f docs/sql-generator-migration-guidance.md
test -f docs/sql-generator-migration-guidance.ko.md
```

예상: each README has one locale-correct guide link 및 모든 local targets exist.

- [ ] **단계 6: Check official external links**

실행 this serial link check:

```bash
for url in \
  https://gorm.io/docs/connecting_to_the_database.html \
  https://entgo.io/docs/sql-integration/ \
  https://entgo.io/docs/transactions/ \
  https://bun.uptrace.dev/guide/golang-orm.html#using-bun-with-existing-code \
  https://bun.uptrace.dev/guide/transactions.html \
  https://docs.sqlc.dev/en/latest/howto/transactions.html \
  https://github.com/go-jet/jet/wiki/FAQ#how-to-execute-jet-statement-in-sql-transaction \
  https://github.com/go-jet/jet/wiki/Generator \
  https://atlasgo.io/versioned/diff \
  https://atlasgo.io/versioned/apply; do
  curl -fsSL --max-time 30 -o /dev/null "$url" || exit 1
done
```

예상: 모든 URLs return successfully. If an upstream blocks automated requests but opens interactively, record that exact URL as manual primary-source evidence instead of deleting it.

- [ ] **단계 7: 커밋 the bilingual guide**

```bash
git diff --check
git add docs/sql-generator-migration-guidance.md docs/sql-generator-migration-guidance.ko.md
git commit -m "docs: clarify ORM and generated SQL boundaries"
```

예상: both locale files are committed together 및 the worktree is clean.

### 작업 4: 실행 repository verification 및 integration review

**파일:**
- 검증 만; 없음 planned edits

- [ ] **단계 1: 실행 targeted fresh 테스트**

```bash
go test -count=1 ./sqlkit -run '^(ExampleSession_repositoryBoundary|ExampleWithTx_generatedQueryHandle)$'
go test -count=1 ./sqlkit
```

예상: both commands exit 0.

- [ ] **단계 2: 증명 formatting 및 module 안정성**

```bash
make fmt-check
make tidy-check
git diff --check
git status --short --branch
```

예상: 모든 commands exit 0; 없음 `go.mod`/`go.sum` diff; branch status is clean.

- [ ] **단계 3: 증명 the scoped file set**

```bash
git diff --name-only develop...HEAD
git diff --name-only develop...HEAD -- '*.go'
git diff --name-only develop...HEAD -- go.mod go.sum '.github/**' 'docs/images/**'
```

Expected complete change set:

```text
docs/sql-generator-migration-guidance.ko.md
docs/sql-generator-migration-guidance.md
docs/superpowers/plans/2026-07-14-issue-531-orm-generated-sql-boundaries-plan.md
docs/superpowers/specs/2026-07-14-issue-531-orm-generated-sql-boundaries-design.md
sqlkit/orm_boundary_example_test.go
```

Expected Go-만 output: `sqlkit/orm_boundary_example_test.go`. Expected prohibited-scope output: empty.

- [ ] **단계 4: Perform the six-perspective 및 integration review**

리뷰 `git diff develop...HEAD` directly 및 record this verdict in the PR DoD table:

| Perspective | Required verdict |
|---|---|
| Performance | N/A: documentation 및 compile-만 example add 없음 runtime path 또는 benchmark claim. |
| Stability | PASS: one transaction owner; pool sharing is 아님 described as transaction sharing; context 및 오류 f낮음 to the owner. |
| Security | PASS: 없음 credentials, real DSNs, unsafe SQL interpolation, dependency, 또는 migration execution added. |
| Operator/Ops | PASS: Atlas remains in CI/CD/runbooks 및 runtime providers assume schema readiness. |
| Developer/API | PASS: `Session`, `*sql.DB`, `*sql.Tx`, 및 generated handles have distinct ownership rules; external APIs match primary sources. |
| User/호출자 | PASS: 영문/한국어 readers receive equivalent selection rules, warnings, example, 및 links. |
| Integration | PASS 만 when P0=0 및 P1=0 및 every prior row has fresh evidence. |

Any plausible P0/P1 blocks `make ci`, push, 및 PR creation until repaired 및 revalidated.

- [ ] **단계 5: 실행 the authoritative local CI gate**

```bash
make ci
```

예상: `tidy-check`, `fmt-check`, `vet`, `lint`, serial uncached 테스트, 및 serial uncached race 테스트 모든 pass. If lint reports a deleted-worktree cache path, run `golangci-lint cache clean` 및 rerun `make ci` from the beginning; 만 the fresh observed exit code is valid.

### 작업 5: 생성 the PR 및 stop at merge approval

**파일:**
- No repository file changes expected

- [ ] **단계 1: Recheck issue metadata 및 branch state**

```bash
gh issue view 531 --json state,assignees,labels,milestone,url
git status --short --branch
git log --oneline develop..HEAD
```

예상: issue is OPEN, assignee includes `debop`, milestone is `0.19.0`, labels include `type: task`, `area: docs`, `area: database`, 및 `priority: p2`; branch is clean 및 contains the design, plan, example, 및 bilingual guide commits.

- [ ] **단계 2: Push the branch**

```bash
git push -u origin docs/issue-531-orm-boundary-examples
```

예상: upstream is set 및 local HEAD equals the remote branch SHA.

- [ ] **단계 3: 생성 the PR 함께 the final DoD heading**

실행 `mkdir -p .tmp`, then create `.tmp/issue-531-pr-body.md` 함께 `apply_patch`
using this exact body:

```markdown
## Summary

- document the supported `database/sql`, `sqlkit.Session`, transaction, and generated-query boundaries
- add GORM, ent, Bun, sqlc, Jet, and Atlas policy and ownership examples without runtime dependencies
- compile-check the shared session and application-owned generated-handle patterns
- keep the English and Korean guides source-equivalent

Fixes #531

## 검증

- `go test -count=1 ./sqlkit -run '^(ExampleSession_repositoryBoundary|ExampleWithTx_generatedQueryHandle)$'`
- `go test -count=1 ./sqlkit`
- locale heading, table, code-fence, URL, README-link, and official-link checks
- `make fmt-check`
- `make tidy-check`
- `git diff --check`
- `make ci`

## DoD Status

| Check | Status | Evidence |
|---|---|---|
| Scope and dependencies | PASS | Production code, module metadata, schema, workflow, and diagram assets are unchanged; no optional dependency was added. |
| Compile examples | PASS | `sqlkit.Session` and generated-handle examples compile against existing public contracts. |
| Documentation parity | PASS | English/Korean headings, tables, code blocks, warnings, and URLs match. |
| Primary sources | PASS | GORM, ent, Bun, sqlc, Jet, and Atlas identifiers and links were refreshed from official/upstream documentation. |
| Performance | N/A | Documentation and compile-only examples add no runtime path or benchmark claim. |
| Stability | PASS | Pool sharing and transaction sharing are distinct; one owner commits or rolls back. |
| Security | PASS | No credentials, runtime dependency, unsafe interpolation, or migration execution was added. |
| Operator/Ops | PASS | Atlas remains an external application CI/CD or runbook boundary. |
| Developer/API | PASS | Standard handles and caller-owned generated interfaces remain the only bluetape-go boundary. |
| User/caller | PASS | Both locales provide equivalent selection rules and runnable-shape examples. |
| Integration | PASS | Local P0=0, P1=0 and `make ci` completed successfully. |
```

Then run:

```bash
gh pr create \
  --base develop \
  --head docs/issue-531-orm-boundary-examples \
  --title "docs: clarify ORM and generated SQL boundaries" \
  --body-file .tmp/issue-531-pr-body.md
```

예상: one OPEN PR URL is returned.

- [ ] **단계 4: Apply 및 verify exact live PR metadata**

```bash
pr=$(gh pr view --json number --jq .number)
gh pr edit "$pr" \
  --add-assignee debop \
  --add-label 'type: task' \
  --add-label 'area: docs' \
  --add-label 'area: database' \
  --add-label 'priority: p2' \
  --milestone '0.19.0'
gh pr view "$pr" --json number,title,state,baseRefName,headRefName,body,assignees,labels,milestone,url
```

예상: base is `develop`, head is `docs/issue-531-orm-boundary-examples`, title is exact, assignee is `debop`, milestone is `0.19.0`, 모든 four labels are present, 및 the body's final `##` heading is `## DoD Status`. Repair any mismatch 전에 waiting for CI.

- [ ] **단계 5: Wait for CI, refresh reviews, 및 stop**

Poll checks in bounded intervals 및 report progress every 2-3 minutes:

```bash
gh pr checks --watch --interval 20
pr=$(gh pr view --json number --jq .number)
gh pr view "$pr" --json reviewDecision,reviews,comments,latestReviews,mergeStateStatus,statusCheckRollup
```

예상: 모든 required checks are green, 없음 unresolved P0/P1 review feedback exists, 및 merge state is ready. 다음을 하지 않는다: merge. Report the PR URL, HEAD SHA, CI results, review state, 및 exact merge hold, then request explicit merge approval.
