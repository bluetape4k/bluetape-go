# Issue #318 SQLKit Builder Design

## Problem

Issue #318 needs the first runtime-first SQL builder slice for `sqlkit` after
the transaction and row mapping foundation from #317. The package currently
keeps SQL caller-owned and visible through `WithTx`, `QueryAll`,
`QueryOptional`, and `QueryOne`; it has no inspectable builder layer.

## Constraints And Evidence

- Current package: `sqlkit/session.go`, `sqlkit/rows.go`,
  `sqlkit/transaction.go`, and `sqlkit/integration_test.go`.
- Existing PostgreSQL fixture:
  `testcontainers/postgres.Start(ctx, t)` returns a pgx-compatible connection
  string and is already used by `sqlkit/integration_test.go`.
- Issue #318 requires SELECT, INSERT, UPDATE, DELETE builders that produce
  explicit SQL plus args, a PostgreSQL-first stance, and a small repository
  example proving CRUD, rollback, and one relational query.
- Non-goals remain: no ORM, no hidden eager loading, no migrations, no broad
  dialect abstraction, no generated-code requirement, and no JSON/encrypted
  column support in this first slice.

## Approach

Add a minimal builder surface inside `sqlkit`:

- `Statement` stores `SQL string` and copied `Args []any`.
- `SelectFrom`, `InsertInto`, `Update`, and `DeleteFrom` return focused builder
  types.
- Builders emit PostgreSQL placeholders (`$1`, `$2`, ...) and keep every
  generated statement directly inspectable in tests.
- Builders validate and double-quote table/column identifiers. SQL fragments
  passed to `Where` are explicit caller-owned fragments; only their `?`
  placeholders are rewritten to PostgreSQL placeholders.
- UPDATE and DELETE require a `Where` clause by default to prevent accidental
  whole-table writes in this first API.

Rejected alternatives:

- A broad dialect abstraction: rejected because #318 prefers PostgreSQL first
  and #319 owns broader generator/migration guidance.
- A type-safe entity/relationship DSL: rejected because it would drift toward
  an ORM and exceed the runtime-first scope.
- Raw string concatenation with no identifier validation: rejected because table
  and column names are caller-provided strings and would invite SQL injection
  misuse.

## API Shape

```go
stmt, err := sqlkit.SelectFrom("accounts").
    Columns("id", "name").
    Where("id = ?", id).
    OrderBy("id").
    Limit(1).
    Build()
```

Expected SQL:

```sql
select "id", "name" from "accounts" where id = $1 order by "id" limit 1
```

`Statement` remains simple enough for direct `database/sql` use:

```go
rows, err := db.QueryContext(ctx, stmt.SQL, stmt.Args...)
```

## Risks And Mitigations

| Risk | Mitigation |
|---|---|
| Identifier injection through table/column strings | Validate dotted identifiers and quote each segment. Reject invalid input with `ErrInvalidArgument`. |
| Placeholder/argument drift in caller-owned `Where` fragments | Count `?` placeholders and require the count to match args before rewriting. |
| Accidental full-table mutation | Reject UPDATE/DELETE without `Where` in the first builder API. |
| Dialect expectations exceed PostgreSQL-first scope | README documents `$n` placeholder output and directs unsupported dialect/features to raw SQL or larger query builders. |
| Hidden SQL undermines runtime-first stance | Tests assert exact SQL and args; repository example builds statements and executes via context-aware `database/sql` boundaries. |

## Acceptance Mapping

- Builder tests assert exact SQL strings and args for SELECT, INSERT, UPDATE,
  and DELETE.
- Builder tests cover invalid identifiers, placeholder mismatches, missing
  mutation guards, and args copy behavior.
- Repository example uses real PostgreSQL Testcontainers and proves CRUD,
  rollback, and an account-to-event relational query.
- Execution boundaries use `context.Context` through `database/sql` and existing
  `sqlkit` query/transaction helpers.
- `sqlkit/README.md`, `sqlkit/README.ko.md`, root `README.md`, and root
  `README.ko.md` document the PostgreSQL placeholder boundary and runtime-first
  stance.
- Package race gate passes with `go test -race -count=1 ./sqlkit`.

## DoD

- Spec and plan exist in the feature worktree.
- TDD red/green evidence is captured for builder tests.
- Targeted and race package tests pass.
- Repo formatting, vet/lint/tidy, and CI gates pass or blockers are recorded.
- Step 6-R review artifact reports `P0=0 P1=0`.
- Lesson is committed before PR creation.
