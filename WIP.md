# WIP

Snapshot: 2026-07-09 KST
Scope: `0.17.0` workshop adoption sync and MongoDB leader follow-up.

## Current Target Release

`v0.17.0` - workshop adoption sync and MongoDB leader follow-up scope that
links source-checked workshop examples, records cross-repo workshop follow-up
issues, adds the MongoDB single leader elector backend, and documents the
application-owned OpenTelemetry bridge boundary.

Issues #414 through #418 close the 0.17.0 workshop adoption slice. The release
also includes the MongoDB single leader backend work and OpenTelemetry bridge
documentation that landed on `develop` before tagging.

## Current State

- `v0.1.0`, `v0.1.1`, `v0.2.0`, `v0.3.0`, `v0.4.0`, `v0.5.0`, `v0.5.1`,
  `v0.6.0`, `v0.6.1` through `v0.6.8`, `v0.7.0`, `v0.8.0`, `v0.9.0`,
  `v0.10.0`, `v0.11.0`, `v0.12.0`, `v0.13.0`, `v0.14.0`, `v0.15.0`, and
  `v0.16.0` are tagged and released.
- Milestone `0.17.0` has zero open issues; #414, #415, #416, #417, and #418 are
  closed.
- `CHANGELOG.md` contains the `v0.17.0` release section dated 2026-07-09.
- `v0.17.0` tag and GitHub Release are not created yet.

## Release Checklist

1. Merge this release-prep branch to `develop` so `CHANGELOG.md`, `WIP.md`,
   README locale files, and release checklist evidence reflect `v0.17.0`.
2. Close milestone `0.17.0` after the release-prep PR lands and the changelog
   gate is present on `develop`.
3. Promote the verified `develop` tree to `main` through a release PR or the
   protected-branch projection fallback if the direct PR is not mergeable.
4. Verify `make ci` locally and GitHub CI on the release PR.
5. Tag `v0.17.0` on `main`.
6. Create the GitHub Release from `CHANGELOG.md` with validation evidence.

## Release Support Notes

The 0.17.0 slice publishes workshop adoption evidence, MongoDB leader backend
documentation and behavior, and OpenTelemetry bridge guidance without moving
exporter ownership into this reusable library. Before `v0.17.0` is tagged,
rollback is closing the release PR and deleting the release branch. After a
tag, release corrections should use a patch release unless an explicit retag
plan is approved.
