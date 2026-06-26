# Issue 317 sqlkit Step 6-R Review

## Scope

- Baseline: `a21cc85` (`develop`)
- Changed package: `sqlkit`
- Supporting docs: root README/README.ko.md, `sqlkit` README/README.ko.md,
  #100 research note milestone wording

## 7-Tier Findings

| Tier | Verdict | Evidence |
|---|---|---|
| Performance | PASS | `QueryOptional`/`QueryOne` now read at most two rows before returning `ErrTooManyRows`; this avoids unbounded cardinality checks. |
| Stability | PASS | `WithTx` commits only on nil callback error, rolls back on callback error, and preserves begin/commit/rollback errors. `QueryAll` closes rows through defer on success and failure. |
| Security | PASS | No SQL construction, interpolation, credentials, auth, or filesystem behavior added. SQL and args remain caller-owned. |
| Operator/Ops | PASS | PostgreSQL Testcontainers integration proves commit and rollback with a real driver path. No pool lifecycle ownership or background goroutines added. |
| Developer/API | PASS | API is small and Go-native: `WithTx`, `QueryAll`, `QueryOptional`, `QueryOne`, `ScanOne`, plus narrow `database/sql` interfaces. No ORM or code generation surface. |
| User/Caller | PASS | README and README.ko.md explain selection guide, direct SQL ownership, non-goals, and when to use direct `database/sql`, `pgx`, sqlc, Jet, ent, Bun, GORM, or goqu. |
| Integration | PASS | Targeted tests, race test, vet, lint, fmt, and full `go test ./...` are green. |

## Blocker Review

- P0: 0
- P1: 0

## Fixed During Review

- P1 candidate: `QueryOne`/`QueryOptional` originally delegated through
  `QueryAll`, so cardinality checks could read an unbounded result set. Fixed
  by adding a bounded internal row mapper that stops after two mapped rows.

## Verification

- `git diff --check`
- `go test -count=1 ./sqlkit`
- `go test -race -count=1 ./sqlkit`
- `make fmt-check`
- `make vet`
- `make lint`
- `go test -count=1 ./...`

## Residual Risk

- `make tidy-check` should be rerun after committing because it checks
  `go.mod`/`go.sum` drift against the working tree; this branch intentionally
  records the new indirect `github.com/jackc/puddle/v2` tidy entry required by
  `pgx/v5/stdlib`.
