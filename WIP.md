# WIP

Snapshot: 2026-06-30 KST
Scope: `0.10.0` graph package milestone.

## Current Target Release

`v0.10.0` - graph model values, graph I/O helpers, backend evaluation, and
domain examples.

Issue #48 introduced the first `graph` package as model-only values: stable
element IDs, labels, vertices, directed edges, paths, shallow properties,
redacted validation errors, and validated JSON. Issue #49 adds `graph/graphio`
for bounded NDJSON and paired CSV import/export helpers. Repository, session,
schema, query, transaction, backend, and algorithm contracts remain out of
scope until #50 and #51 prove shared behavior through backend evaluation and
examples.

## Current State

- `0.1.0`, `0.1.1`, `0.2.0`, `0.3.0`, `0.4.0`, `0.5.0`, `v0.5.1`,
  `v0.6.0`, `v0.6.1` through `v0.6.8`, `v0.7.0`, and `v0.8.0` are tagged and
  released.
- Milestone `0.9.0` has been released as `v0.9.0`.
- Milestone `0.10.0` is active. Issues #48 and #49 cover graph values and
  graph I/O helpers; #50 and #51 remain the next graph slices.
- `CHANGELOG.md` contains Unreleased graph and graphio entries for `v0.10.0`.

## Release Checklist

1. Finish issue #49 with Step 6-R P0=0/P1=0 and PR DoD evidence.
2. Continue #50 backend adapter evaluation and #51 domain examples.
3. Verify `make ci` on the release-prep branch when milestone `0.10.0` closes.
4. Close milestone `0.10.0`, promote `develop` to `main`, tag `v0.10.0`, and
   create the GitHub Release from `CHANGELOG.md`.

## Release Support Notes

The `graph` and `graph/graphio` packages have no service or runtime dependency.
Before `v0.10.0` is tagged, rollback is removing those packages plus docs and
release bookkeeping. After a tag, public API changes must preserve Go
compatibility or be deferred to a breaking release.
