# WIP

Snapshot: 2026-07-08 KST
Scope: `0.16.0` probabilistic Redis structure expansion after Redis Bloom.

## Current Target Release

`v0.16.0` - Redis probabilistic follow-up scope that adds core Redis
HyperLogLog support, Testcontainers-backed Redis probabilistic validation, and
operator documentation that keeps RedisBloom `CF*` Cuckoo support explicitly
module-gated for a future issue.

Issues #410 through #413 close the 0.16.0 probabilistic Redis slice under epic
#409. Issue #410 records the Cuckoo/HLL decision, #411 implements Redis
HyperLogLog, #412 adds Testcontainers and stress/race coverage, and #413
documents Redis module assumptions and README runtime visuals.

## Current State

- `v0.1.0`, `v0.1.1`, `v0.2.0`, `v0.3.0`, `v0.4.0`, `v0.5.0`, `v0.5.1`,
  `v0.6.0`, `v0.6.1` through `v0.6.8`, `v0.7.0`, `v0.8.0`, `v0.9.0`,
  `v0.10.0`, `v0.11.0`, `v0.12.0`, `v0.13.0`, `v0.14.0`, and `v0.15.0` are
  tagged and released.
- Milestone `0.16.0` has zero open issues; #409, #410, #411, #412, #413, and
  #466 are closed.
- `CHANGELOG.md` contains the `v0.16.0` release section dated 2026-07-08.
- `v0.16.0` tag and GitHub Release are not created yet.

## Release Checklist

1. Merge this release-prep branch to `develop` so `CHANGELOG.md`, `WIP.md`,
   README locale files, and release checklist evidence reflect `v0.16.0`.
2. Close milestone `0.16.0` after the release-prep PR lands and the changelog
   gate is present on `develop`.
3. Promote the verified `develop` tree to `main` through a release PR or the
   protected-branch projection fallback if the direct PR is not mergeable.
4. Verify `make ci` locally and GitHub CI on the release PR.
5. Tag `v0.16.0` on `main`.
6. Create the GitHub Release from `CHANGELOG.md` with validation evidence.

## Release Support Notes

The 0.16.0 probabilistic Redis slice adds HyperLogLog on core Redis commands
and strengthens live Redis validation. It does not add RedisBloom module-backed
Cuckoo APIs. Before `v0.16.0` is tagged, rollback is closing the release PR and
deleting the release branch. After a tag, release corrections should use a
patch release unless an explicit retag plan is approved.
