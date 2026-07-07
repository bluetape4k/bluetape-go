# Benchmark Artifact Retention

Issue #401 makes benchmark output traceable before recommendation work starts.

## Lessons

- Keep raw output, command, and environment metadata together. A benchmark row
  without OS, CPU, Go version, git SHA, and dirty-tree state is not enough for a
  cross-repo recommendation.
- Store local snapshots as evidence, not rankings. Reports can say what a file
  measured; production defaults require a later decision with caller constraints
  and security boundaries.
- Use stable issue-specific output directories. Downstream reports should cite
  files instead of relying on pasted benchmark excerpts.

## Evidence

- `docs/research/2026-07-07-issue-401-benchmark-artifact-retention.md`
- `docs/research/benchmark-artifact-template.md`
- `docs/research/outputs/issue-401/`
