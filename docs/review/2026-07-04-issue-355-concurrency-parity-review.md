# Issue #355 Concurrency Parity Review

Date: 2026-07-04

Scope:

- `concurrency/round_robin.go`
- `concurrency/concurrency_test.go`
- `concurrency/concurrency_example_test.go`
- `concurrency/doc.go`
- `concurrency/README.md`
- `concurrency/README.ko.md`
- `docs/research/2026-07-04-issue-355-concurrency-parity.md`

## Evidence

- Issue #355 requires adapt/skip decisions for reducer, round-robin,
  future-composition, and lock/latch patterns.
- Kotlin source references reviewed:
  - `concurrent/ConcurrentReducer.kt`
  - `concurrent/AtomicIntRoundrobin.kt`
  - `concurrent/LockSupport.kt`
  - `concurrent/CompletableFutureSupport.kt`
  - `concurrent/CompletionStageSupport.kt`
  - `concurrent/FutureSupport.kt`
  - `concurrent/virtualthread/*`
- Current Go `concurrency` package already provides `Group`, `ForEach`, `Map`,
  `WorkerPool`, `Go`, and panic-to-error handling around `context.Context` and
  `errgroup`.

## 7-Tier Lanes

| Lane | Verdict | Notes |
|---|---|---|
| Performance | Pass | Initial P1 found the concurrent test did not prove no lost updates. Fixed by asserting exact per-slot counts and total calls under contention. |
| Stability | Pass | Initial P1s found the untracked implementation/doc artifact and weak concurrent test. Fixed by including new files in the staged patch and strengthening the test. |
| Security | Pass | No auth or secret boundary changed. `RoundRobin` validates maximum and `Set` range, uses atomic CAS, and docs describe retry/slot rotation rather than security rotation. |
| Operator/Ops | Pass | Initial P1 found the patch was not self-contained because the implementation was untracked. Fixed by staging `round_robin.go` and the parity decision document before commit. |
| Developer/API | Pass | `RoundRobin` is a small Go-native primitive. JVM future, virtual-thread, coroutine, Reactor, executor, thread-factory, and latch wrappers remain explicit non-goals. |
| User/Caller | Pass | README pairs and examples show construction, `Next`, and race-test expectations. Decision notes document adapt/skip choices for each Kotlin candidate family. |
| Integration | Pass | Targeted tests, race tests, and the full local gate passed. |

## Validation

- `git diff --check`: PASS
- `git diff --cached --check`: PASS
- `go test -count=1 ./concurrency`: PASS
- `go test -race -count=1 ./concurrency`: PASS
- `make fmt-check`: PASS
- `make tidy-check`: PASS
- `make vet`: PASS
- `make lint`: PASS
- `make test`: PASS
- `make race`: PASS

## Findings

- P0: 0
- P1: 0

## Residual Risk

`RoundRobin` is intentionally only a cyclic counter. If future call sites need
bounded reduction, streaming backpressure, or first-success/first-completed
coordination, they should be scoped separately with concrete call-site evidence
instead of expanding this package into a generic future abstraction.
