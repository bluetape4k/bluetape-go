# WIP

Snapshot: 2026-06-27 KST
Scope: `0.8.0` text search package release preparation.

## Current Target Release

`v0.8.0` - text search, blockword masking, tokenizer adapters, and language detection.

The milestone is complete. `textsearch` provides immutable compiled matchers,
overlap-aware first/all match modes, replacement/masking hooks, Unicode
normalization, explicit boundary modes, blockword dictionaries, tokenizer core
interfaces, and optional Kagome/Lingua-Go adapters without adding those larger
dependencies to the base package.

## Current State

- `0.1.0`, `0.1.1`, `0.2.0`, `0.3.0`, `0.4.0`, `0.5.0`, `v0.5.1`,
  `v0.6.0`, `v0.6.1` through `v0.6.8`, and `v0.7.0` are tagged and released.
- Milestone `0.8.0` has zero open issues and epic #45 is closed.
- Issues #52, #53, #54, #55, #336, and #337 are complete.
- `CHANGELOG.md` contains the `v0.8.0` section required before tagging.

## Release Checklist

1. Verify `make ci` locally on the release-prep branch.
2. Merge release-prep into `develop` through a PR.
3. Close milestone `0.8.0`.
4. Promote `develop` to `main` through a release PR.
5. Tag `main` as `v0.8.0` and create the GitHub Release from `CHANGELOG.md`.
