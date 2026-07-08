# WIP

Snapshot: 2026-07-08 KST
Scope: `0.15.0` audit publisher adoption, sqloutbox test publishers, and
SerDe allocation follow-up hardening.

## Current Target Release

`v0.15.0` - deterministic audit publisher test/adoption helpers plus focused
JSON and zstd allocation reductions, without selecting Kafka, NATS, Redis
Streams, or another durable broker adapter before a concrete consumer needs it.

Issues #405 through #408 close the audit publisher adoption slice under epic
#404. Issues #455 and #456 close the SerDe follow-up profiling slice by
preserving evidence and reducing allocation pressure in zstd byte-slice
compression and default JSON decoding.

## Current State

- `v0.1.0`, `v0.1.1`, `v0.2.0`, `v0.3.0`, `v0.4.0`, `v0.5.0`, `v0.5.1`,
  `v0.6.0`, `v0.6.1` through `v0.6.8`, `v0.7.0`, `v0.8.0`, `v0.9.0`,
  `v0.10.0`, `v0.11.0`, `v0.12.0`, `v0.13.0`, and `v0.14.0` are tagged and
  released.
- Milestone `0.15.0` is closed with zero open issues; #404, #405, #406, #407,
  #408, #455, and #456 are closed.
- `CHANGELOG.md` contains the `v0.15.0` release section dated 2026-07-08.
- `v0.15.0` tag and GitHub Release are not created yet.

## Release Checklist

1. Merge this release branch to `main` so `CHANGELOG.md`, `WIP.md`, audit
   publisher docs, and SerDe allocation evidence reflect `v0.15.0`.
2. Verify `make ci` locally and GitHub CI on the release PR.
3. Tag `v0.15.0` on `main`.
4. Create the GitHub Release from `CHANGELOG.md` with validation evidence.

## Release Support Notes

The 0.15.0 audit publisher slice adds deterministic test/example publishers
and adoption documentation, not a durable broker adapter or new external
runtime dependency. The SerDe follow-up slice changes local JSON/zstd hot paths
while preserving existing public APIs. Before `v0.15.0` is tagged, rollback is
closing the release PR and deleting the release branch. After a tag, release
corrections should use a patch release unless an explicit retag plan is
approved.
