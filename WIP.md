# WIP

Snapshot: 2026-06-26 KST
Scope: `0.7.0` release preparation for relational SQL DSL and repository helpers.

## Current Target Release

`v0.7.0` - runtime-first relational SQL support for Go callers.

The milestone introduces `sqlkit` as a small `database/sql` helper package:
context-aware transaction ownership, explicit row mapping/cardinality helpers,
PostgreSQL-first inspectable SQL builders, Testcontainers-backed repository
examples, and optional sqlc/Jet/Atlas guidance without adding those tools as
core runtime dependencies.

## Current State

- `0.1.0`, `0.1.1`, `0.2.0`, `0.3.0`, `0.4.0`, `0.5.0`, `v0.5.1`,
  `v0.6.0`, and `v0.6.1` through `v0.6.8` are tagged and released.
- Milestone `0.7.0` has no open issues after #317, #318, #319, and epic #101
  were closed.
- Issue #100 recorded the runtime-first SQL direction and deferred broad ORM,
  hidden migration, and mandatory generation behavior.
- Issue #317 added the `sqlkit` transaction and row mapping foundation.
- Issue #318 added inspectable PostgreSQL-first builders and repository
  examples.
- Issue #319 documented optional sqlc/Jet generator workflows and Atlas as an
  external migration boundary.

## Release Checklist

1. `0.7.0` issue-linked PRs are merged on `develop`.
2. Milestone `0.7.0` is closed with `open_issues=0`.
3. `CHANGELOG.md` has a `v0.7.0` section dated `2026-06-26`.
4. `git diff --check`, local `make ci`, GitHub CI on `develop`, and Nightly
   Testcontainers evidence are release gates.
5. Promote `develop` to `main`, tag `v0.7.0` on `main`, and publish the GitHub
   Release.
