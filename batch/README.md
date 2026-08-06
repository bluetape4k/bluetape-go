# batch

English | [한국어](README.ko.md)

`batch` provides a small context-aware batch processing core. A `Step` reads
items, processes or filters them one at a time, writes kept items in chunks, and
optionally stores checkpoints. A `Job` runs steps sequentially and stops at the
first failed or cancelled child.

![batch step and job flow](../docs/images/readme-diagrams/batch-step-job-flow.png)

## Import

```go
import "github.com/bluetape4k/bluetape-go/batch"
```

## Usage

```go
step, err := batch.NewStep(batch.StepOptions[string, string]{
    Name:      "normalize-users",
    ChunkSize: 100,
    Reader:    reader,
    Processor: batch.ProcessorFunc[string, string](func(ctx context.Context, value string) (string, bool, error) {
        if err := ctx.Err(); err != nil {
            return "", false, err
        }
        return strings.TrimSpace(value), value != "", nil
    }),
    Writer: writer,
    RetryPolicy: batch.RetryPolicy{
        MaxAttempts: 3,
    },
    CheckpointStore: batch.NewMemoryCheckpointStore(),
})
if err != nil {
    return err
}

report := step.Run(ctx)
if !report.IsSuccess() {
    return report.Err
}
```

Multiple steps can be composed as one sequential job:

```go
job, err := batch.NewJob("nightly-import", extractStep, publishStep)
if err != nil {
    return err
}
report := job.Run(ctx)
```

## Behavior

- `Step` opens reader and writer resources, restores checkpoints when a
  checkpoint store is configured, then loops until the reader is exhausted or
  the context is cancelled.
- `Processor` can filter an item with `keep=false` without failing the step.
- `RetryPolicy` applies to processor and writer errors before skip/fail
  handling.
- `SkipPolicy` can skip processor errors. Writer chunk skips are rejected when
  checkpointing is enabled because the writer cannot expose a safe committed
  item boundary.
- `Report` captures status, counters, elapsed boundary timestamps, child
  reports, and the terminal error.

## Atomic PostgreSQL Checkpoints

The legacy `Writer + CheckpointStore` path can use a durable checkpoint store,
but it is **not atomic with business writes**: `Writer.Write` and
`CheckpointStore.Save` are separate operations. A crash between them can replay
an already committed chunk. This remains a supported at-least-once path when
business writes and checkpoints intentionally use different stores.

Use the additive `NewAtomicStep` constructor with
[`batch/sqlcheckpoint`](sqlcheckpoint/README.md) when PostgreSQL business rows
and reader progress must commit in one transaction. For this atomic path,
`RetryPolicy` and `SkipPolicy` apply only to processor failures; they never
retry/skip `AtomicCheckpointWriter.Commit`, the callback, checkpoint CAS, or an
unknown-outcome error.

## Operational Boundaries

- The package owns control flow, counters, retry, skip, chunking, and checkpoint
  coordination. It does not provide database, queue, or file adapters.
- `MemoryCheckpointStore` is useful for tests and local jobs; production
  checkpoint durability should use a caller-owned store.
- Resource close runs with `context.WithoutCancel` so cleanup still executes
  after caller cancellation.

## Test

```bash
go test -count=1 ./batch
```
