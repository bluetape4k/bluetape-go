# Issue #355 Concurrency Parity Decisions

Date: 2026-07-04

Issue #355 compares `bluetape4k-core` concurrent helpers with the current
Go-native `concurrency` package. The decision is selective parity, not a JVM
facade port.

## Source Families

| Kotlin family | Decision | Go reasoning |
|---|---|---|
| `ConcurrentReducer` | Keep existing Go helpers | Current `Group`, `ForEach`, `Map`, and `WorkerPool` already provide context-aware bounded execution, first-error cancellation, and panic-to-error conversion. A future reducer would need a concrete call-site requiring associative reduction or streaming backpressure. |
| `AtomicIntRoundrobin` | Adapt | A small atomic cyclic counter is useful for sharding and target rotation, does not compete with channels or `errgroup`, and can be tested under `go test -race`. |
| `CompletableFutureSupport`, `CompletionStageSupport`, `FutureSupport` | Skip | Go callers should use `context`, channels, direct function calls, and `errgroup`-style groups. A generic future abstraction would compete with idiomatic Go concurrency without current call-site evidence. |
| `LockSupport` latch helpers | Skip | `sync`, channels, `context.WithTimeout`, and package-local tests cover this shape. A public latch wrapper would add API surface without production value. |
| `ExecutorSupport`, `NamedThreadFactory`, `ThreadSupport` | Skip | JVM executor/thread concepts do not map to Go scheduler contracts. |
| `concurrent/virtualthread/*` | Skip | Virtual thread, coroutine, Reactor, scoped value, and structured task scope helpers are JVM-specific or framework-specific. |

## Implemented Surface

- `NewRoundRobin(maximum int) (*RoundRobin, error)`
- `(*RoundRobin).Get() int`
- `(*RoundRobin).Set(value int) error`
- `(*RoundRobin).Next() int`

`RoundRobin` is goroutine-safe and keeps values in `[0, maximum)`. It does not
start goroutines, own resources, or provide scheduling. That keeps it separate
from `Group`, `ForEach`, `Map`, and `WorkerPool`.

## Non-Goals

- No `Future` or promise abstraction.
- No latch wrapper.
- No executor/thread factory abstraction.
- No replacement of `Group`, `ForEach`, `Map`, or `WorkerPool`.
- No logging facade or global worker registry.

## Verification Target

- `go test -count=1 ./concurrency`
- `go test -race -count=1 ./concurrency`
- Full local gate before PR merge.
