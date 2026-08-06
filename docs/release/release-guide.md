# 릴리스 가이드

> 한국어 릴리스 경계: 이 문서는 릴리스 절차를 한국어 독자가 추적할 수 있도록 정리한다. 버전, 태그, 권한 게이트, 명령, CI 증거, GitHub 상태 이름은 원문의 재현성 앵커로 보존한다.

This guide defines the Go module release flow for `bluetape-go`.

## 모델

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

## 릴리스 사전 조건

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

## 표준 릴리스 흐름

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

## `v0.13.0` 릴리스 계획

`v0.13.0` contains the retrospective hardening pass for all work completed
through `v0.12.0`:

- 7-tier retrospective review over `0.1.0` through `0.12.0`, with confirmed
  P0/P1 findings routed to fixes.
- Missing stress and async cancellation coverage for existing concurrency,
  resilience, DynamoDB batchwrite, and testing-helper contracts.
- Cumulative lesson hardening for bounded cleanup contexts, caller-owned key
  preservation, and README cleanup examples.
- Feature-gap triage that implemented only the evidence-backed MongoDB
  Testcontainers fixture and deferred or rejected broader package ideas.

Release sequence:

1. Verify milestone `0.13.0` has zero open issues.
2. Merge the release-readiness PR so `CHANGELOG.md`, `WIP.md`, README locale
   files, and this release guide reflect `v0.13.0`.
3. Close milestone `0.13.0`.
4. Merge `develop` into `main` through a release PR.
5. Tag `main` as `v0.13.0`.
6. Create GitHub Release `v0.13.0`.
7. Update downstream consumers that should require
   `github.com/bluetape4k/bluetape-go v0.13.0`.
