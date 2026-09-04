# async Go API용 cancellation assertion helper 추가

Closes #213.

## 요약

이 PR은 root `testing` package에 test 전용 cancellation assertion helper를
추가한다. 이를 통해 async Go API가 caller-owned context cancellation을
숨기거나 재시도하지 않고 보존한다는 사실을 검증할 수 있다.

helper는 direct cancellation, deadline expiration, blocked waiter release,
cancellation 후 cleanup 관측을 다룬다. 대응하는 `Require*` wrapper는
전달받은 `testing.TB`를 실패시키고, `Check*` 함수는 test에서 쉽게 assertion할
수 있는 diagnostic error를 반환한다.

## 배경

Milestone 0.6.4는 bluetape ecosystem 전반에서 사용하는 cancellation 및
concurrency contract에 Go test helper를 맞춘다. Go는 `ctx.Done()`을 영원히
무시하는 goroutine을 안전하게 중지할 수 없으므로, 이 helper는 작업을 강제
종료할 수 있는 것처럼 가장하지 않고 cooperative cancellation 동작을
의도적으로 assertion한다.

## 작업 내용

- `ContextOperation`, `WaiterProbe`, `CleanupProbe` contract를 추가했다.
- `CheckContextCanceled`와 `RequireContextCanceled`를 추가했다.
- `CheckDeadlineExceeded`와 `RequireDeadlineExceeded`를 추가했다.
- `CheckWaiterReleased`와 `RequireWaiterReleased`를 추가했다.
- `CheckCleanupOnCancel`과 `RequireCleanupOnCancel`을 추가했다.
- success, diagnostic, timeout, cleanup, example test를 추가했다.
- cancellation helper 사용법과 cooperative cancellation 지침을 동기화하여
  `testing/README.md`와 `testing/README.ko.md`를 갱신했다.
- workflow 게이트의 spec, review, lesson 증거를 추가했다.

## 검증

- `go mod download`: PASS
- `go test -count=1 ./testing/...`: PASS baseline
- TDD RED: 구현 전에 정의되지 않은 cancellation API로
  `go test -count=1 ./testing` 실패
- `go test -count=1 ./testing`: PASS
- `go test -race -count=1 ./testing`: PASS
- `go test -count=1 ./testing/...`: PASS
- `golangci-lint cache clean`: PASS
- `make fmt-check && make vet && make lint`: PASS, `0 issues.`
- `make test`: PASS
- `make race`: PASS
- `git diff --check`: PASS

## 검토 메모

- 추적한 Step 6-R review: `docs/review/2026-06-23-issue-213-cancellation-assertions-review.md`
- stale native subagent cleanup이 막힌 뒤 중단되었기 때문에 main-session
  7-tier fallback을 사용했다.
- 검토 결과: P0=0, P1=0.

## 메타데이터

- 이슈: #213
- Milestone: `0.6.4`
- Assignee: `debop`
- Labels: `type: task`, `area: testing`, `priority: p0`, `area: concurrency`

## DoD Status

| 단계 | 상태 | 증거 |
| --- | --- | --- |
| 이슈 metadata | PASS | #213 assignee `debop`, milestone `0.6.4`, label live 확인 |
| Worktree | PASS | `.worktrees/issue-213-cancellation-assertions`, branch `issue-213-cancellation-assertions` |
| TDD RED | PASS | 정의되지 않은 API로 구현 전에 `go test -count=1 ./testing` 실패 |
| Implementation | PASS | `testing/cancellation.go`, test, example, README pair |
| Step 6-R review | PASS | `docs/review/2026-06-23-issue-213-cancellation-assertions-review.md`, P0=0 P1=0 |
| Lessons | PASS | `docs/lessons/2026-06-23-cancellation-assertions.md` |
| 로컬 검증 | PASS | `make fmt-check && make vet && make lint`, `make test`, `make race` |
| PR body verification | PENDING | PR 생성 후 `gh pr view <number> --json body`로 확인 |
| Step 7-R PR review | PENDING | PR 생성 후 실행 |
| CI | PENDING | GitHub Actions 시작 후 `statusCheckRollup` 확인 |
