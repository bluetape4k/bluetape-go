# Issue 100 SQL DSL And Repository Research Scope

Issue #100 decides the Go relational data-access direction before the 0.7.0
implementation epic #101. The outcome is runtime-first: implement small
`database/sql` transaction, row mapping, and inspectable SQL builder helpers,
while keeping code generation optional and external.

## Source Inventory

Source repositories:

- `/Users/debop/work/bluetape4k/bluetape4k-exposed`
- `/Users/debop/work/bluetape4k/bluetape4k-projects/data`

Relevant source capabilities:

- `bluetape4k-exposed` provides repository patterns, transaction DSLs,
  JDBC/R2DBC repositories, CTE helpers, cache decorators, JSON columns, Tink
  encrypted columns, measured columns, dialect modules, Spring Boot wiring, and
  real database examples.
- `data/jdbc` provides connection/statement helpers, typed result access,
  transaction support, batch processing, and row mapping helpers.
- `data/r2dbc` provides coroutine/Flow data access, pool tuning, transaction
  support, query helpers, and Spring auto-configuration.
- `data/mongodb` and `data/cassandra` are comparison evidence only; they are not
  part of the relational 0.7.0 scope.

Current bluetape-go evidence:

- Existing Testcontainers fixtures cover PostgreSQL, MySQL, and MariaDB.
- The 0.7.0 research note already rejects a Kotlin Exposed clone, full ORM
  layer, and mandatory generated code by default.
- Existing Go packages use explicit `context.Context`, visible errors, and
  caller-owned resource lifecycles.

## External Go Evidence

- `database/sql` remains the standard-library execution boundary and owns
  context-aware query, exec, transaction, row, and rows APIs.
- sqlc generates type-safe Go code from hand-written SQL and is best when a
  project accepts generated source as part of the workflow.
- Jet provides type-safe SQL builder/model code from database schema and is
  strong for complex dynamic SQL, but its builder files are generated.
- ent is a code-generation entity framework with schema-as-code, graph-style
  edges, and migrations; it is too broad for bluetape-go's first SQL package.
- Bob supports query/model/ORM generation for PostgreSQL, MySQL/MariaDB, and
  SQLite, but that makes it a later comparison candidate rather than the core
  default.
- goqu is an expressive runtime SQL builder/executor candidate, but adopting it
  would add a dependency before bluetape-go proves its own minimal SQL contract.
- Atlas is a credible external migration tool, but migration execution should
  stay outside the first library package.

Sources:

- https://pkg.go.dev/database/sql
- https://docs.sqlc.dev/
- https://github.com/go-jet/jet
- https://entgo.io/docs/getting-started/
- https://bob.stephenafamo.com/docs/
- https://doug-martin.github.io/goqu/
- https://atlasgo.io/

## Candidate Ranking

| Area | Go fit | Risk | Decision |
|---|---:|---:|---|
| `database/sql` transaction helpers | High | Medium | Implement in #317. |
| Row mapping/cardinality helpers | High | Medium | Implement in #317. |
| Minimal inspectable SQL builder | Medium/high | Medium/high | Implement after foundation in #318. |
| Repository example conventions | High | Medium | Implement with #318. |
| PostgreSQL Testcontainers anchor | High | Medium | Use first; MySQL/MariaDB later. |
| sqlc optional generated-code workflow | Medium/high | Medium | Document as optional in #319. |
| Jet optional generated builder workflow | Medium/high | Medium | Document as optional in #319. |
| Atlas migration workflow | Medium | Medium | Recommend externally in #319; do not wrap. |
| ent | Medium | High | Defer; full entity framework is too broad. |
| Bob | Medium | High | Defer; generator/ORM surface is too broad for first slice. |
| Bun/GORM | Medium | High | Defer/reject for core; too ORM-shaped for this milestone. |
| goqu | Medium | Medium | Defer dependency adoption until minimal builder proves insufficient. |
| JSON/encrypted/measured columns | Medium | High | Defer to later child issues after base API lands. |
| Cache-backed repositories | Medium | High | Defer until repository contracts and invalidation semantics are proven. |
| Non-relational MongoDB/Cassandra | Low | High | Out of #101 scope. |

## Implement

Create three 0.7.0 child issues:

- #317: runtime SQL transaction and row mapping foundation.
- #318: inspectable SQL builder and repository prototype.
- #319: optional sqlc/Jet generator and Atlas migration guidance.

## Direction For #101

- Start with `database/sql` because it keeps SQL visible, driver behavior
  caller-owned, and dependencies small.
- Make PostgreSQL the first integration anchor because existing Testcontainers
  fixtures cover it and PostgreSQL exposes enough SQL features to reveal weak
  abstractions early.
- Keep MySQL/MariaDB compatibility as a follow-up once the API shape is stable.
- Treat `pgx` as a compatible driver/runtime candidate, not a required public
  dependency in the first package.
- Keep code generation optional. sqlc and Jet are good workflow examples, but
  mandatory generated code would conflict with the runtime-first user story.
- Keep migrations external. Atlas can be recommended in docs, but bluetape-go
  should not hide migration execution.

## Defer

- ent, Bob, Bun, GORM, and goqu adoption until concrete consumers prove the
  minimal runtime APIs are insufficient.
- JSON column helpers until serialization/envelope behavior is clear.
- Encrypted columns until #315 lands and a SQL column consumer owns key
  material and associated data rules.
- Measured columns until SQL package users need `measure`/`money` persistence
  helpers.
- Cache-backed repositories until transaction boundaries, stale-read behavior,
  invalidation, and retry semantics are designed.
- CTE, upsert, batch, schema metadata, and dialect modules until the base
  builder/repository prototype lands.

## Issue Updates Required

- #100 should record this outcome and close through the research PR.
- #101 should list #317, #318, and #319 as the first implementation children.
- #7 should record that SQL research completed and moved implementation to
  #101 / 0.7.0.

## Validation Plan

- Documentation-only PR: `git diff --check` and targeted `rg`.
- Verify #100, #101, #7, #317, #318, and #319 issue bodies contain the #100
  research outcome or research note path.
- Preserve external evidence in `bluetape4k-wiki` and validate with
  `gno update`, `gno embed --collection bluetape4k-wiki`, and representative
  `gno search`.
- No Go tests are required for this PR because no Go code changes.

## Follow-up Recommendation

Work #317 first in 0.7.0. Do not start the SQL builder until transaction and row
mapping contracts are tested against a real PostgreSQL container.
