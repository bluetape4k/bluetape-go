# Release Guide

This guide defines the Go module release flow for `bluetape-go`.

## Model

`bluetape-go` is a Go module. Consumers select versions through semantic Git
tags, not through branch names. A release tag such as `v0.3.0` points at one
commit, and Go tools resolve that tag as the module version.

Branch roles:

- `develop` is the integration branch.
- `main` is the release branch.
- Release tags are created on `main`, after `develop` has been promoted through
  a pull request.

Versioning:

- `v0.x.0` is used for milestone feature groups before `v1.0.0`.
- `v0.x.y` is used for patch fixes and release hygiene updates.
- `v0` versions are valid Go module releases, but they do not promise stable
  public API compatibility.
- `v0` and `v1` module paths do not use a major-version suffix. A `/v2` module
  path suffix is only needed when publishing `v2.0.0` or later.

References:

- Go module release workflow: <https://go.dev/doc/modules/release-workflow>
- Go module version numbers: <https://go.dev/doc/modules/version-numbers>
- Go module reference: <https://go.dev/ref/mod>

## Release Preconditions

Before creating a release PR:

1. The milestone has no open issues.
2. The epic issue is closed after its child checklist is updated.
3. `CHANGELOG.md` contains `## [vX.Y.Z] - YYYY-MM-DD`.
4. `docs/release/release-guide.md` reflects the current release policy.
5. No `vX.Y.Z` tag exists locally or remotely.
6. No GitHub Release exists for `vX.Y.Z`.
7. Local validation passes.
8. GitHub CI passes on the target `develop` commit.

Useful preflight commands:

```bash
git status --short --branch
git fetch --prune origin main develop --tags
git log --oneline origin/main..origin/develop
gh issue list --repo bluetape4k/bluetape-go --milestone "X.Y.Z" --state open
gh pr list --repo bluetape4k/bluetape-go --state open
git tag --list "vX.Y.Z"
git ls-remote --tags origin "refs/tags/vX.Y.Z*"
gh release view vX.Y.Z --repo bluetape4k/bluetape-go
rg -n "## \\[vX\\.Y\\.Z\\]" CHANGELOG.md
make ci
```

## Standard Release Flow

1. Prepare `develop`.
   - Update `CHANGELOG.md`.
   - Update `WIP.md`.
   - Update release docs when the process changes.
   - Run local validation.

2. Close milestone bookkeeping.
   - Verify milestone open issue count is zero.
   - Close the GitHub milestone.

3. Promote `develop` to `main`.
   - Open a release PR with base `main` and head `develop`.
   - Verify the PR body includes release scope, validation evidence, milestone
     status, and tag plan.
   - Wait for PR CI to pass.
   - Merge the release PR.

4. Tag the release commit.
   - Fetch `main`.
   - Fast-forward local `main` to `origin/main`.
   - Confirm `CHANGELOG.md` on `main` contains the version section.
   - Create an annotated tag on `main`:

```bash
git switch main
git pull --ff-only origin main
git tag -a vX.Y.Z -m "Release vX.Y.Z"
git push origin vX.Y.Z
```

5. Create the GitHub Release.
   - Use the matching `CHANGELOG.md` section as the release note source.
   - Include validation evidence and tag target commit.

6. Update downstream modules.
   - In each consumer repository:

```bash
go get github.com/bluetape4k/bluetape-go@vX.Y.Z
go mod tidy
go test ./...
```

   - Commit `go.mod` and `go.sum` changes.
   - Open a PR in the consumer repository.

## Current Release Notes

`v0.7.0` is the current published release. Its protected-branch promotion
projected the full `origin/develop` tree onto `main`, so later milestone
bookkeeping must verify tree deltas before creating additional tags.

Milestone `0.8.0` currently has zero open issues, but
`git diff v0.7.0^{}..origin/develop` is empty and both refs resolve to the same
tree object. Do not publish `v0.8.0` unless new commits land after `v0.7.0`.
Close `0.8.0` as already shipped by `v0.7.0` and continue with the next
milestone that has an actual content delta.

The next active work target is milestone `0.9.0`, currently scoped to text
search and tokenizer packages.
