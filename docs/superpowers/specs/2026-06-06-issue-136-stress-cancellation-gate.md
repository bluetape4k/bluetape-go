# Issue 136 Stress And Cancellation Gate

> 한국어 요구사항 경계: 이 spec/design/test-spec 문서는 한국어 독자가 요구사항을 추적할 수 있도록 목적과 검증 경계를 한국어로 보강한다. API 이름, command, code identifier, issue/PR 번호, compatibility matrix, acceptance keyword, DoD/test evidence는 요구사항 약화를 막기 위해 원문 그대로 보존한다. 변경자는 아래 literal contract를 삭제하거나 의미를 약하게 바꾸지 않아야 한다.
> 추가 한국어 검증 메모: 영어로 남은 항목은 대부분 code/API/evidence literal이다. 구현 전에는 한국어 경계 문장과 원문 acceptance checklist를 함께 읽고, 검증 gate가 줄어들지 않았는지 확인한다.\n

Issue: #136
Milestone: 0.4.0
Parent: #4

## 목표

Define the milestone-level stress and cancellation coverage gate for the 0.4.0
`state`, `workreport`, and `workflow` packages.

The gate is intentionally lightweight: every required check must fit ordinary
targeted `go test` and targeted `go test -race` runs. Heavy soak tests,
benchmarks, or long-running scheduler simulations are out of scope for 0.4.0.

## Required Gate

| Package | Required scenario | Current evidence | Required validation |
|---|---|---|---|
| `state` | Concurrent guarded transition attempts commit once and return deterministic conflicts for concurrent losers. | `state/state_concurrency_test.go:14` uses `GoroutineStressTester`; errors are checked against `ErrConcurrentTransition`. | `go test -count=1 ./state`; `go test -race -count=1 ./state`. |
| `state` | Guard cancellation is propagated and does not mutate state. | `state/state_concurrency_test.go:76` uses `AsyncJobTester`; `state/state_test.go:247` checks cancellation before commit. | `go test -count=1 ./state`; `go test -race -count=1 ./state`. |
| `state` | Invalid, guarded, and deterministic error behavior is covered. | `state/state_test.go:53`, `:66`, `:230`; Step 6-R evidence in `docs/superpowers/reviews/2026-06-05-issue-26-state-code-review.md`. | `go test -count=1 ./state`. |
| `workreport` | Child report aggregation and failure-policy decisions are race-compatible. | `workreport/report_concurrency_test.go:12` uses `GoroutineStressTester`; `workreport/report_test.go:101` and `:127` cover stop/continue policies. | `go test -count=1 ./workreport`; `go test -race -count=1 ./workreport`. |
| `workreport` | Cancellation reports preserve caller cancellation errors. | `workreport/report_concurrency_test.go:46` uses `AsyncJobTester`; `workreport/report_test.go:127` preserves cancelled child errors. | `go test -count=1 ./workreport`; `go test -race -count=1 ./workreport`. |
| `workflow` | Parallel runner aggregation preserves input order and child errors. | `workflow/parallel_test.go:12`; Step 6-R evidence in `docs/superpowers/reviews/2026-06-06-issue-27-workflow-runners-code-review.md`. | `go test -count=1 ./workflow ./workreport`; `go test -race -count=1 ./workflow ./workreport`. |
| `workflow` | Stop/continue policy behavior is covered under sequential and concurrent execution. | `workflow/sequential_test.go`; `workflow/parallel_test.go:37`; `workflow/workflow_concurrency_test.go:13` uses `GoroutineStressTester`. | `go test -count=1 ./workflow ./workreport`; `go test -race -count=1 ./workflow ./workreport`. |
| `workflow` | Caller cancellation propagates and started goroutines are joined. | `workflow/parallel_test.go:69`, `:117`, `:135`; `workflow/workflow_concurrency_test.go:38` uses `AsyncJobTester`. | `go test -count=1 ./workflow ./workreport`; `go test -race -count=1 ./workflow ./workreport`. |

## Milestone Validation Commands

```bash
rg -n "GoroutineStressTester|AsyncJobTester|go test .*race|cancellation|context" state workflow workreport testing docs/superpowers
go test -count=1 ./state ./workflow ./workreport
go test -race -count=1 ./state ./workflow ./workreport
go test -count=1 ./...
git diff --check
```

## Exclusions

- Heavy soak tests and benchmark loops are not required for 0.4.0 closure.
- Durable workflow execution, retry/repeat runners, schedulers, and any-success
  parallel semantics remain out of scope for #136.
- Infrastructure/Testcontainers stress is not part of this gate because the
  affected 0.4.0 packages are in-memory and service-free.

## Linkage

- Epic #4 references #136 as the owner of milestone stress/cancellation
  coverage.
- Issue #26 references #136 for state stress/cancellation coverage.
- Issue #27 references #136 for workflow cancellation/stress coverage.
- Issue #28 references #136 for workreport aggregation/cancellation coverage.

## Acceptance Mapping

| #136 acceptance criterion | Status |
|---|---|
| #26 covers concurrent transition attempts, invalid/guarded transitions, and deterministic errors under contention. | Covered by `state` tests and race command. |
| #27 covers context cancellation propagation, parallel aggregation, and stop/continue policy under concurrent execution. | Covered by `workflow` unit, stress, cancellation, and race tests. |
| #28 covers child report aggregation and failure-policy decisions without data races. | Covered by `workreport` unit, stress, cancellation, and race tests. |
| Targeted race-compatible validation commands are documented in implementing PRs. | Covered by #26, #27, #28 verifier/review artifacts and this gate. |
| Gate is referenced from Epic #4 or implementing issue bodies before 0.4.0 closes. | Covered by Epic #4 and #26/#27/#28 issue bodies. |
