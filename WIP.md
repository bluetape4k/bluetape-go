# WIP

Snapshot: 2026-07-10 KST
Scope: `0.18.0` ecosystem follow-up release prep.

## 현재 Target Release

`v0.18.0`은 MongoDB group/strategic leader elector provider, bounded GraphML
graph I/O, 그리고 Redis Streams 기반의 첫 broker-backed audit sqloutbox
publisher provider를 추가하는 ecosystem follow-up 범위다.

Issue #489, #490, #491, #533은 0.18.0 implementation slice를 닫는다. 이
release는 `v0.17.0`에서 tag된 shared public contract를 바꾸지 않고 기존
`leader`, `graph`, `audit` package를 확장한다.

## 현재 상태

- `v0.1.0`, `v0.1.1`, `v0.2.0`, `v0.3.0`, `v0.4.0`, `v0.5.0`, `v0.5.1`,
  `v0.6.0`, `v0.6.1` through `v0.6.8`, `v0.7.0`, `v0.8.0`, `v0.9.0`,
  `v0.10.0`, `v0.11.0`, `v0.12.0`, `v0.13.0`, `v0.14.0`, `v0.15.0`, and
  `v0.16.0`, and `v0.17.0`은 tag와 release가 끝났다.
- Milestone `0.18.0`의 open issue는 0개이며 #489, #490, #491, #533은
  closed 상태다.
- `CHANGELOG.md`에는 2026-07-10 날짜의 `v0.18.0` release section이 있다.
- `v0.18.0` tag와 GitHub Release는 아직 생성되지 않았다.

## Release Checklist

1. 이 release-prep branch를 `develop`에 merge해서 `CHANGELOG.md`, `WIP.md`,
   README locale file, release checklist evidence가 `v0.18.0`을 반영하게 한다.
2. release-prep PR이 landing되고 changelog gate가 `develop`에 존재하면
   milestone `0.18.0`을 닫는다.
3. 검증된 `develop` tree를 release PR로 `main`에 승격한다. direct PR이
   mergeable하지 않으면 protected-branch projection fallback을 사용한다.
4. local `make ci`와 release PR의 GitHub CI를 검증한다.
5. `main`에 `v0.18.0` tag를 생성한다.
6. validation evidence와 함께 `CHANGELOG.md`에서 GitHub Release를 생성한다.

## Release Support Notes

0.18.0 slice는 MongoDB leader election, GraphML interchange, Redis Streams audit
delivery의 provider follow-up을 publish한다. 동시에 MongoDB collection, XML
stream limit, Redis stream key, outbox idempotency metadata의 caller ownership을
보존한다. `v0.18.0` tag 생성 전 rollback은 release PR을 닫고 release branch를
삭제하는 것이다. tag 이후의 release correction은 명시적인 retag plan 승인이
없는 한 patch release로 처리해야 한다.
