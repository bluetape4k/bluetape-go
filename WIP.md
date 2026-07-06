# WIP

Snapshot: 2026-07-06 KST
Scope: `0.12.0` core foundation parity, rule primitives, and README diagram coverage.

## Current Target Release

`v0.12.0` - source-backed Go-native core, collection, codec, concurrency,
observability, and rules primitives without importing broad Kotlin/JVM helper
surfaces.

Issues #354, #359, #360, #357, #355, and #361 close the core-foundation parity
epic with narrow replacements for useful `bluetape4k-core` behavior. Issues
#375, #376, and #377 add first-party rule primitives, bounded inference, and
expression-backed YAML/JSON readers. PR #432 completes README diagram coverage
for packages that lacked visual contract evidence and repairs existing diagram
geometry.

## Current State

- `v0.1.0`, `v0.1.1`, `v0.2.0`, `v0.3.0`, `v0.4.0`, `v0.5.0`, `v0.5.1`,
  `v0.6.0`, `v0.6.1` through `v0.6.8`, `v0.7.0`, `v0.8.0`, `v0.9.0`,
  `v0.10.0`, and `v0.11.0` are tagged and released.
- Milestone `0.12.0` has zero open issues; #353, #354, #355, #357, #359,
  #360, #361, #375, #376, and #377 are closed.
- `CHANGELOG.md` contains the `v0.12.0` release section dated 2026-07-06.
- `v0.12.0` tag and GitHub Release are not created yet.

## Release Checklist

1. Merge this release-history PR to satisfy the changelog gate.
2. Close milestone `0.12.0`.
3. Verify `make ci` on the release-prep branch.
4. Promote `develop` to `main`, tag `v0.12.0`, and create the GitHub Release
   from `CHANGELOG.md`.

## Release Support Notes

The 0.12.0 core helpers, rules primitives, observability docs, and README
diagram assets do not introduce external service requirements beyond existing
Testcontainers-backed CI coverage. Before `v0.12.0` is tagged, rollback is
removing the 0.12.0 packages, docs, and release bookkeeping. After a tag,
public API changes must preserve Go compatibility or be deferred to a breaking
release.
