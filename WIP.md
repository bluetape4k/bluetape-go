# WIP

Snapshot: 2026-07-03 KST
Scope: `0.11.0` image, encryption, and graph adapter milestone.

## Current Target Release

`v0.11.0` - bounded image helpers, optional libvips example boundary,
stdlib-backed authenticated encryption, Neo4j/Memgraph graph adapter proofs,
IAM access graph example, and rule-engine boundary research.

Issue #309 adds `imagekit` as a dependency-light pure-Go helper package for
bounded image decode, resize, thumbnail, conversion, and encode workflows.
Issue #310 keeps libvips integration as an optional example module under
`examples/imagekit-govips` instead of promoting `govips` into the core module
dependency graph. Issue #315 adds the first `encrypt` package slice around
stdlib AES-GCM framing, validation, AAD binding, and tamper coverage. Issues
#365 and #366 prove a Neo4j-driver adapter boundary plus Memgraph
compatibility tests before broader graph backend abstractions. Issue #368 adds
`examples/graph/iamaccess` as the second runnable graph example, showing
identity, role, policy, and resource access path analysis. Issue #37 records
that rule-engine primitives need a Go-native boundary before any public runtime
package is added.

## Current State

- `v0.1.0`, `v0.1.1`, `v0.2.0`, `v0.3.0`, `v0.4.0`, `v0.5.0`, `v0.5.1`,
  `v0.6.0`, `v0.6.1` through `v0.6.8`, `v0.7.0`, `v0.8.0`, `v0.9.0`, and
  `v0.10.0` are tagged and released.
- Milestone `0.11.0` has zero open issues; #37, #309, #310, #315, #365,
  #366, and #368 are closed.
- `CHANGELOG.md` contains the `v0.11.0` release section dated 2026-07-03.
- `v0.11.0` tag and GitHub Release are not created yet.

## Release Checklist

1. Merge this release-history PR to satisfy the changelog gate.
2. Close milestone `0.11.0`.
3. Verify `make ci` on the release-prep branch.
4. Promote `develop` to `main`, tag `v0.11.0`, and
   create the GitHub Release from `CHANGELOG.md`.

## Release Support Notes

The `imagekit`, `encrypt`, `graph/neo4j`, and runnable example additions do not
start background services. Before `v0.11.0` is tagged, rollback is removing the
0.11.0 packages, examples, docs, and release bookkeeping. After a tag, public
API changes must preserve Go compatibility or be deferred to a breaking
release.
