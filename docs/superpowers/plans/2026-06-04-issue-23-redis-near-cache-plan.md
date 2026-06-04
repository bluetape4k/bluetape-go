# Issue 23 Redis Near Cache Implementation Plan

Issue: #23
Milestone: 0.3.0
Date: 2026-06-04
Spec: `docs/superpowers/specs/2026-06-04-issue-23-redis-near-cache-spec.md`

## Objective

Implement the first Redis NearCache strategy as application-level Pub/Sub
invalidation while preserving a strategy boundary for future RESP3
`CLIENT TRACKING` support.

## Task Plan

| Task | Scope | Details | Validation |
|---|---|---|---|
| T1 Package scaffold | `cache/redisnear` | Add `doc.go`, options, client interface, `ErrClosed`, operation constants, and constructor defaults. | `go test -count=1 ./cache/redisnear` |
| T2 Message contract | `cache/redisnear` | Encode/decode JSON invalidation messages with version, namespace, origin ID, operation, and key. | Unit tests for valid, malformed, unknown version/op, namespace/origin filtering. |
| T3 Subscriber lifecycle | `cache/redisnear` | `NewPubSub` subscribes, waits for ack, starts one receive loop, clears local cache on receive error, applies bounded backoff, and supports idempotent `Close`. | Unit tests for close behavior; Testcontainers constructor readiness test. |
| T4 Cache operations | `cache/redisnear` | Implement `Get`, `Set`, `Delete`, `Clear`, `GetOrLoad`; local mutations delegate to `cache.LoadingCache[string,V]`; mutating operations publish invalidations. | Unit tests for local behavior and `ErrClosed`; Testcontainers peer invalidation tests. |
| T5 Testcontainers peer proof | `cache/redisnear` | Use Redis fixture to run two `NearCache` instances and prove `Set`, `Delete`, `Clear`, and repopulation after invalidation. | `go test -count=1 ./cache/redisnear -run TestPubSub` |
| T6 Stress/cancellation | `cache/redisnear` | Use `GoroutineStressTester` for concurrent operations and peer invalidation; use `AsyncJobTester` for cancellation/close paths. | `go test -count=1 ./cache/redisnear -run 'Stress|Async|Cancellation'` |
| T7 Examples/docs | `cache/redisnear`, README pair, CHANGELOG | Add package docs/example, README/README.ko cache section update, and CHANGELOG Unreleased entry. | `go test -count=1 ./cache/redisnear`; targeted README grep. |
| T8 Lessons/review prep | `docs/lessons`, `docs/superpowers/reviews` | Add lesson, run local 7-tier implemented diff review, and record P0/P1 convergence. | `git diff --check`; review artifact P0=0/P1=0. |

## API and Lifecycle Checks

- `Client` must include `redis.Cmdable` plus `Subscribe`.
- `NewPubSub` must return only after Redis subscription acknowledgement.
- `ErrClosed` must be returned by public operations after `Close`.
- `Close` must be idempotent and must close the subscription goroutine.
- Receive errors before close must call local `Clear`, call `OnError` when set,
  and retry with bounded backoff.
- `Set`, `Delete`, and `Clear` must return publish errors without rolling back
  local mutation.

## Test Matrix

| Behavior | Test type | Required assertion |
|---|---|---|
| Defaults/options | Unit | default namespace/channel/local/origin are set; invalid client/channel/namespace fail. |
| Message payload | Unit | encode/decode round trips; malformed and unknown payloads are reported. |
| Inbound filtering | Unit | same origin and wrong namespace do not mutate local cache. |
| Close lifecycle | Unit | repeated close succeeds; operations after close return `ErrClosed`. |
| Set invalidation | Testcontainers | peer local value is deleted after another instance calls `Set`. |
| Delete invalidation | Testcontainers | peer local value is deleted after another instance calls `Delete`. |
| Clear invalidation | Testcontainers | peer local cache is cleared after another instance calls `Clear`. |
| GetOrLoad repopulation | Testcontainers | peer `GetOrLoad` reloads after invalidation. |
| Subscription readiness | Testcontainers | invalidation published immediately after constructor is observed by peer. |
| Concurrent operations | Stress | `GoroutineStressTester` catches panics/errors/races across operations. |
| Cancellation/close | Async | `AsyncJobTester` verifies canceled constructor/loader/close paths do not hang. |

## Validation Commands

Run Testcontainers-backed commands serially.

```bash
go test -count=1 ./cache
go test -count=1 ./cache/redisnear
go test -race -count=1 ./cache/redisnear
go test -count=1 ./...
git diff --check
```

If Docker/Testcontainers is unavailable, record the blocker and run the
non-container unit subset first:

```bash
go test -count=1 ./cache/redisnear -run 'TestOptions|TestMessage|TestClose'
```

## Benchmark Handling

Stress tests are part of #23. Benchmarks remain follow-up work under #107.
After #23 lands, #107 should include:

- local `cache.Memory` baseline;
- Redis NearCache local hit/miss;
- publish latency;
- peer invalidation latency;
- concurrent `GetOrLoad`;
- measured command and environment notes.

## Documentation Tasks

- README.md: add Redis NearCache package row and concise cache section note.
- README.ko.md: synchronized Korean update.
- CHANGELOG.md: Unreleased entry for `cache/redisnear`.
- Package docs/comments: Go source comments are Korean and short; public
  contributor artifacts remain English.

## Step 3 Checklist Completion Report

| Item | Status | Notes |
|---|---|---|
| Every spec requirement mapped | Done | See task plan and test matrix. |
| Task order is implementable | Done | Scaffold/message/lifecycle before behavior/tests/docs. |
| Concrete validation commands named | Done | Targeted package tests, race test, full `go test`, and `git diff --check`. |
| Testcontainers handled serially | Done | Commands explicitly say serial execution. |
| Stress and benchmark handling clear | Done | Stress in #23, benchmarks in #107. |
| README/CHANGELOG/lessons assigned | Done | T7/T8. |
