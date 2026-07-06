# concurrency

[English](README.md) | [한국어](README.ko.md)

`concurrency` provides context-aware goroutine helpers built around
`golang.org/x/sync/errgroup`: task groups, bounded parallel map/for-each, a
simple worker pool, a goroutine-safe round-robin counter, and panic-to-error
conversion.

![concurrency package map](../docs/images/readme-diagrams/concurrency-package-map.png)

## Import

```go
import "github.com/bluetape4k/bluetape-go/concurrency"
```

## Usage

```go
values, err := concurrency.Map(ctx, []int{1, 2, 3}, 2,
    func(ctx context.Context, value int) (int, error) {
        return value * value, ctx.Err()
    },
)
if err != nil {
    return err
}

pool, err := concurrency.NewWorkerPool[int](4, func(ctx context.Context, value int) error {
    return process(ctx, value)
})
if err != nil {
    return err
}
err = pool.Run(ctx, jobs)

roundRobin, err := concurrency.NewRoundRobin(4)
nextShard := roundRobin.Next()
```

## Behavior

- `Group`, `ForEach`, `Map`, and `WorkerPool` propagate context cancellation.
- Task panics are returned as `PanicError` so callers can handle goroutine
  failures through the same error path.
- Parallel helpers require a positive limit or worker count.
- `Map` preserves input order in the returned result slice.
- `RoundRobin` is a small goroutine-safe cyclic counter for sharding, slot
  selection, and retry target rotation. It does not schedule goroutines or own
  resources.
- Java `Future`/`CompletableFuture`, virtual-thread, coroutine, Reactor,
  thread-factory, and latch wrappers are intentionally not mirrored. Use
  `context`, channels, `Group`, `ForEach`, `Map`, and `WorkerPool` directly.

## Test

```bash
go test -count=1 ./concurrency
go test -race -count=1 ./concurrency
```
