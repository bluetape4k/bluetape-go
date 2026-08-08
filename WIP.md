# 진행 상황

스냅샷: 2026-08-06 KST
범위: `0.19.0` provider-foundation 릴리스 준비.

## 현재 대상 릴리스

`v0.19.0`은 필수 provider conformance와 Redis/SQL/etcd 기반의
조정, rate-limit, durable checkpoint, value-cache 기반을 묶는
provider-foundation 릴리스입니다. `leader/leadertest`, `lock/locktest`,
`ratelimit/ratelimittest`의 공통 검증 경계와 caller-owned resource 계약을
유지하고 Redis와 SQL의 commit-unknown 경계를 릴리스 runbook에 기록합니다.

마일스톤 `0.19.0`의 구현 이슈 21개와 병합된 PR 20개가 이 범위를
완료했습니다: #527, #528, #529, #530, #531, #532, #535, #536, #537, #560, #569,
#570, #571, #579, #581, #583, #585, #588, #590, #592, #594.

이슈 #611은 rate-limit provider cutover와 commit-unknown conformance를 위한
마일스톤 미지정 P2 후속 작업입니다. 이 이슈는 `v0.19.0`에서 명시적으로
제외하고 향후 마일스톤을 위해 열어 둡니다.

## 현재 상태

- `v0.1.0`, `v0.1.1`, `v0.2.0`, `v0.3.0`, `v0.4.0`, `v0.5.0`, `v0.5.1`,
  `v0.6.0`, `v0.6.1`부터 `v0.6.8`까지, `v0.7.0`, `v0.8.0`, `v0.9.0`,
  `v0.10.0`, `v0.11.0`, `v0.12.0`, `v0.13.0`, `v0.14.0`, `v0.15.0`,
  `v0.16.0`, `v0.17.0`, `v0.18.0`의 태그와 릴리스가 완료되었습니다.
- 마일스톤 `0.19.0`의 열린 이슈는 0개이며 구현 이슈 21개와 구현 PR 20개가
  모두 종료·병합 상태입니다.
- `CHANGELOG.md`에는 2026-08-06 날짜의 `v0.19.0` 릴리스 섹션이 있습니다.
- `v0.19.0` 태그와 GitHub Release는 아직 생성되지 않았습니다.
- `develop` 기준점 `e185baa99e762442239f5e3f376acd93ca9478c1`의 GitHub CI와
  최종 릴리스 준비 source tree의 로컬 `make ci`가 통과했습니다. 중간 재실행에서
  Testcontainers timing-sensitive package test가 간헐적으로 실패했지만,
  새로 실행한 full CI가 다시 통과해 로컬 validation gate를 닫았습니다.

## 릴리스 체크리스트

1. 이 릴리스 준비 브랜치를 `develop`에 merge하여 `CHANGELOG.md`, `WIP.md`,
   릴리스 체크리스트 근거가 `v0.19.0`을 반영하게 합니다.
2. 릴리스 준비 PR이 반영되고 changelog gate가 `develop`에 존재하면
   마일스톤 `0.19.0`을 닫습니다.
3. 검증된 `develop` tree를 릴리스 PR로 `main`에 승격합니다. 직접 PR이
   mergeable하지 않으면 protected-branch projection fallback을 사용합니다.
4. 로컬 `make ci`와 릴리스 PR의 GitHub CI를 검증합니다.
5. `main`에 `v0.19.0` 태그를 생성합니다.
6. validation evidence와 함께 `CHANGELOG.md`에서 GitHub Release를 생성합니다.

## 릴리스 지원 참고

0.19.0 범위는 provider conformance, PostgreSQL/etcd leader election,
PostgreSQL rate limiting과 durable checkpoint, Redis Fory/value cache, Redis
foundation을 publish합니다. Caller-owned database/client/resource ownership,
bounded cleanup, commit-unknown inspection, 자동 replay 금지 contract를
유지합니다. `v0.19.0` 태그 생성 전 rollback은 릴리스 PR을 닫고 릴리스 브랜치를
삭제하는 것입니다. 태그 이후의 릴리스 정정은 명시적인 retag plan 승인이
없는 한 patch release로 처리해야 합니다.
