# Release Guide

## Branches

- `develop` is the default integration branch.
- `main` is release-only and should be updated from `develop` through a pull
  request.

## Versioning

Use semantic versioning once the first tag is published.

- `v0.1.0`: first foundation release.
- `v0.x.0`: new package families or meaningful feature groups.
- `v0.x.y`: bug fixes, docs, compatibility adjustments, and small
  non-breaking improvements.

Before `v1.0.0`, public APIs may still change. Document breaking changes in
`CHANGELOG.md`.

## `v0.1.0` Tag Criteria

- `README.md` and `README.ko.md` are current.
- `CHANGELOG.md` has an `Unreleased` section that can become `v0.1.0`.
- `docs/research/2026-06-01-milestone-0.1.0-foundation-research.md` reflects
  final release scope.
- `make ci` passes locally.
- GitHub Actions CI passes on `develop`.
- Redis leader API semantics are documented.
- Redis compatibility decision with `bluetape4k-leader` is documented.
- At least two representative examples exist or the missing examples are listed
  in release notes as deferred.

## Changelog Rule

Keep `CHANGELOG.md` in Keep a Changelog style:

- `Added`
- `Changed`
- `Deprecated`
- `Removed`
- `Fixed`
- `Security`

Move entries from `Unreleased` into the version section when tagging.

## Tag Procedure

1. Ensure `develop` is green.
2. Update `CHANGELOG.md`.
3. Open and merge a `develop` to `main` release PR.
4. Tag on `main`.
5. Push the tag.
6. Create GitHub release notes from `CHANGELOG.md`.

