# Lessons Learned - SQLKit Builder Boundary (2026-06-26)

Related issue: #318
Affected module: `sqlkit`

## L1: Keep the first builder inspectable before making it expressive

### Problem

The relational SQL milestone needs a useful repository prototype, but a broad
query DSL could quickly become an ORM-shaped surface. Joins, dialect switching,
metadata, and code generation are all tempting additions once builders exist.

### Lesson

The first slice should prove explicit SQL and args before expanding expression
coverage. `Statement` keeps SQL visible, tests assert exact generated strings,
and `Where` remains a caller-owned escape hatch for subqueries while identifiers
and bind values are still guarded.

### Evidence

- Builder tests assert exact SQL and args for SELECT/INSERT/UPDATE/DELETE.
- Repository prototype proves CRUD, rollback, and a relational `exists` query
  against PostgreSQL Testcontainers.
- README documents the PostgreSQL `$n` placeholder boundary and non-goals.

### Future Guidance

Only add first-class JOIN, dialect, or generator surfaces after a follow-up
issue proves the need with repository examples and review evidence. Until then,
prefer explicit raw SQL fragments over hidden ORM behavior.
