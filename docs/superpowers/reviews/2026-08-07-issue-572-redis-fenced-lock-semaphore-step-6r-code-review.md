# 이슈 #572 Redis fenced lock/semaphore Step 6-R 코드 리뷰

> 한국어 리뷰 경계: 이 문서는 승인된 설계와 구현을 여섯 독립 관점 및 main
> 통합 관점에서 대조한 최종 로컬 리뷰다. 심각도 토큰, 판정 토큰, 파일 경로,
> 명령, 코드 식별자는 증거 앵커로 보존한다.

이슈: #572 `feat: Add Redis fenced lock and semaphore primitives`

날짜: 2026-08-07

기준 및 merge base: `92cfb7fa0c48fbc201e96144be009578fad12b4a`

검토한 구현 SHA: `6d3944f85d3c83b0a56d3de102ccde65e69e539a`

브랜치: `feat/issue-572-redis-fenced-semaphore`

게이트: 독립적인 여섯 관점과 main 세션 통합 검토.

## 검토 범위

- `redis/lock`: persistent fencing counter, same-slot owner/counter keys,
  owner-safe release, context-aware `Acquire`/`TryAcquire`.
- `redis/semaphore`: Redis server-time expiry cleanup, bounded permits,
  exact owner-token release, context-aware `Acquire`/`TryAcquire`.
- bilingual README, compile examples, Testcontainers integration, contention,
  redaction, cancellation/deadline, and race coverage.
- Issue #572 설계 명세와 lock/semaphore 구현 계획의 API·비목표·DoD 대조.

## 최종 정확한 HEAD 결과

| Tier | 관점 | 판정 | P0 | P1 | P2 | P3 |
|---|---|---:|---:|---:|---:|---:|
| 1 | 성능 | PASS | 0 | 0 | 0 | 0 |
| 2 | 안정성 | PASS | 0 | 0 | 0 | 0 |
| 3 | 보안 | PASS | 0 | 0 | 0 | 0 |
| 4 | 운영/Ops | PASS | 0 | 0 | 0 | 0 |
| 5 | 개발자/API | PASS | 0 | 0 | 0 | 0 |
| 6 | 사용자/호출자 | PASS | 0 | 0 | 0 | 0 |
| 메인 | 통합 | PASS | 0 | 0 | 0 | 0 |

## 관점별 근거

### 성능

Lua acquire가 lock의 owner 존재 검사와 counter 증가/TTL 설정을 한 번의
원자적 mutation으로 수행한다. Semaphore도 server `TIME`, 만료 member 정리,
`ZCARD`, `ZADD`를 한 script로 수행한다. 두 package 모두 5ms→100ms bounded
timer/select backoff을 사용하며 goroutine, ticker, watchdog을 만들지 않는다.
실제 Redis contention/stress와 전체 race suite가 통과했다. P0/P1/P2/P3 없음.

### 안정성

Acquire는 `ErrNotAcquired`만 재시도하고 provider 오류는 재시도하지 않는다.
lock acquire mutation 결과가 불명확하면 owner/counter를 bounded background
probe해 lease를 복원하거나 `OpError + ErrCommitUnknown`을 반환한다. Release는
exact owner token을 비교하고 만료/교체 상태를 `(false, nil)`로 처리한다.
Cancellation/deadline은 사전 검증과 `errors.Is` 경로로 보존되며 full normal/race
suite가 PASS했다. P0/P1/P2/P3 없음.

### 보안

Caller logical key는 SHA-256 digest hash tag로 Redis key와 진단에 반영되고 raw
key와 owner token은 `OpError.Error()`에 노출되지 않는다. OwnerToken은 256-bit
random lowercase hex이며 Redis argument 이외의 표시가 redacted된다. 공개
`Lease.Key()`는 caller가 자신의 logical key를 조회하는 API로만 raw value를
반환하며 provider 오류 경계와 혼동하지 않는다. P0/P1/P2/P3 없음.

### 운영/Ops

Redis client와 lifecycle은 caller-owned이고, 문서와 examples는 취소된 업무
context 뒤 별도 bounded cleanup context 사용 및 `ErrCommitUnknown` 재확인을
명시한다. lock의 fencing은 외부 resource가 token ordering을 저장·검증할 때만
유효하고, semaphore는 fencing을 제공하지 않으며 TTL 이후 overlap할 수 있다는
경계를 README/doc.go에 고정했다. P0/P1/P2/P3 없음.

### 개발자/API

Options validation은 blank key, invalid TTL, nil client, non-positive permits를
거부한다. `TryAcquire`는 즉시형이고 `Acquire`는 context deadline까지 대기한다.
Lease accessors는 nil/zero-safe하고 bilingual README와 compile examples가 공개
계약을 설명한다. 기존 `lock/redis.Mutex`는 수정하지 않았다. P0/P1/P2/P3 없음.

### 사용자/호출자

성공 lock lease는 양수 monotonic fencing token을 받고, stale release가 fresh
owner를 삭제하지 않는다. Semaphore는 permit accounting, expiry cleanup,
idempotent release, canceled waiter의 permit 비유출을 실제 Redis에서 검증한다.
호출자가 보호 resource의 fencing/버전 검증을 직접 구성해야 한다는 caveat도
examples와 README에 있다. P0/P1/P2/P3 없음.

### Main 통합

설계 명세의 persistent counter, same-slot digest keys, owner-safe release,
server-time expiry, context-first wait, no Redlock/no watchdog/no FIFO 비목표가
구현·테스트·문서에 일치한다. 신규 package와 기존 Redis/lock regression, 저장소
전체 normal/race 테스트가 모두 통과했다. P0/P1/P2/P3 없음.

## 검증 증거

- `go test -p 1 -count=1 ./redis/lock` — PASS, 실제 Redis Testcontainers
  acquire/release, expiry, fencing monotonicity, wait/deadline, redaction,
  single-owner contention 포함.
- `go test -p 1 -count=1 ./redis/semaphore` — PASS, permit accounting, expiry,
  stale member, deadline, redaction, 16-worker capacity stress 포함.
- `go test -p 1 -race -count=1 ./redis/lock` — PASS.
- `go test -p 1 -race -count=1 ./redis/semaphore` — PASS.
- `go test -p 1 -count=1 ./redis ./lock/redis` — PASS, 기존 shared substrate와
  기존 Mutex 회귀 확인.
- `go test -p 1 -count=1 ./...` — PASS.
- `go test -race -p 1 -count=1 ./...` — PASS.
- `make fmt-check`, `make tidy-check`, `make vet` — PASS.
- `golangci-lint run ./redis/lock ./redis/semaphore` — PASS (`0 issues`).
- `make lint` — 저장소 밖의 sibling worktree
  `../issue-518-s3-examples`를 가리키는 stale cache 경고와 기존 10개 issue로
  실패했다. 현재 브랜치 신규 package lint는 별도 명령으로 PASS했으며, 이
  환경 실패는 구현 파일의 issue가 아니다.
- `git diff --check` — PASS.

## 메인 통합 판정

로컬 Step 6-R PASS.

- P0 = 0
- P1 = 0
- P2 = 0
- P3 = 0
- 승인된 명세·계획과 구현, 테스트, README, examples가 일치한다.
- Merge-ready 판정은 원격 PR의 정확한 head, CI, 리뷰 스레드 확인 및 별도 최신
  merge 승인 전에는 하지 않는다.

## DoD

| 항목 | 상태 |
|---|---|
| 여섯 독립 관점 검토 | 완료 |
| main 통합 검토 | 완료 |
| P0/P1/P2/P3 정규화 | 완료: 모두 0 |
| 실제 Redis normal/race 증거 | 완료 |
| 저장소 전체 normal/race 증거 | 완료 |
| 정적·format·tidy·vet 증거 | 완료 |
| 신규 package lint | 완료 |
| 전체 `make lint` | 환경 stale sibling worktree issue로 미완료 |
| Type A lesson | 다음 문서로 기록 |
| 원격 CI/PR/review | PR 생성 후 수행 |
| Merge 부수 효과 | 별도 최신 명시 승인 필요 |
