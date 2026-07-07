# WIP

Snapshot: 2026-07-07 KST
Scope: `0.14.0` cross-repo SerDe and compression benchmark baseline.

## Current Target Release

`v0.14.0` - reproducible serialization, codec, and compression benchmark
baseline across Go, Rust, and JVM bluetape modules, with raw evidence retained
before recommendation claims.

Issues #399, #400, #401, #402, and #403 close the 0.14.0 benchmark baseline
track under epic #398. Follow-up profiling candidates from #403 were filed as
#455 and #456 for a later milestone; they are not release blockers for
`v0.14.0`.

## Current State

- `v0.1.0`, `v0.1.1`, `v0.2.0`, `v0.3.0`, `v0.4.0`, `v0.5.0`, `v0.5.1`,
  `v0.6.0`, `v0.6.1` through `v0.6.8`, `v0.7.0`, `v0.8.0`, `v0.9.0`,
  `v0.10.0`, `v0.11.0`, `v0.12.0`, and `v0.13.0` are tagged and released.
- Milestone `0.14.0` is closed with zero open issues; #398 through #403 are
  closed.
- `CHANGELOG.md` contains the `v0.14.0` release section dated 2026-07-07.
- `v0.14.0` tag and GitHub Release are not created yet.
- The 0.15.0 #405 release-target research commit
  `47d9860ef88a5ed1f5b52bcfeb7f3a47a08aa696` is intentionally excluded from
  this release branch.

## Release Checklist

1. Merge this release branch to `main` so `CHANGELOG.md`, `WIP.md`, raw
   benchmark outputs, and README evidence links reflect `v0.14.0`.
2. Verify `make ci` locally and GitHub CI on the release PR.
3. Tag `v0.14.0` on `main`.
4. Create the GitHub Release from `CHANGELOG.md` with validation evidence.

## Release Support Notes

The 0.14.0 line is a benchmark and documentation release. It adds benchmark
test files and preserved raw outputs, but it does not change runtime public API
contracts. Before `v0.14.0` is tagged, rollback is closing the release PR and
deleting the release branch. After a tag, release corrections should use a
patch release unless a named retag plan is explicitly approved.
