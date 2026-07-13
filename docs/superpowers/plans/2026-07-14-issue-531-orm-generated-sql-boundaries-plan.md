# Issue #531 ORM and Generated SQL Boundaries Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Clarify how applications combine bluetape-go with GORM, ent, Bun, sqlc, Jet, and Atlas while keeping ORM state and optional dependencies outside core packages.

**Architecture:** Keep production packages unchanged. Add dependency-free external-package examples that compile the shared `sqlkit.Session` and transaction-bound generated-handle patterns, then expand the existing English/Korean SQL generator guide with a policy matrix and upstream-verified integration snippets. Treat pool sharing, transaction sharing, and migration ownership as separate contracts.

**Tech Stack:** Go 1.26, `database/sql`, existing `sqlkit` interfaces, Go examples, bilingual Markdown, official GORM/ent/Bun/sqlc/Jet/Atlas documentation, GitHub CLI.

---

## Execution constraints

- Work only in `/Users/debop/work/bluetape4k/bluetape-go/.worktrees/docs-issue-531-orm-boundary-examples` on `docs/issue-531-orm-boundary-examples`.
- Design authority: `docs/superpowers/specs/2026-07-14-issue-531-orm-generated-sql-boundaries-design.md` at commit `ff23d6d`.
- Keep production behavior unchanged. No production `.go` file, `go.mod`, `go.sum`, schema, migration, workflow, diagram, or generated source may change.
- Do not add GORM, ent, Bun, sqlc, Jet, Atlas, goqu, or Bob dependencies.
- Use `apply_patch` for file edits. Use `gofmt` only for the new Go example file.
- Public documentation, commits, and PR metadata are English. Keep the Korean guide source-equivalent and natural rather than literal.
- Verify external API names against official/upstream sources immediately before editing. Do not document internal fields or unsupported transaction binding.
- Run heavyweight checks serially. Stop before merge; CI green only authorizes a merge-approval request.
- CodeGraph is not exposed in this session. Direct source inspection is the recorded fallback for `sqlkit.Session` and `sqlkit.WithTx`.

## File map

| Path | Responsibility |
|---|---|
| `sqlkit/orm_boundary_example_test.go` | Compile-check standard session and application-owned generated-handle boundaries without optional dependencies |
| `docs/sql-generator-migration-guidance.md` | English policy matrix, ownership rules, and upstream-backed tool examples |
| `docs/sql-generator-migration-guidance.ko.md` | Natural Korean source-equivalent guide |
| `docs/superpowers/specs/2026-07-14-issue-531-orm-generated-sql-boundaries-design.md` | Approved design authority; no implementation edits expected |
| `docs/superpowers/plans/2026-07-14-issue-531-orm-generated-sql-boundaries-plan.md` | This execution contract |

`sqlkit/README.md` and `sqlkit/README.ko.md` already link their locale-specific guide under the selection section. Verify those links, but do not edit the README pair unless the link is missing or incorrect; a scope expansion requires a revised plan.

### Task 1: Add dependency-free compile examples

**Files:**
- Create: `sqlkit/orm_boundary_example_test.go`

This maintenance task adds no production behavior, so manufacturing a RED production test is N/A. The gate is that the new public example compiles against the existing `sqlkit` contracts without adding a dependency.

- [ ] **Step 1: Create the standard session and generated-handle examples**

Create `sqlkit/orm_boundary_example_test.go` with this exact content:

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

- [ ] **Step 2: Format and compile the examples**

Run:

```bash
gofmt -w sqlkit/orm_boundary_example_test.go
go test -count=1 ./sqlkit -run '^(ExampleSession_repositoryBoundary|ExampleWithTx_generatedQueryHandle)$'
```

Expected: `ok github.com/bluetape4k/bluetape-go/sqlkit`; `go.mod` and `go.sum` remain unchanged.

- [ ] **Step 3: Review the example boundary**

Run:

```bash
rg -n 'gorm|entgo|uptrace|go-jet|atlasgo|sqlc' sqlkit/orm_boundary_example_test.go
git diff --check
git diff -- sqlkit/orm_boundary_example_test.go
```

Expected: the dependency search returns no matches; the diff contains only external-package examples, standard-library imports, and the existing `sqlkit` import; diff check exits 0.

- [ ] **Step 4: Commit the compile examples**

```bash
git add sqlkit/orm_boundary_example_test.go
git commit -m "docs: add SQL boundary compile examples"
```

Expected: one new `_test.go` file is committed and the worktree is clean.

### Task 2: Expand the English boundary guide

**Files:**
- Modify: `docs/sql-generator-migration-guidance.md`

- [ ] **Step 1: Refresh the official source ledger**

Open and verify these exact primary sources:

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

- GORM: `mysql.New(mysql.Config{Conn: sqlDB})` initializes GORM from caller-owned `*sql.DB`.
- ent: `entsql.OpenDB(dialect.Postgres, sqlDB)` and `tx.Client()` remain documented.
- Bun: `bun.NewDB(sqldb, pgdialect.New())` and `.Conn(tx)` remain documented; `bun.Tx` embeds `*sql.Tx`.
- sqlc: generated `DBTX`, `Queries.WithTx(*sql.Tx)`, and returned transaction-bound `*Queries` remain documented.
- Jet: `stmt.QueryContext(ctx, tx, &dest)` and `stmt.ExecContext(ctx, tx)` remain documented.
- Atlas: `migrate diff` and `migrate apply` remain external CLI workflows.

If an identifier has changed, update both locale snippets to the currently documented public form before continuing. Never substitute an internal field.

- [ ] **Step 2: Insert the policy matrix after `## Selection Matrix`**

After the current selection table and before `## Runtime-First Boundary`, insert:

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

- [ ] **Step 3: Replace the runtime-first section with explicit handle ownership**

Replace the existing `## Runtime-First Boundary` section through the line before `## Isolated sqlc Example` with:

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

The compile-checked guide example uses an application-owned binder and narrow
query interface so bluetape-go does not import generated code.

### Jet: keep generated imports at the application edge

Jet statements accept the standard transaction at execution time:

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

Keep generated table/model imports and generator output outside core packages.
The code that starts the transaction owns commit and rollback.

### Atlas: keep migration ownership outside runtime providers

Atlas has no runtime query handle to pass into `sqlkit`. Keep `migrate diff`,
lint policy, and `migrate apply` in application CI/CD or operator runbooks.
Runtime repositories should assume the required schema version already exists
and should not invoke Atlas commands.
````

- [ ] **Step 4: Extend the reference list**

Add these official sources to `## References`, preserving the existing entries:

```markdown
- [GORM existing database connection](https://gorm.io/docs/connecting_to_the_database.html)
- [ent `sql.DB` integration](https://entgo.io/docs/sql-integration/)
- [ent transactions](https://entgo.io/docs/transactions/)
- [Bun integration with existing transactions](https://bun.uptrace.dev/guide/golang-orm.html#using-bun-with-existing-code)
- [Bun transactions](https://bun.uptrace.dev/guide/transactions.html)
- [sqlc transactions](https://docs.sqlc.dev/en/latest/howto/transactions.html)
- [Jet transaction execution](https://github.com/go-jet/jet/wiki/FAQ#how-to-execute-jet-statement-in-sql-transaction)
```

Expected: every technical identifier in the new English section is backed by one of these primary sources or an existing Atlas/Jet source.

### Task 3: Add the source-equivalent Korean guide

**Files:**
- Modify: `docs/sql-generator-migration-guidance.ko.md`
- Modify: `docs/sql-generator-migration-guidance.md` only for parity corrections found during this task

- [ ] **Step 1: Insert the Korean policy matrix after `## 선택 Matrix`**

After the current selection table and before `## Runtime-First 경계`, insert:

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

- [ ] **Step 2: Replace the Korean runtime-first section with explicit ownership**

Replace the existing `## Runtime-First 경계` section through the line before `## 격리된 sqlc 예제` with:

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

Compile-checked guide example은 application-owned binder와 narrow query interface를
사용하므로 bluetape-go가 generated code를 import하지 않습니다.

### Jet: generated import를 application edge에 유지

Jet statement는 실행 시점에 표준 transaction을 받을 수 있습니다.

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

Generated table/model import와 generator output은 core package 밖에 둡니다. Transaction을
시작한 code가 commit과 rollback을 소유합니다.

### Atlas: migration 소유권을 runtime provider 밖에 유지

Atlas에는 `sqlkit`에 전달할 runtime query handle이 없습니다. `migrate diff`, lint policy,
`migrate apply`는 application CI/CD 또는 operator runbook에서 관리합니다. Runtime
repository는 필요한 schema version이 이미 준비되어 있다고 가정하며 Atlas command를
호출하지 않습니다.
````

- [ ] **Step 3: Add all Korean reference links**

Add Korean link labels for the same URLs:

```markdown
- [GORM existing database connection](https://gorm.io/docs/connecting_to_the_database.html)
- [ent `sql.DB` integration](https://entgo.io/docs/sql-integration/)
- [ent transaction](https://entgo.io/docs/transactions/)
- [Bun existing transaction 통합](https://bun.uptrace.dev/guide/golang-orm.html#using-bun-with-existing-code)
- [Bun transaction](https://bun.uptrace.dev/guide/transactions.html)
- [sqlc transaction](https://docs.sqlc.dev/en/latest/howto/transactions.html)
- [Jet transaction 실행](https://github.com/go-jet/jet/wiki/FAQ#how-to-execute-jet-statement-in-sql-transaction)
```

- [ ] **Step 4: Verify locale parity and natural Korean**

Run:

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

Expected: the four counts match by locale and the URL diff is empty. Manually reject English sentence skeletons, vague promotional language, or inconsistent translations of pool, transaction, handle, lifecycle, and ownership.

- [ ] **Step 5: Verify the existing README links and local references**

Run:

```bash
rg -n 'sql-generator-migration-guidance\.md' sqlkit/README.md
rg -n 'sql-generator-migration-guidance\.ko\.md' sqlkit/README.ko.md
test -f sqlkit/orm_boundary_example_test.go
test -f docs/sql-generator-migration-guidance.md
test -f docs/sql-generator-migration-guidance.ko.md
```

Expected: each README has one locale-correct guide link and all local targets exist.

- [ ] **Step 6: Check official external links**

Run this serial link check:

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

Expected: all URLs return successfully. If an upstream blocks automated requests but opens interactively, record that exact URL as manual primary-source evidence instead of deleting it.

- [ ] **Step 7: Commit the bilingual guide**

```bash
git diff --check
git add docs/sql-generator-migration-guidance.md docs/sql-generator-migration-guidance.ko.md
git commit -m "docs: clarify ORM and generated SQL boundaries"
```

Expected: both locale files are committed together and the worktree is clean.

### Task 4: Run repository verification and integration review

**Files:**
- Verify only; no planned edits

- [ ] **Step 1: Run targeted fresh tests**

```bash
go test -count=1 ./sqlkit -run '^(ExampleSession_repositoryBoundary|ExampleWithTx_generatedQueryHandle)$'
go test -count=1 ./sqlkit
```

Expected: both commands exit 0.

- [ ] **Step 2: Prove formatting and module stability**

```bash
make fmt-check
make tidy-check
git diff --check
git status --short --branch
```

Expected: all commands exit 0; no `go.mod`/`go.sum` diff; branch status is clean.

- [ ] **Step 3: Prove the scoped file set**

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

Expected Go-only output: `sqlkit/orm_boundary_example_test.go`. Expected prohibited-scope output: empty.

- [ ] **Step 4: Perform the six-perspective and integration review**

Review `git diff develop...HEAD` directly and record this verdict in the PR DoD table:

| Perspective | Required verdict |
|---|---|
| Performance | N/A: documentation and compile-only examples add no runtime path or benchmark claim. |
| Stability | PASS: one transaction owner; pool sharing is not described as transaction sharing; context and errors flow to the owner. |
| Security | PASS: no credentials, real DSNs, unsafe SQL interpolation, dependency, or migration execution added. |
| Operator/Ops | PASS: Atlas remains in CI/CD/runbooks and runtime providers assume schema readiness. |
| Developer/API | PASS: `Session`, `*sql.DB`, `*sql.Tx`, and generated handles have distinct ownership rules; external APIs match primary sources. |
| User/caller | PASS: English/Korean readers receive equivalent selection rules, warnings, examples, and links. |
| Integration | PASS only when P0=0 and P1=0 and every prior row has fresh evidence. |

Any plausible P0/P1 blocks `make ci`, push, and PR creation until repaired and revalidated.

- [ ] **Step 5: Run the authoritative local CI gate**

```bash
make ci
```

Expected: `tidy-check`, `fmt-check`, `vet`, `lint`, serial uncached tests, and serial uncached race tests all pass. If lint reports a deleted-worktree cache path, run `golangci-lint cache clean` and rerun `make ci` from the beginning; only the fresh observed exit code is valid.

### Task 5: Create the PR and stop at merge approval

**Files:**
- No repository file changes expected

- [ ] **Step 1: Recheck issue metadata and branch state**

```bash
gh issue view 531 --json state,assignees,labels,milestone,url
git status --short --branch
git log --oneline develop..HEAD
```

Expected: issue is OPEN, assignee includes `debop`, milestone is `0.19.0`, labels include `type: task`, `area: docs`, `area: database`, and `priority: p2`; branch is clean and contains the design, plan, example, and bilingual guide commits.

- [ ] **Step 2: Push the branch**

```bash
git push -u origin docs/issue-531-orm-boundary-examples
```

Expected: upstream is set and local HEAD equals the remote branch SHA.

- [ ] **Step 3: Create the PR with the final DoD heading**

Run `mkdir -p .tmp`, then create `.tmp/issue-531-pr-body.md` with `apply_patch`
using this exact body:

```markdown
## Summary

- document the supported `database/sql`, `sqlkit.Session`, transaction, and generated-query boundaries
- add GORM, ent, Bun, sqlc, Jet, and Atlas policy and ownership examples without runtime dependencies
- compile-check the shared session and application-owned generated-handle patterns
- keep the English and Korean guides source-equivalent

Fixes #531

## Validation

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

Expected: one OPEN PR URL is returned.

- [ ] **Step 4: Apply and verify exact live PR metadata**

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

Expected: base is `develop`, head is `docs/issue-531-orm-boundary-examples`, title is exact, assignee is `debop`, milestone is `0.19.0`, all four labels are present, and the body's final `##` heading is `## DoD Status`. Repair any mismatch before waiting for CI.

- [ ] **Step 5: Wait for CI, refresh reviews, and stop**

Poll checks in bounded intervals and report progress every 2-3 minutes:

```bash
gh pr checks --watch --interval 20
pr=$(gh pr view --json number --jq .number)
gh pr view "$pr" --json reviewDecision,reviews,comments,latestReviews,mergeStateStatus,statusCheckRollup
```

Expected: all required checks are green, no unresolved P0/P1 review feedback exists, and merge state is ready. Do not merge. Report the PR URL, HEAD SHA, CI results, review state, and exact merge hold, then request explicit merge approval.
