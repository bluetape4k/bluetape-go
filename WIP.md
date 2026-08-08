# WIP

스냅샷: 2026-08-06 KST
범위: `0.19.0` provider-foundation release 준비.

## 현재 Target Release

`v0.19.0`은 mandatory provider conformance와 Redis/SQL/etcd 기반의
coordination, rate-limit, durable checkpoint, value-cache foundation을 묶는
provider-foundation release다. `leader/leadertest`, `lock/locktest`,
`ratelimit/ratelimittest`의 공통 검증 경계와 caller-owned resource 계약을
유지하며, Redis와 SQL의 commit-unknown 경계를 release runbook에 기록한다.

Milestone `0.19.0`의 implementation issue 21개와 merged PR 20개가 이 범위를
닫는다: #527, #528, #529, #530, #531, #532, #535, #536, #537, #560, #569,
#570, #571, #579, #581, #583, #585, #588, #590, #592, #594.

Issue #611은 rate-limit provider cutover와 commit-unknown conformance를 위한
unmilestoned P2 후속 작업이다. 이 issue는 `v0.19.0`에서 명시적으로 제외하고
향후 milestone을 위해 open 상태로 둔다.

## 현재 상태

- `v0.1.0`, `v0.1.1`, `v0.2.0`, `v0.3.0`, `v0.4.0`, `v0.5.0`, `v0.5.1`,
  `v0.6.0`, `v0.6.1`부터 `v0.6.8`까지, `v0.7.0`, `v0.8.0`, `v0.9.0`,
  `v0.10.0`, `v0.11.0`, `v0.12.0`, `v0.13.0`, `v0.14.0`, `v0.15.0` 및
  `v0.16.0`, `v0.17.0`, `v0.18.0`은 tag와 release가 끝났다.
- Milestone `0.19.0`의 open issue는 0개이며 implementation issue 21개와
  implementation PR 20개가 모두 closed/merged 상태다.
- `CHANGELOG.md`에는 2026-08-06 날짜의 `v0.19.0` release section이 있다.
- `v0.19.0` tag와 GitHub Release는 아직 생성되지 않았다.
- `develop` baseline `e185baa99e762442239f5e3f376acd93ca9478c1`의 GitHub CI와
  final release-prep source tree의 local `make ci`가 통과했다. 중간 재실행에서
  Testcontainers timing-sensitive package test가 간헐적으로 실패했지만, fresh
  full CI가 다시 통과해 local validation gate를 닫았다.

## Release checklist

1. 이 release-prep branch를 `develop`에 merge해서 `CHANGELOG.md`, `WIP.md`,
   release checklist evidence가 `v0.19.0`을 반영하게 한다.
2. release-prep PR이 landing되고 changelog gate가 `develop`에 존재하면
   milestone `0.19.0`을 닫는다.
3. 검증된 `develop` tree를 release PR로 `main`에 승격한다. direct PR이
   mergeable하지 않으면 protected-branch projection fallback을 사용한다.
4. local `make ci`와 release PR의 GitHub CI를 검증한다.
5. `main`에 `v0.19.0` tag를 생성한다.
6. validation evidence와 함께 `CHANGELOG.md`에서 GitHub Release를 생성한다.

## Release 지원 메모

0.19.0 slice는 provider conformance, PostgreSQL/etcd leader election,
PostgreSQL rate limiting과 durable checkpoint, Redis Fory/value cache, Redis
foundation을 publish한다. Caller-owned database/client/resource ownership,
bounded cleanup, commit-unknown inspection, 자동 replay 금지 contract를
유지한다. `v0.19.0` tag 생성 전 rollback은 release PR을 닫고 release branch를
삭제하는 것이다. tag 이후의 release correction은 명시적인 retag plan 승인이
없는 한 patch release로 처리해야 한다.
