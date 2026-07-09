# WIP

Snapshot: 2026-07-10 KST
Scope: `0.18.0` ecosystem follow-up release prep.

## Current Target Release

`v0.18.0` - ecosystem follow-up scope that adds MongoDB group and strategic
leader elector providers, bounded GraphML graph I/O, and the first broker-backed
audit sqloutbox publisher provider through Redis Streams.

Issues #489, #490, #491, and #533 close the 0.18.0 implementation slice. The
release extends the existing leader, graph, and audit packages without changing
the shared public contracts that were tagged in `v0.17.0`.

## Current State

- `v0.1.0`, `v0.1.1`, `v0.2.0`, `v0.3.0`, `v0.4.0`, `v0.5.0`, `v0.5.1`,
  `v0.6.0`, `v0.6.1` through `v0.6.8`, `v0.7.0`, `v0.8.0`, `v0.9.0`,
  `v0.10.0`, `v0.11.0`, `v0.12.0`, `v0.13.0`, `v0.14.0`, `v0.15.0`, and
  `v0.16.0`, and `v0.17.0` are tagged and released.
- Milestone `0.18.0` has zero open issues; #489, #490, #491, and #533 are
  closed.
- `CHANGELOG.md` contains the `v0.18.0` release section dated 2026-07-10.
- `v0.18.0` tag and GitHub Release are not created yet.

## Release Checklist

1. Merge this release-prep branch to `develop` so `CHANGELOG.md`, `WIP.md`,
   README locale files, and release checklist evidence reflect `v0.18.0`.
2. Close milestone `0.18.0` after the release-prep PR lands and the changelog
   gate is present on `develop`.
3. Promote the verified `develop` tree to `main` through a release PR or the
   protected-branch projection fallback if the direct PR is not mergeable.
4. Verify `make ci` locally and GitHub CI on the release PR.
5. Tag `v0.18.0` on `main`.
6. Create the GitHub Release from `CHANGELOG.md` with validation evidence.

## Release Support Notes

The 0.18.0 slice publishes provider follow-ups for MongoDB leader election,
GraphML interchange, and Redis Streams audit delivery while preserving caller
ownership of MongoDB collections, XML stream limits, Redis stream keys, and
outbox idempotency metadata. Before `v0.18.0` is tagged, rollback is closing the
release PR and deleting the release branch. After a tag, release corrections
should use a patch release unless an explicit retag plan is approved.
