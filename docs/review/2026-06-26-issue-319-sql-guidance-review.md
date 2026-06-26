# Issue #319 SQL generator guidance review

Date: 2026-06-26
Branch: `docs/issue-319-sql-guidance`
Baseline: `2c6a7a2 feat: add inspectable sqlkit builders (#326)`
Scope: documentation-only SQL generator and migration guidance.

## Review

| Tier | Verdict | Evidence |
|---|---|---|
| Performance | PASS | Documentation-only change; no Go code or runtime dependency changed. |
| Stability | PASS | Examples write only under `.tmp/sql-guidance/*` and keep generated code out of normal package paths. |
| Security | PASS | Atlas example requires caller-supplied disposable dev database URL; no credentials are committed. |
| Operator/Ops | PASS | Atlas is documented as an external CI/CD or runbook boundary, not a hidden repository helper. |
| Developer/API | PASS | Selection matrix covers direct `database/sql`, `sqlkit`, sqlc, Jet, ent, Bun, GORM, goqu, and Atlas. |
| User/Caller | PASS | README and `sqlkit` README link to the new English/Korean guidance pages. |
| Integration | PASS | Guidance cross-links #100 research and #101 epic, and keeps core `sqlkit` dependency-free. |

P0=0 P1=0

## Verification

- `git diff --check`
- targeted `rg` for guidance links, `sqlc`, `Jet`, `Atlas`, and #100/#101 anchors
- Web evidence preserved in `bluetape4k-wiki` commit `d5d6828`

## Residual Risk

The sqlc, Jet, and Atlas CLIs are intentionally not executed locally because the
examples require those external tools and, for Jet/Atlas, a caller-owned
database boundary. The commands are copied from current official documentation
patterns and isolated under `.tmp/sql-guidance`.
