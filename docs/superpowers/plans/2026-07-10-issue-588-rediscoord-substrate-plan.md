# Redis Cache Coordinator Substrate Migration Plan

> 한국어 재작성 범위: 이 계획 문서는 한국어 운영 문서로 읽히도록 제목, 판단, 작업 설명, 위험, 검증, 롤백 문맥을 한국어로 정리한다. 명령, 경로, API 이름, 이슈/PR 번호, 브랜치명, 코드 블록, 테스트 출력 같은 증거 문자열은 정확성을 위해 원문 그대로 보존한다.


**목표:** Standardize `cache/rediscoord` direct Redis provider 진단 on
the 공유 substrate 변경하지 않고 cache-coordination behavior 또는 stored
data.

**아키텍처:** 유지 local key formatting, opaque result envelopes, 기존
duration handling, 및 the migrated `lock/redis` lease boundary. 추가 one small
operation-오류 helper that joins a late 호출자 context 함께 the provider 원인
및 constructs a redacted `redis.OpError`.

## 작업 1: Failing Regression Coverage

- [x] 추가 closed-client 테스트 for `readOwnerResult`, `ownerToken`,
  `ensureOwner`, 및 `storeResult`.
- [x] 검증 original 원인, typed operation 오류, stable redacted key ID, 및
  없음 raw key/token/payload marker in formatted 오류.
- [x] 추가 a direct unit-level late-context 테스트. It deterministically verifies
  the 오류 boundary without relying on an unreliable network race.
- [x] 실행 the focused 테스트 set 및 capture the pre-implementation failure.

## 작업 2: Minimal Error-Boundary Migration

- [x] 가져오기 the 공유 `redis` 패키지 as `btredis`.
- [x] 추가 a private operation-오류 helper 함께 stable family/operation labels.
- [x] 교체 만 direct provider 오류 returns from result read, owner read,
  owner check, 및 result write.
- [x] 보존 `redis.Nil` branches 및 모든 preflight/sentinel behavior.
- [x] Format 및 run targeted serial 테스트.

## 작업 3: Documentation And 검증

- [x] 업데이트 영문 및 한국어 패키지 README 만 to describe the preserved
  `errors.Is`/`errors.As` 원인 계약 및 redacted 진단.
- [x] 다음을 하지 않는다: refresh benchmark data; record N/A 및 #560 ownership in review.
- [x] 실행 normal/race 패키지 테스트, 공유 dependency 테스트, static gate, 및
  `make ci`.

## 작업 4: 리뷰 And Publication

- [x] 기록 a local six-perspective 7-Tier review (native review lanes are
  unavailable in this session) 및 require P0=0/P1=0.
- [x] 추가 a focused lesson about compatibility boundaries for 공유 helpers.
- [ ] 커밋 함께 Lore trailers, create a PR closing #588, verify CI, then
  rebase-merge, sync `develop`, 및 clean the worktree 후 approved merge.

## 롤백

Revert the migration commit. The 공유 substrate 및 `lock/redis` migration
remain independently usable; 없음 key, payload, token, 또는 schema migration has
occurred.
