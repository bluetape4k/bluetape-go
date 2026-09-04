## 요약

Fixes #201.

이 PR은 `0.6.2` corrective milestone에서 확인한 failure, cancellation, race,
cleanup 게이트의 누락을 보완한다. 주요 목표는 더 넓은 parity 확장 작업을
계속하기 전에 로컬 CI와 stress 검증을 신뢰할 수 있게 만드는 것이다.

## 배경

여러 Redis/Testcontainers 기반 package는 개별적으로는 정상이었지만 전체
로컬 CI에서는 불안정했다. 이 failure는 production behavior regression이
아니라 test-gate의 취약점이었다.

- cleanup이 제한 없는 `context.Background()` 종료 경로를 사용했다.
- Redis test가 부하가 큰 package에서 test마다 container 하나를 반복 생성했다.
- 일부 Redis fixture가 Testcontainers module readiness strategy를 log-only
  wait로 덮어썼다.
- 저장소 전체 `make test`와 `make race`가 Testcontainers 기반 package를
  동시에 실행했다.

## 해결 내용

- parent cancellation은 무시하되 context value는 보존하는 bounded internal
  Testcontainers cleanup helper를 추가한다.
- Testcontainers wrapper와 직접 Redis fixture test가 bounded cleanup을
  사용하도록 연결한다.
- 부하가 큰 Redis suite에서 package가 공유하는 Redis fixture와 test별
  `FlushDB` 격리를 사용한다.
- Redis module의 기본 readiness 동작을 유지하고 log-only wait로 대체하지
  않는다.
- 저장소 전체 test, race, coverage 게이트를 package 직렬 스케줄링으로
  실행한다.
- 누락된 `GoroutineStressTester` failure/cancellation coverage를 추가하고
  대상 stress/race 증거를 기록한다.

## 작업 내용

- parent cancellation, timeout, nil terminator 처리를 unit coverage로 검증하는
  `internal/testcleanup`을 추가했다.
- Redis, Kafka, MySQL, NATS, Postgres Testcontainers wrapper가 bounded
  cleanup을 사용하도록 변경했다.
- package 공유 fixture와 bounded readiness check로 Redis 기반 `jwt`,
  `leader/redis`, `probabilistic/redis` test를 안정화했다.
- Go source에서 직접 사용하는 `container.Terminate(context.Background())`를
  제거했다.
- `-p 1` test/race 동작을 설명하도록 root README locale pair를 갱신했다.
- Step 6-R 및 lesson 산출물을 추가했다.

## 검증

- `make ci`: PASS.
- `git diff --check`: PASS.
- `go test -count=1 ./testing/concurrency ./cache/rediscoord ./jwt ./leader/redis ./probabilistic/redis -run 'GoroutineStressTester|Stress'`: PASS.
- `go test -race -count=1 ./testing/concurrency ./cache/rediscoord ./jwt ./leader/redis ./probabilistic/redis -run 'GoroutineStressTester|Stress'`: PASS.
- `rg -n "container\\.Terminate\\(context\\.Background\\(\\)|WithWaitStrategy\\(|testcontainers/internal/cleanup|go test -race -count=1 ./\\.\\.\\.|go test -count=1 ./\\.\\.\\." --glob '*.go' README.md README.ko.md Makefile`: PASS, no hits.

## 검토 메모

- Step 2-R spec review 기록: `docs/superpowers/reviews/2026-06-14-issue-201-test-gates-step-2r-spec-review.md`, P0=0 P1=0.
- Step 3-R plan review 기록: `docs/superpowers/reviews/2026-06-14-issue-201-test-gates-step-3r-plan-review.md`, P0=0 P1=0.
- Step 6-R code review 기록: `docs/superpowers/reviews/2026-06-14-issue-201-test-gates-step-6r-code-review.md`, P0=0 P1=0.
- Step 7-R PR review 기록: `docs/superpowers/reviews/2026-06-14-issue-201-test-gates-step-7r-pr-review.md`, P0=0 P1=0.
- Subagent 메모: 이번 session에서는 native subagent lane이 불안정했기 때문에
  7-Tier lane을 main-session role switching으로 실행하고 fallback으로
  기록했다.

## 메타데이터

- 이슈: #201.
- Milestone: `0.6.2`.
- Base: `develop`.
- Head: `issue-201-test-gates`.

## DoD Status

| 단계 | 상태 | 증거 |
|---|---|---|
| Step 0 - 이슈/워크트리 설정 | PASS | 이슈 #201 확인; worktree `.worktrees/issue-201-test-gates`, branch `issue-201-test-gates`. |
| Step 2 - Spec | PASS | `docs/superpowers/specs/2026-06-14-issue-201-test-gates-design.md`; `docs/images/readme-diagrams/` 아래 diagram asset 생성. |
| Step 2-R - Spec review | PASS | `docs/superpowers/reviews/2026-06-14-issue-201-test-gates-step-2r-spec-review.md`, P0=0 P1=0. |
| Step 3 - Plan | PASS | `docs/superpowers/plans/2026-06-14-issue-201-test-gates-plan.md`. |
| Step 3-R - Plan review | PASS | `docs/superpowers/reviews/2026-06-14-issue-201-test-gates-step-3r-plan-review.md`, P0=0 P1=0. |
| Step 4 - Implementation | PASS | bounded cleanup helper, Redis fixture 안정화, 직렬 test 게이트, GoroutineStressTester edge coverage를 commit. |
| Step 4-T - Tests | PASS | `make ci`; 위의 대상 GoroutineStressTester 일반 및 race 명령. |
| Step 6-R - 7-Tier code review | PASS | `docs/superpowers/reviews/2026-06-14-issue-201-test-gates-step-6r-code-review.md`, P0=0 P1=0. |
| Step 7 - Lessons | PASS | `docs/lessons/2026-06-14-issue-201-test-gates.md`. |
| Step 7-P - PR creation | PASS | `develop`를 대상으로 PR #237 생성; milestone `0.6.2`, assignee `debop`; live body 확인. |
| Step 7-R - Post-PR review | PASS | `docs/superpowers/reviews/2026-06-14-issue-201-test-gates-step-7r-pr-review.md`, P0=0 P1=0; PR comment 및 공식 review 게시. |
| Step 8 - CI gate | PASS | GitHub Actions `ci` 통과: https://github.com/bluetape4k/bluetape-go/actions/runs/27494105916/job/81264830021. |
