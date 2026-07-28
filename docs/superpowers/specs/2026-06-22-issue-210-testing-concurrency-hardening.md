# Issue #210 Testing Concurrency Hardening Spec

> 한국어 요구사항 경계: 이 spec/design/test-spec 문서는 한국어 독자가 요구사항을 추적할 수 있도록 목적과 검증 경계를 한국어로 보강한다. API 이름, command, code identifier, issue/PR 번호, compatibility matrix, acceptance keyword, DoD/test evidence는 요구사항 약화를 막기 위해 원문 그대로 보존한다. 변경자는 아래 literal contract를 삭제하거나 의미를 약하게 바꾸지 않아야 한다.
> 추가 한국어 검증 메모: 영어로 남은 항목은 대부분 code/API/evidence literal이다. 구현 전에는 한국어 경계 문장과 원문 acceptance checklist를 함께 읽고, 검증 gate가 줄어들지 않았는지 확인한다.\n

Issue: #210
Milestone: 0.6.4
Worktree: `issue-210-testing-concurrency-audit`
Date: 2026-06-22

## 목표

Harden `testing/concurrency` reporting and tests so callers can prove bounded
worker execution, cooperative timeout/cancellation cleanup, panic/error
aggregation, and deterministic queued-work accounting.

## Evidence

- Baseline `go test -count=1 ./testing/concurrency` passed.
- Baseline `go test -race -count=1 ./testing/concurrency` passed.
- Current helpers already cover basic bounded parallelism, panic/error capture,
  invalid options, timeout, caller cancellation, and `RunT` success.
- Kotlin source concepts reviewed:
  - `StressTester` and `WorkerStressTester` define workers/rounds bounds.
  - `MultithreadingTester` aggregates failures and uses fixed workers.
  - `StructuredTaskScopeTester` limits concurrent blocks and supports timeout.
  - `SuspendedJobTester` treats coroutine cancellation as a structural signal.

## Selected Hardening Slice

1. Keep the current Go API shape: stateless testers, `Options`, `Task`, `Run`,
   and `RunT`.
2. Add report accounting fields:
   - `Scheduled`: total planned task executions after rounds expansion.
   - `Skipped`: planned executions not started because the run context ended
     before they were queued.
3. Preserve current `Started`, `Completed`, `Failures`, `Panics`,
   `MaxConcurrent`, and `Duration` semantics.
4. Add tests proving:
   - each task is executed exactly `RoundsPerTask` times on successful runs,
   - timeout returns and the cooperative task goroutine exits,
   - cancellation reports skipped queued work deterministically,
   - race detector passes for the helper package.
5. Update English and Korean README files with when to use the helpers and when
   plain `testing`, `sync.WaitGroup`, `errgroup`, or table tests are enough.

## Explicit Constraint

Go cannot forcibly terminate a goroutine that ignores `context.Context`.
Therefore the helper contract must be cooperative:

- Timeout/cancellation is bounded when tasks observe `ctx.Done()` or return.
- A task that blocks forever while ignoring context is caller misuse; the
  helper will not claim leak-free cleanup for that case.

## 검증 Plan

- `go test -count=1 ./testing/concurrency`
- `go test -race -count=1 ./testing/concurrency`
- `go test ./testing/...`
- `make fmt-check`
- `make vet`
- `make lint`
- `git diff --check`

## Step 2-R / 3-R Review

Native subagent unavailable/stale cleanup hang; main-session 7-tier fallback
performed.

| Tier | Perspective | P0 | P1 | Notes |
|---|---:|---:|---:|---|
| 1 | Performance | 0 | 0 | Additive integer accounting only. |
| 2 | Stability | 0 | 0 | Cooperative cancellation limit is explicit; skipped work is reportable. |
| 3 | Security | 0 | 0 | Test helper only; no IO/auth/secret surface. |
| 4 | Operator/Ops | 0 | 0 | README will document deterministic failure reports and limits. |
| 5 | Developer/API | 0 | 0 | Report field additions are backward-compatible for callers. |
| 6 | User/Caller | 0 | 0 | Docs distinguish helper use from plain testing primitives. |
| 7 | Integration | 0 | 0 | Scope stays inside `testing/concurrency`. |

P0 = 0, P1 = 0. Proceed to implementation.
