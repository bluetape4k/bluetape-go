# concurrency

`concurrency` provides context-aware goroutine helpers built around
`golang.org/x/sync/errgroup`: task groups, bounded parallel map/for-each, a
simple worker pool, and panic-to-error conversion.

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
```

## Behavior

- `Group`, `ForEach`, `Map`, and `WorkerPool` propagate context cancellation.
- Task panics are returned as `PanicError` so callers can handle goroutine
  failures through the same error path.
- Parallel helpers require a positive limit or worker count.
- `Map` preserves input order in the returned result slice.

## Test

```bash
go test -count=1 ./concurrency
```
