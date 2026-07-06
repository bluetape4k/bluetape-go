# WIP

Snapshot: 2026-07-07 KST
Scope: `0.13.0` retrospective review, P0/P1 closure, cumulative hardening, stress coverage, and feature-gap triage.

## Current Target Release

`v0.13.0` - release-readiness hardening for all work completed through
`v0.12.0`, without broadening the Go API surface beyond evidence-backed fixes.

Issues #424, #443, #425, #429, #426, #427, and #428 close the 0.13.0
retrospective track. The only must-have feature addition found by the gap
triage was the reusable MongoDB Testcontainers fixture delivered through #430.
All other feature candidates are deferred to 0.14.0+, later package-specific
issues, Backlog, or rejected by Go-native rationale.

## Current State

- `v0.1.0`, `v0.1.1`, `v0.2.0`, `v0.3.0`, `v0.4.0`, `v0.5.0`, `v0.5.1`,
  `v0.6.0`, `v0.6.1` through `v0.6.8`, `v0.7.0`, `v0.8.0`, `v0.9.0`,
  `v0.10.0`, `v0.11.0`, and `v0.12.0` are tagged and released.
- Milestone `0.13.0` has no open child implementation blockers after #426 and
  #427 were closed; #428 and epic #423 remain as release-readiness closure
  bookkeeping until this audit is merged and the milestone is closed.
- `CHANGELOG.md` contains the `v0.13.0` release section dated 2026-07-07.
- `v0.13.0` tag and GitHub Release are not created yet.

## Release Checklist

1. Merge the #428 release-readiness audit PR so `CHANGELOG.md`, `WIP.md`,
   README locale files, and the release guide reflect `v0.13.0`.
2. Close #428 and epic #423 after the audit PR is merged and CI is green.
3. Close milestone `0.13.0` once no open milestone issues remain.
4. Promote the target release branch to `main` according to
   `docs/release/release-guide.md`, tag `v0.13.0` on `main`, and create the
   GitHub Release from `CHANGELOG.md`.

## Release Support Notes

The 0.13.0 line is a retrospective hardening release. It fixes confirmed P1
behavior, adds missing stress/race evidence, publishes cumulative lesson
hardening notes, and adds only the MongoDB Testcontainers fixture that existing
JWT Mongo tests already needed. Before `v0.13.0` is tagged, rollback is
removing the 0.13.0 docs and release bookkeeping commits. After a tag, public
API changes must preserve Go compatibility or be deferred to a later release.
