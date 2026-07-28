# Release Guide

상세 release procedure는 [`docs/release/release-guide.md`](release/release-guide.md)에
있다.

## Branches

- `develop`은 기본 integration branch다.
- `main`은 release-only branch이며 `develop`에서 pull request를 통해 갱신해야
  한다.

## Versioning

첫 tag가 publish된 뒤에는 semantic versioning을 사용한다.

- `v0.1.0`: 첫 foundation release.
- `v0.x.0`: 새 package family 또는 의미 있는 feature group.
- `v0.x.y`: bug fix, docs, compatibility adjustment, 작은 non-breaking
  improvement.

`v1.0.0` 전에는 public API가 아직 바뀔 수 있다. breaking change는
`CHANGELOG.md`에 기록한다.

## Release Tag Criteria

- `README.md`와 `README.ko.md`가 release scope를 최신으로 반영한다.
- `CHANGELOG.md`에 target `vX.Y.Z` section으로 승격할 수 있는 `Unreleased`
  section이 있다.
- `WIP.md`가 target release-preparation state를 기록한다.
- 대응 milestone이 open issue 0개로 closed 상태다.
- local `make ci`가 통과한다.
- `develop`의 GitHub Actions CI가 통과한다.
- Nightly `smoke` 또는 `testcontainers` scope가 실제 Testcontainers 실행으로
  통과한다.
- Public package semantics, example, release note가 문서화되어 있다.

## Changelog Rule

`CHANGELOG.md`는 Keep a Changelog style을 유지한다.

- `Added`
- `Changed`
- `Deprecated`
- `Removed`
- `Fixed`
- `Security`

tagging할 때 `Unreleased` entry를 version section으로 옮긴다.

## Tag Procedure

1. `develop`이 green인지 확인한다.
2. `CHANGELOG.md`를 갱신한다.
3. `develop`에서 `main`으로 가는 release PR을 열고 merge한다.
4. `main`에 tag를 만든다.
5. tag를 push한다.
6. `CHANGELOG.md`에서 GitHub release note를 생성한다.
