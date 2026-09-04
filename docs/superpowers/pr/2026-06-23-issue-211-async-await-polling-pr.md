# async await 및 polling test helper 추가

Closes #211.

## 요약

이 PR은 root `testing` package에 context-aware await/polling helper를
추가한다. 이를 통해 test가 caller-owned context cancellation과 deadline
동작을 보존하면서 제한된 eventual consistency를 기다릴 수 있다.

API는 의도적으로 작게 유지한다. generic `CheckAwait`/`RequireAwait` 한 쌍,
value/error convenience helper, 최종 관측 value·error·attempt·경과 시간을
기록하는 `AwaitResult` diagnostic payload를 제공한다.

## 배경

Milestone 0.6.4는 Go API의 idiomatic한 형태를 유지하면서 bluetape-go의
test helper를 JUnit5/Awaitility 방식에 맞춘다. 기존
`Eventually`/`Consistently` helper는 boolean assertion에 계속 유용하다. 새
await helper는 caller가 context propagation, polling interval, immediate
failure, 최종 관측 diagnostic을 필요로 하는 더 풍부한 probe를 다룬다.

## 작업 내용

- `AwaitStatus`, `AwaitProbe`, `AwaitCheck`, `AwaitErrorProbe`, `AwaitResult`를
  추가했다.
- `CheckAwait`와 `RequireAwait`를 추가했다.
- `CheckAwaitValue`와 `RequireAwaitValue`를 추가했다.
- `CheckAwaitError`와 `RequireAwaitError`를 추가했다.
- 다음 항목을 검증하는 test를 추가했다: 즉시 성공, 최종 성공, timeout 진단,
  즉시 실패 진단, 잘못된 입력, caller cancellation, probe cancellation.
- 다음 example을 추가했다: cache 무효화, lock 획득, Testcontainers 준비 상태,
  workflow 상태 확인.
- `testing/README.md`와 `testing/README.ko.md`를 함께 갱신했다.
- workflow 게이트의 review 및 lesson 증거를 추가했다.

## 검증

- `go test -count=1 ./testing/...`: 구현 전 baseline PASS
- TDD RED: 구현 전에 `go test -count=1 ./testing`가 정의되지 않은
  `CheckAwait*` 및 `AwaitStatus`로 실패
- `go test -count=1 ./testing`: PASS
- `go test -race -count=1 ./testing`: PASS
- `go test -count=1 ./testing/...`: PASS
- `git diff --check`: PASS
- `golangci-lint cache clean`: PASS
- `make fmt-check && make vet && make lint`: PASS, `0 issues.`
- `make test`: PASS
- `make race`: PASS

## 검토 메모

- 추적한 Step 6-R review: `docs/review/2026-06-23-issue-211-async-await-polling-review.md`
- 이전에 stale native subagent cleanup이 한 시간 규모로 대기하며 막혔기
  때문에 main-session 7-tier fallback을 사용했다.
- 검토 결과: P0=0, P1=0.

## 메타데이터

- 이슈: #211
- Milestone: `0.6.4`
- Assignee: `debop`
- Labels: `type: task`, `area: testing`, `priority: p1`, `area: concurrency`

## DoD Status

| 단계 | 상태 | 증거 |
| --- | --- | --- |
| 이슈 metadata | PASS | #211 assignee `debop`, milestone `0.6.4`, label live 확인 |
| Worktree | PASS | `.worktrees/issue-211-async-await-polling`, branch `issue-211-async-await-polling` |
| TDD RED | PASS | 정의되지 않은 API로 구현 전에 `go test -count=1 ./testing` 실패 |
| Implementation | PASS | `testing/await.go`, test, example, README pair |
| Step 6-R review | PASS | `docs/review/2026-06-23-issue-211-async-await-polling-review.md`, P0=0 P1=0 |
| Lessons | PASS | `docs/lessons/2026-06-23-async-await-polling.md` |
| 로컬 검증 | PASS | `make fmt-check && make vet && make lint`, `make test`, `make race` |
| PR body verification | PENDING | PR 생성 후 `gh pr view <number> --json body`로 확인 |
| Step 7-R PR review | PENDING | PR 생성 후 실행 |
| CI | PENDING | GitHub Actions 시작 후 `statusCheckRollup` 확인 |
