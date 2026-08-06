# Issue #355 Concurrency Parity Decisions

Date: 2026-07-04

Issue #355는 `bluetape4k-core` concurrent helper와 현재 Go-native `concurrency` package를 비교한다. 결정은 선택적
parity이며 JVM facade port가 아니다.

## Source Families

| Kotlin family | Decision | Go reasoning |
|---|---|---|
| `ConcurrentReducer` | 기존 Go helper 유지 | 현재 `Group`, `ForEach`, `Map`, `WorkerPool`은 context-aware bounded execution, first-error cancellation, panic-to-error conversion을 이미 제공한다. future reducer는 associative reduction 또는 streaming backpressure를 요구하는 concrete call-site가 있을 때만 필요하다. |
| `AtomicIntRoundrobin` | Adapt | 작은 atomic cyclic counter는 sharding 및 target rotation에 유용하고 channel 또는 `errgroup`과 경쟁하지 않으며 `go test -race`로 테스트할 수 있다. |
| `CompletableFutureSupport`, `CompletionStageSupport`, `FutureSupport` | Skip | Go caller는 `context`, channel, 직접 function call, `errgroup`-style group을 써야 한다. generic future abstraction은 현재 call-site evidence 없이 idiomatic Go concurrency와 경쟁한다. |
| `LockSupport` latch helper | Skip | `sync`, channel, `context.WithTimeout`, package-local test가 이 shape를 덮는다. public latch wrapper는 production value 없이 API surface만 늘린다. |
| `ExecutorSupport`, `NamedThreadFactory`, `ThreadSupport` | Skip | JVM executor/thread 개념은 Go scheduler contract로 매핑되지 않는다. |
| `concurrent/virtualthread/*` | Skip | virtual thread, coroutine, Reactor, scoped value, structured task scope helper는 JVM-specific 또는 framework-specific이다. |

## Implemented Surface

- `NewRoundRobin(maximum int) (*RoundRobin, error)`
- `(*RoundRobin).Get() int`
- `(*RoundRobin).Set(value int) error`
- `(*RoundRobin).Next() int`

`RoundRobin`은 goroutine-safe이며 값을 `[0, maximum)` 안에 유지한다. goroutine을 시작하지 않고 resource를 소유하지 않으며
scheduling을 제공하지 않는다. 그래서 `Group`, `ForEach`, `Map`, `WorkerPool`과 분리된다.

## Non-Goals

- `Future` 또는 promise abstraction 없음.
- latch wrapper 없음.
- executor/thread factory abstraction 없음.
- `Group`, `ForEach`, `Map`, `WorkerPool` 대체 없음.
- logging facade 또는 global worker registry 없음.

## Verification Target

- `go test -count=1 ./concurrency`
- `go test -race -count=1 ./concurrency`
- PR merge 전 full local gate.
