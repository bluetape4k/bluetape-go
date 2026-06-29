# WIP

Snapshot: 2026-06-29 KST
Scope: `0.10.0` graph package milestone.

## Current Target Release

`v0.10.0` - graph model values, graph I/O helpers, backend evaluation, and
domain examples.

Issue #48 introduces the first `graph` package as model-only values: stable
element IDs, labels, vertices, directed edges, paths, shallow properties,
redacted validation errors, and validated JSON. Repository, session, schema,
query, transaction, backend, and algorithm contracts remain out of scope until
#49, #50, and #51 prove shared behavior through I/O helpers, backend evaluation,
and examples.

## Current State

- `0.1.0`, `0.1.1`, `0.2.0`, `0.3.0`, `0.4.0`, `0.5.0`, `v0.5.1`,
  `v0.6.0`, `v0.6.1` through `v0.6.8`, `v0.7.0`, and `v0.8.0` are tagged and
  released.
- Milestone `0.9.0` has been released as `v0.9.0`.
- Milestone `0.10.0` is active. Issue #48 is the first implementation slice and
  keeps graph support to model values plus documentation and release
  bookkeeping.
- `CHANGELOG.md` contains an Unreleased graph package entry for `v0.10.0`.

## Release Checklist

1. Finish issue #48 with Step 6-R P0=0/P1=0 and PR DoD evidence.
2. Continue #49 graph I/O helpers, #50 backend adapter evaluation, and #51
   domain examples.
3. Verify `make ci` on the release-prep branch when milestone `0.10.0` closes.
4. Close milestone `0.10.0`, promote `develop` to `main`, tag `v0.10.0`, and
   create the GitHub Release from `CHANGELOG.md`.

## Release Support Notes

The `graph` package has no service or runtime dependency. Before `v0.10.0` is
tagged, rollback is removing `graph` plus docs and release bookkeeping. After a
tag, public API changes must preserve Go compatibility or be deferred to a
breaking release.
