# WIP

Snapshot: 2026-06-29 KST
Scope: `0.9.0` audit and event package release.

## Current Target Release

`v0.9.0` - audit/event model, history query contracts, SQL outbox adapter, and
runnable audit example.

The milestone implementation is complete. Issue #56 introduced the
storage-neutral aggregate event and audit model: stable aggregate identity,
positive revisions, caller-owned event IDs, idempotency keys, validated JSON
audit entries, non-destructive pending-event handoff, and history
reconstruction. Issue #57 added repository/history query contracts, reusable
adapter conformance tests, and a goroutine-safe non-durable in-memory
repository. Issue #58 selected SQL outbox store plus relay contracts as the
first durable publisher target, and #346 implemented the PostgreSQL-backed
store and relay. Issue #59 added the runnable audit example service and README
diagram that shows the source-state, audit-history, and outbox boundaries.

## Current State

- `0.1.0`, `0.1.1`, `0.2.0`, `0.3.0`, `0.4.0`, `0.5.0`, `v0.5.1`,
  `v0.6.0`, `v0.6.1` through `v0.6.8`, `v0.7.0`, and `v0.8.0` are tagged and
  released.
- Milestone `0.9.0` has zero open issues. Child issues #56, #57, #58, and #59
  are closed, and the epic #46 is closed.
- `CHANGELOG.md` contains the `## [v0.9.0] - 2026-06-29` section required
  before tagging `v0.9.0`.

## Release Checklist

1. Verify `make ci` locally on the release-prep branch.
2. Merge release-prep into `develop` through a PR.
3. Close milestone `0.9.0`.
4. Promote `develop` to `main` through a release PR.
5. Tag `main` as `v0.9.0` and create the GitHub Release from `CHANGELOG.md`.
