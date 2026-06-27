# WIP

Snapshot: 2026-06-27 KST
Scope: `0.9.0` audit and event package implementation.

## Current Target Release

`v0.9.0` - audit/event model, history query contracts, outbox adapters, and
examples.

The milestone is in progress. Issue #56 started the package with a
storage-neutral aggregate event and audit model: stable aggregate identity,
positive revisions, caller-owned event IDs, idempotency keys, validated JSON
audit entries, non-destructive pending-event handoff, and history
reconstruction. Issue #57 adds repository/history query contracts, reusable
adapter conformance tests, and a goroutine-safe non-durable in-memory
repository. Outbox, SQL/Redis/Kafka/NATS adapters, and example services remain
tracked by later `0.9.0` issues.

## Current State

- `0.1.0`, `0.1.1`, `0.2.0`, `0.3.0`, `0.4.0`, `0.5.0`, `v0.5.1`,
  `v0.6.0`, `v0.6.1` through `v0.6.8`, `v0.7.0`, and `v0.8.0` are tagged and
  released.
- Milestone `0.9.0` is open. Issues #56 and #57 are the first implementation
  slices.
- `CHANGELOG.md` contains the `[Unreleased]` audit package entry required before
  tagging a future `v0.9.0`.

## Release Checklist

1. Complete the remaining `0.9.0` audit issues and close the milestone epic.
2. Verify `make ci` locally on the release-prep branch.
3. Merge release-prep into `develop` through a PR.
4. Close milestone `0.9.0`.
5. Promote `develop` to `main` through a release PR.
6. Tag `main` as `v0.9.0` and create the GitHub Release from `CHANGELOG.md`.
